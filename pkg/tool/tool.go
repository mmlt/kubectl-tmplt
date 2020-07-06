package tool

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mitchellh/mapstructure"
	"github.com/mmlt/kubectl-tmplt/pkg/azure"
	"github.com/mmlt/kubectl-tmplt/pkg/expand"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"text/template"
)

// Tool is responsible for reading a job file with one or more steps.
// Some steps like 'tmplt' and 'action' take a template file and 'values' as parameters and Tool will expand those.
// Other steps like 'wait' just take a line with flags as parameter.
// Finally Tool hands over each step to Execute for further processing.
type Tool struct {
	// Mode selects what the Tool should do.
	Mode Mode
	// DryRun sets if the Tool is allowed to make changes to the target cluster.
	DryRun bool
	// Environ are the environment variables on Tool invocation.
	Environ []string
	// JobFilepath refers to a yaml format file with 'steps' and 'defaults' fields.
	JobFilepath string
	// ValueFilepath refers to a yaml format file with key-values.
	// These values override job defaults and template values.
	ValueFilepath string
	// VaultPath refers to a directory containing files;
	//	type - Type of vault to read from, valid values are: azure-key-vault | file
	//	url - URL of Vault
	//	clientID, clientSecret - Credential to access vault (cli credentials are used if absent)
	VaultPath string

	// Execute knows how to perform apply, wait and actions on target cluster.
	Execute Executor

	//
	Log logr.Logger

	// readFileFn reads files relative to the job file.
	readFileFn func(string) (string, []byte, error)

	// Vault allows reading from the master-vault.
	vault getter
}

// Mode selects what the Tool should do; see Mode* constants for more.
type Mode int

const (
	// ModeUnknown means no Mode has been specified.
	ModeUnknown Mode = 0

	// ModeGenerate generates templates and writes them to out instead of applying them.
	ModeGenerate Mode = 1 << iota
	// ModeGenerateWithActions generates templates and actions and writes them to out instead of applying them.
	ModeGenerateWithActions = ModeGenerate | ModeActions

	// ModeApply generates and applies templates to the target cluster but doesn't perform actions.
	ModeApply Mode = 1 << iota
	// ModeApplyWithActions generates and applies templates and actions to the target cluster.
	ModeApplyWithActions = ModeApply | ModeActions

	// ModePrune removes the resources from the target cluster that are not in the generated
	// resources but are within a label selection.
	// NB. how is remove resources that are still in use by 'old' pods during rolling update is TBD.
	// NB2. needs a way to automatically add a label to all resources deployed.
	//ModePrune Mode = 1 << iota

	// The following Modes can only be used in combination with above modes.

	// ModeActions is true for a modes that perform actions.
	ModeActions Mode = 1 << iota
)

// Executor provides methods to apply a step to the target cluster or write a textual representation to out.
type Executor interface {
	Wait(id int, flags string) error
	Apply(id int, name string, doc []byte) error
	Action(id int, name string, doc []byte, portForward string, passedValues *yamlx.Values) error
}

// Getter allows reading object fields in templates.
type getter interface {
	// Get returns the value of an object field.
	// An object is identified by key.
	// For composite objects field selects the value, for non-composites field should be empty or "."
	Get(key, field string) string
}

// ModeFromString return tool mode based on (kebab formatted) arg.
func ModeFromString(arg string) (Mode, error) {
	switch arg {
	case "apply":
		return ModeApply, nil
	case "apply-with-actions":
		return ModeApplyWithActions, nil
	case "generate":
		return ModeGenerate, nil
	case "generate-with-actions":
		return ModeGenerateWithActions, nil
	}
	return ModeUnknown, fmt.Errorf("expected mode to be one of [apply,apply-with-actions,generate,generate-with-actions] instead of: %s", arg)
}

// Run runs the Tool.
func (t *Tool) Run(values yamlx.Values) error {
	// create function to read template file content.
	t.readFileFn = func(path string) (string, []byte, error) {
		p := filepath.Join(filepath.Dir(t.JobFilepath), path)
		b, err := ioutil.ReadFile(p)
		return p, b, err
	}

	// create master vault.
	v, err := newVault(t.VaultPath)
	if err != nil {
		return err
	}
	t.vault = v

	// get global values.
	gb := []byte{}
	if t.ValueFilepath != "" {
		gb, err = ioutil.ReadFile(t.ValueFilepath)
		if err != nil {
			return fmt.Errorf("set file: %w", err)
		}
	}

	// get job.
	jb, err := ioutil.ReadFile(t.JobFilepath)
	if err != nil {
		return fmt.Errorf("job file: %w", err)
	}

	return t.run(values, gb, jb)
}

// Run performs all steps in the job.
func (t *Tool) run(setValues yamlx.Values, values, job []byte) error {
	// read values and merge with setValues into globalValues.
	var globalValues yamlx.Values
	err := yaml2.Unmarshal(values, &globalValues)
	if err != nil {
		return fmt.Errorf("parse %s: %w", t.ValueFilepath, err)
	}

	globalValues = yamlx.Merge(globalValues, setValues)

	// job.
	j := &struct {
		Steps    []yamlx.Values `yaml:"steps"`
		Defaults yamlx.Values   `yaml:"defaults"`
	}{}

	err = yaml2.Unmarshal(job, j)
	if err != nil {
		return fmt.Errorf("j file %s: %w", t.JobFilepath, err)
	}

	// passedValues may be set by a step and read by a next step.
	passedValues := yamlx.Values{}

	// perform steps.
	for id, stp := range j.Steps {
		err = t.step(id, stp, j.Defaults, globalValues, &passedValues)
		if err != nil {
			return err
		}
	}

	return nil
}

// Step performs a step.
func (t *Tool) step(id int, stp, defaultValues, globalValues yamlx.Values, passedValues *yamlx.Values) error {
	s, err := decodeStep(stp)
	if err != nil {
		return err
	}

	st := typeOfStep(stp)
	var tmpltPath string
	switch st {
	case TypeWait:
		return t.Execute.Wait(id, s.W)
	case TypeTmplt:
		tmpltPath = s.T
	case TypeAction:
		if t.Mode&ModeActions == 0 {
			// stop before expanding action template because passedValues depends on a previous action.
			return nil
		}
		tmpltPath = s.A
	default:
		return fmt.Errorf("unknown step: %v", stp)
	}

	// read template
	p, b1, err := t.readFileFn(tmpltPath)
	if err != nil {
		return err
	}

	// expand template.
	vs := yamlx.Merge(defaultValues, s.Values, globalValues)

	b, err := expand.Run(t.Environ, p, b1, vs, *passedValues, t.tmpltFunctions())
	if err != nil {
		return fmt.Errorf("expand %s: %w", tmpltPath, err)
	}

	n := filepath.Base(tmpltPath)
	switch st {
	case TypeTmplt:
		err = t.Execute.Apply(id, n, b)
	case TypeAction:
		err = t.Execute.Action(id, n, b, s.PortForward, passedValues)
	}
	if err != nil {
		n := filepath.Base(tmpltPath)
		return fmt.Errorf("tmplt %s: %w", n, err)
	}

	return nil
}

// TypeOfStep returns the step type from stp dynamic yaml.
func typeOfStep(stp yamlx.Values) string {
	for _, t := range []string{TypeTmplt, TypeWait, TypeAction} {
		if _, ok := stp[t]; ok {
			return t
		}
	}
	return ""
}

// Step names.
const (
	TypeTmplt  = "tmplt"
	TypeWait   = "wait"
	TypeAction = "action"
)

// DecodeStep turns the stp dynamic yaml into a struct.
func decodeStep(stp yamlx.Values) (*genericStep, error) {
	cfg := &mapstructure.DecoderConfig{TagName: "yaml"}

	cfg.Result = &genericStep{}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}
	err = dec.Decode(stp)
	if err != nil {
		return nil, err
	}

	return cfg.Result.(*genericStep), nil
}

// GenericStep can represent any step.
type genericStep struct {
	// A is a relative filepath to an action file.
	A string `yaml:"action"`
	// T is a relative filepath to an action file.
	T string `yaml:"tmplt"`
	// W are the wait-for-condition flags.
	W string `yaml:"wait"`
	// PortForward are the flags passed to a concurrently executed 'kubectl port-forward'
	// (ICW A)
	PortForward string `yaml:"portForward"`
	// Values are the template scoped variables.
	// (ICW A, T)
	Values yamlx.Values `yaml:"values"`
}

// TmpltFunctions returns functions that are available during template expansion.
// NB. other functions are defined in package expand.
func (t *Tool) tmpltFunctions() template.FuncMap {
	r := template.FuncMap{}
	if t.vault != nil {
		r["vault"] = t.vault.Get
	}
	return r
}

// NewVault creates a Vault according to values in configPath.
// If no configPath is specified an empty vault is returned.
func newVault(configPath string) (getter, error) {
	if configPath == "" {
		return fileGet{}, nil
	}

	// read vault config directory.
	files, err := ioutil.ReadDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("master-vault-path: %w", err)
	}
	m := map[string]string{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(configPath, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("master-vault-path: %w", err)
		}
		m[f.Name()] = string(b)
	}

	// create vault
	t, ok := m["type"]
	if !ok {
		return nil, fmt.Errorf("expected 'type' file")
	}
	switch t {
	case "azure-key-vault":
		g, err := azure.NewKeyVault(m)
		if err != nil {
			return nil, fmt.Errorf("Azure KeyVault config %s: %w", configPath, err)
		}
		return g, nil
	case "file":
		return fileGet(m), nil
	default:
		return nil, fmt.Errorf("vault config %s must be one of [azure-key-vault,file], got: %s", filepath.Join(configPath, "type"), t)
	}
}

// FileGet allows reading secrets.
type fileGet map[string]string

// Get value addressed by key from files.
// If field is empty return the value as-is.
// Otherwise expect the value to be a JSON object and field a field of the object.
func (fg fileGet) Get(key, field string) string {
	v, ok := fg[key]
	if !ok {
		return fmt.Sprintf("<not found: %s>", key)
	}

	if field == "" {
		return v
	}

	// assume v is in JSON and field is a key of the object.
	m := map[string]string{}
	err := json.Unmarshal([]byte(v), &m)
	if err != nil {
		return fmt.Sprintf("<%v>", err)
	}

	v, ok = m[field]
	if !ok {
		return fmt.Sprintf("<not found: %s>", field)
	}
	return v
}

var _ getter = fileGet{}

package tool

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/kubectl-tmplt/pkg/expand"
	"github.com/mmlt/kubectl-tmplt/pkg/step"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe/kubectl"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"
)

// Mode selects what the tool should do; see Mode* constants for more.
type Mode int

const (
	// ModeUnknown means no mode has been specified.
	ModeUnknown Mode = 0
	// ModeGenerate generates templates without applying them.
	ModeGenerate Mode = 1 << iota
	// ModeApply generates and applies templates to the target cluster.
	ModeApply Mode = 1 << iota
	// ModePrune removes the resources from the target cluster that are not in the generated
	// resources but are within a label selection.
	// NB. how is remove resources that are still in use by 'old' pods during roling update is TBD.
	ModePrune Mode = 1 << iota
)

// Tool is responsible for reading a job file and one or more template files,
// expanding them and applying them to a target k8s cluster.
type tool struct {
	// Mode selects what the tool should do.
	mode Mode
	// TODO DryRun sets if the tool is allowed to make changes to the target cluster.
	dryRun bool
	// Environ are the environment variables on tool invocation.
	environ []string
	// JobFilepath refers to a yaml format file with 'steps' and 'defaults' fields.
	jobFilepath string
	// SetFilepath refers to a yaml format file with key-values.
	// These values override job defaults and template values.
	setFilepath string
	// Kubectl name and global arguments.
	kubeCtl, kubeConfig, kubeContext string

	// readFileFn reads files relative to the job file.
	readFileFn func(string) (string, []byte, error)
	//
	log logr.Logger
}

// New returns a new 'tool' instance.
func New(log logr.Logger,
	kubeCtl string,
	kubeConfig string,
	kubeContext string,
	environ []string,
	mode Mode,
	dryRun bool,
	jobFilepath string,
	setFilepath string) *tool {

	// create function to read template file content.
	bp := filepath.Dir(jobFilepath)
	fn := func(path string) (string, []byte, error) {
		p := filepath.Join(bp, path)
		b, err := ioutil.ReadFile(p)
		return p, b, err
	}

	return &tool{
		mode:        mode,
		dryRun:      dryRun,
		environ:     environ,
		jobFilepath: jobFilepath,
		setFilepath: setFilepath,
		kubeCtl:     kubeCtl,
		kubeConfig:  kubeConfig,
		kubeContext: kubeContext,
		readFileFn:  fn,
		log:         log,
	}
}

// Run runs the tool.
func (t *tool) Run(out io.Writer) error {
	job, err := ioutil.ReadFile(t.jobFilepath)
	if err != nil {
		return fmt.Errorf("job file: %w", err)
	}

	values, err := ioutil.ReadFile(t.setFilepath)
	if err != nil {
		return fmt.Errorf("set file: %w", err)
	}

	return t.run(values, job, out)
}

// Run 'job' with global scoped 'values'.
func (t *tool) run(values, job []byte, out io.Writer) error {
	instrs, err := t.generate(values, job)
	if err != nil {
		return err
	}

	if t.mode == ModeGenerate {
		instrs.fprint(out)
		return nil
	}

	if t.mode&ModeApply != 0 {
		err = t.apply(instrs)
		if err != nil {
			return err
		}
	}

	//TODO if t.mode&ModePrune {
	//}

	return nil
}

// Generate the instructions to apply.
func (t *tool) generate(values, job []byte) (instructions, error) {
	instrs := instructions{}

	var vs yamlx.Values
	err := yaml2.Unmarshal(values, &vs)
	if err != nil {
		return instrs, fmt.Errorf("parse %s: %w", t.setFilepath, err)
	}

	si, err := step.Iterator(job)
	if err != nil {
		return instrs, fmt.Errorf("parse %s: %w", t.jobFilepath, err)
	}

	for o := si.Next(); o != nil; o = si.Next() {
		//TODO consider removing switch by dispatching on 'o' instead of 't'
		switch o.(type) {
		case *step.Tmplt:
			err = t.generateTmplt(o.(*step.Tmplt), vs, si.Defaults(), &instrs)
		case *step.Wait:
			err = t.generateWait(o.(*step.Wait), &instrs)
		case error:
			return instrs, o.(error)
		default:
			panic("should not happen")
		}

		if err != nil {
			return instrs, err
		}
	}

	return instrs, nil
}

func (t *tool) generateTmplt(tmplt *step.Tmplt, globals, defaults yamlx.Values, instrs *instructions) error {
	// merge template values.
	vs := yamlx.Merge(defaults, tmplt.Values, globals)

	// read template.
	p, b1, err := t.readFileFn(tmplt.Tmplt)
	if err != nil {
		return fmt.Errorf("%s: %w", tmplt.Tmplt, err)
	}

	// expand template.
	b, err := expand.Run(t.environ, p, b1, vs)
	if err != nil {
		return fmt.Errorf("expand %s: %w", tmplt.Tmplt, err)
	}

	o := filepath.Base(tmplt.Tmplt)

	// generate an "apply" instruction per resource.
	docs, err := yamlx.SplitDoc(b)
	if err != nil {
		return fmt.Errorf("split %s: %w", tmplt.Tmplt, err)
	}
	for _, d := range docs {
		if yamlx.IsEmpty(d) {
			continue
		}
		instrs.Add(&instruction{
			typ:    InstrApply,
			input:  string(d),
			args:   []string{"apply", "-f", "-"},
			origin: o,
		})
	}

	return nil
}

func (t *tool) generateWait(condition *step.Wait, instrs *instructions) error {
	instrs.Add(&instruction{
		typ:  InstrWait,
		args: append([]string{"wait"}, strings.Split(condition.C, " ")...),
	})

	return nil
}

// Apply uses kubectl to apply instructions to a target cluster.
func (t *tool) apply(instrs instructions) error {
	// prepare options.
	opt := &kubectl.Opt{
		ExeOpt: &exe.Opt{
			Env: t.environ,
		},
		KubeCtl:     t.kubeCtl,
		KubeConfig:  t.kubeConfig,
		KubeContext: t.kubeContext,
	}

	for _, instr := range instrs.items {
		var stdout string
		var err error
		var exp exponential

		switch instr.typ {
		case InstrWait:
			if t.dryRun {
				continue
			}
			end := time.Now().Add(10 * time.Minute)
			for !time.Now().After(end) {
				stdout, _, err = kubectl.RunTxt(t.log, opt, instr.input, instr.args...)
				if strings.Contains(stdout, "condition met") {
					break
				}
				exp.Sleep(10 * time.Second)
			}
		case InstrApply:
			stdout, _, err = kubectl.RunTxt(t.log, opt, instr.input, instr.args...)
		default:
			return fmt.Errorf("unexpected instruction: %v", instr.typ)
		}
		stdout = strings.TrimSuffix(stdout, "\n")
		t.log.Info(instr.name(), "id", instr.id, "tpl", instr.origin, "msg", stdout)
		if err != nil {
			return fmt.Errorf("##%d tpl %s: %w", instr.id, instr.origin, err)
		}
	}

	return nil
}

// Exponential Sleep
type exponential int64

// Sleep exponentially longer on each invocation with a limit of max time.
func (ex *exponential) Sleep(max time.Duration) {
	if *ex == 0 {
		*ex = 1
	}
	d := int64(*ex) * 100 * time.Millisecond.Nanoseconds()
	if d > max.Nanoseconds() {
		d = max.Nanoseconds()
	} else {
		*ex <<= 1
	}

	time.Sleep(time.Duration(d))
}

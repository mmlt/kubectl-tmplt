package main

import (
	"flag"
	"fmt"
	"github.com/go-logr/stdr"
	"github.com/mmlt/kubectl-tmplt/pkg/execute"
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"
)

// Version as set during go build.
var Version string

func main() {
	var mode modeFlag
	flag.Var(&mode, "m",
		`Mode is one of;
apply - generates and applies templates to the target cluster but doesn't perform actions
apply-with-actions - generates and applies templates and actions to the target cluster
generate - generates templates and writes them to stdout instead of applying them
generate-with-actions - generates templates and actions and writes them to stdout instead of applying them`)
	var dryRun bool
	flag.BoolVar(&dryRun, "dry-run", false,
		`Dry-run prevents any change being made to the target cluster`)
	var noDelete bool
	flag.BoolVar(&noDelete, "no-delete", false,
		`No-delete prevents prune from deleting objects in target cluster`)

	var jobFile string
	flag.StringVar(&jobFile, "job-file", "",
		`Yaml file with steps to perform`)
	var setFile string
	flag.StringVar(&setFile, "set-file", "",
		`Yaml file with values that override template values`)
	var values setValuesFlag
	flag.Var(&values, "set-value",
		`Set value to be used as template value, multiple set-value's are allowed`)

	var kubeContext, kubeConfig, kubeCtl string
	flag.StringVar(&kubeContext, "context", "",
		`Equivalent of kubectl --context`)
	flag.StringVar(&kubeConfig, "kubeconfig", "",
		`Equivalent of kubectl --kubeconfig`)
	flag.StringVar(&kubeCtl, "kubectl", "kubectl",
		`The binary to access the target cluster with`)

	var masterVaultPath string
	flag.StringVar(&masterVaultPath, "master-vault-path", "",
		`Path to a directory containing master vault configuration (also see help)`)
	var verbosity int
	flag.IntVar(&verbosity, "v", 0,
		`Log verbosity, higher numbers produce more output`)
	var version bool
	flag.BoolVar(&version, "version", false,
		"Print version")
	var hlp bool
	flag.BoolVar(&hlp, "help", false,
		`Help page`)

	flag.Parse()

	if hlp {
		fmt.Fprintf(os.Stderr, help, filepath.Base(os.Args[0]), Version)
		os.Exit(0)
	}

	if version {
		fmt.Println(filepath.Base(os.Args[0]), Version)
		os.Exit(0)
	}

	if msg := validate(jobFile, verbosity); len(msg) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, strings.Join(msg, ", "))
		flag.Usage()
		os.Exit(1)
	}

	stdr.SetVerbosity(verbosity)
	log := stdr.New(stdlog.New(os.Stderr, "I ", stdlog.Ltime))

	var out io.Writer
	if mode.V&tool.ModeGenerate != 0 {
		//TODO move this to tool?
		out = os.Stdout
	}

	environ := os.Environ()

	t := tool.Tool{
		Mode:          mode.V,
		DryRun:        dryRun,
		Environ:       os.Environ(),
		JobFilepath:   jobFile,
		ValueFilepath: setFile,
		VaultPath:     masterVaultPath,
		Execute: &execute.Execute{
			DryRun:   dryRun,
			NoDelete: noDelete,
			Environ:  environ,
			Kubectl: execute.Kubectl{
				KubeConfig:  kubeConfig,
				KubeContext: kubeContext,
				KubeCtl:     kubeCtl,
				Environ:     environ,
				Log:         log,
			},
			Out: out,
			Log: log,
		},
		Log: log,
	}
	err := t.Run(values.V)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "E", err)
		os.Exit(1)
	}
}

// Validate checks flags and environment variables and returns a list error strings.
func validate(jobFile string, verbosity int) []string {
	var r []string

	if jobFile == "" {
		r = append(r, "-job-file should be defined")
	}

	if verbosity < 0 || verbosity > 5 {
		r = append(r, "-verbosity should be in the range 0..5")
	}

	return r
}

// ModeFlag is a custom flag type that accepts -m <mode>.
type modeFlag struct {
	V tool.Mode
}

func (f *modeFlag) String() string {
	return fmt.Sprintf("%v", f.V)
}

func (f *modeFlag) Set(s string) error {
	m, err := tool.ModeFromString(s)
	f.V = m
	return err
}

// SetValuesFlag is a custom flag type that accepts one or more --set-value k=v occurrences.
type setValuesFlag struct {
	V yamlx.Values
}

func (f *setValuesFlag) String() string {
	if f.V != nil {
		return fmt.Sprintf("%v", f.V)
	}
	return ""
}

func (f *setValuesFlag) Set(s string) error {
	kv := strings.Split(s, "=")
	if len(kv) != 2 {
		return fmt.Errorf("expected key=value")
	}
	if f.V == nil {
		f.V = make(yamlx.Values)
	}
	f.V[kv[0]] = kv[1]

	return nil
}

// Help text
// text argument: %[1]=program name, %[2]=program version.
const help = `%[1]s reads a job file and performs the steps. 

%[1]s can operate in 'generate' or 'apply' mode.
In 'generate' mode a 'kubectl apply -f -' consumable output is generated ('wait' and 'action' steps are skipped)
In 'apply' mode steps are applied to the target cluster and (optionally) objects are pruned.


JOB FILE
A Job file specifies what %[1]s should do.

Consider a job file containing;
	prune:
	  labels:
		my.example.com/gitops: minikube-all
	  store:
		name: minikube-all
		namespace: default
		x:
		  time: "{{ now | date ` + "`" + `2000-01-02 13:14:15` + "`" + ` }}"
    
	steps:
	- tmplt: tpl/example.txt
	  values:
		text: "{{ .Values.first }} {{ .Values.second }}"
	defaults:
	  first: hello
	  second: world

Job files can contain templated values. In the above example .Values.text="hello world" is being passed to the template.
Caveats:
- The job file is parsed before expansion therefore {{ }} need to be wrapped in double quotes to have (arguably) valid yaml.
- There is currently no easy way to see the content of the job file after expansion.

Prune (optional) makes %[1]s to 1) add labels to all objects and 2) delete cluster objects that are no longer in the
list of deployed objects. The list of deployed objects is stored as a ConfigMap with store.namespace/name in the target cluster.
Extra fields can be stored by putting them below 'x', in the example a 'time' field is added with the time of deployment.
Note:
- Each Job file must use an unique store.namespace/name (otherwise they prune each others objects)
- Labeling causes fields in yaml output to be sorted, comments to be removed, single quotes become double quotes.


STEPS
A step can be one of:
    tmplt: expand a template with values and apply the result to a target k8s cluster.
    wait: wait until a target k8s cluster satisfies a condition.
    action: expand a template to invoke a build-in action.

All steps except 'wait' accept 'value:' as extra arguments for template expansion.


TMPLT STEP
A tmplt step expands the argument template file. 


WAIT STEP
A wait step halts until a certain condition in the target cluster becomes true.


ACTION STEP
An action step perform an action on the target cluster. The kind of action is set by the 'type:' field.
Type can be one of;
    getSecret - to read a secret from a target cluster.
    setVault - to write secrets to target cluster Vault.

getSecret
GetSecret reads a secret from a target cluster.
A getSecret template contains the following arguments;
    type: getSecret
    namespace: namespace-of-secret
    name: name-of-secret
    postCondition: an-optional-expression
If a postCondition is present getSecret will be retried until the condition becomes 'true'.
Expressions use https://golang.org/pkg/text/template/ syntax without the curlies. 
For example postCondition: gt (len (index .data "xyz")) 10 is 'true' when the .data.xyz field of the fetched Secret
contains more then 10 characters.
When a Secret is successfully fetched its 'data' field can be used in subsequent templates via;
    {{ .Get.secret.namespace-of-secret.name-of-secret.data.xyz }}

setVault
SetVault writes one or more secrets to target cluster Vault.
This action step accepts a 'portForward:' setting that tunnels a localhost connection to the target cluster. 
A getSecret template contains the following arguments;
    type: setVault
	url: https://localhost:8200
	tlsSkipVerify: "true"
	token: {{ index .Get "secret" "namespace-of-secret" "name-of-secret" "data" "vault-root" | b64dec }}
	config:
	  logicals:
	  - path: secret/data/test
		data:
		  data:
			USER: superman
			PW: supersecret
      policies:
      - name: secret_allow
    	rule: path "secret/*" {
            capabilities = ["create", "read", "update", "delete", "list"]
          }


TEMPLATING
Templating uses 'https://golang.org/pkg/text/template/' with additional functions:
    http://masterminds.github.io/sprig/
    indexOrDefault - like text/template 'index' function but takes a default value as first argument.
    toToml, to/fromYaml, to/fromJson - convert between string and object form
	vault path/to/object field - read a value from master vault (also see MASTER VAULT)

Templating examples:
    {{ .Files.Get "filename" }}
	
    {{ range $name, $content := .Files.Glob "examples/*.yaml" }}
      {{ filebase $name }}: |
    {{ $content | indent 4 }}{{ end }}
	
    {{ (.Files.Glob "examples/*.yaml").AsConfig | indent 4 }}
	
    {{ (.Files.Glob "secrets/*").AsSecrets }}

    {{ vault "secretname" "fieldname" }}

    {{ indexOrDefault "not found" .Values "path" "to" "elem" }}

Beware, file access is not sanitized!


MASTER VAULT
The master vault config directory contains config files to access a remote KeyVault or a local file vault.
Files:
    type - contains 'azure-key-vault' or 'file'
Files when type contains 'azure-key-vault':
    cli - if 'true' the 'az login' token is used to access the KeyVault otherwise the 'VAULT_*' values are used.
    URL - contains the KeyVault URL
    VAULT_ - files contain values to authenticate with KeyVault. Filename matches environment variable name,
        file content is the environment variable value.
        See https://docs.microsoft.com/en-us/azure/go/azure-sdk-go-authorization#use-environment-based-authentication
Files when type contains 'file':
Any number of files is allowed. The filename matches the secret name, the file contents is the secret value.

For example a secret named 'xyz' with value '{"name":"superman"}'
    {{ vault "xyz" "name" }} expands to superman
    {{ vault "xyz" "" }} expands to {"name":"superman"}

`

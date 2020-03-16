package main

import (
	"flag"
	"fmt"
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	"os"
	"path/filepath"
)

var (
	// Version as set during build.
	Version string

	mode = flag.String("m", "",
		`Mode is one of;
generate - write expanded templates to stdout
apply - generate templates and apply them
`)  //TODO "prune", "apply-prune" or allow -m apply,prune
	//TODO label = flag.String("l", "",
	//	`Label is a key=value that is added to all applied resources. Prune uses this label to delete resources.`)
	dryRun = flag.Bool("dry-run", false,
		`Dry-run prevents any change being made to the target cluster`)
	jobFile = flag.String("job-file", "",
		`Yaml file with steps to perform.`)
	setFile = flag.String("set-file", "",
		`Yaml file with values that override template values`)

	kubeContext = flag.String("context", "",
		`Equivalent of kubectl --context`)
	kubeConfig = flag.String("kubeconfig", "",
		`Equivalent of kubectl --kubeconfig`)
	kubeCtl = flag.String("kubectl", "kubectl",
		`The binary to access the target cluster with`)

	// Usage text argument: %[1]=program name, %[2]=program version.
	usage = `%[1]s %[2]s 
%[1]s reads a job file and performs the steps. A step can be one of:
    tmplt: expand a template with values and apply the result to a target k8s cluster.
    condition: wait until a target k8s cluster satisfies a condition.

Templating uses 'https://golang.org/pkg/text/template/' with 'http://masterminds.github.io/sprig/'
and additional templating functions; toToml, to/fromYaml, to/fromJson.

Templating examples:
    {{ .Files.Get "filename" }}
	
    {{ range $name, $content := .Files.Glob "examples/*.yaml" }}
      {{ filebase $name }}: |
    {{ $content | indent 4 }}{{ end }}
	
    {{ (.Files.Glob "examples/*.yaml").AsConfig | indent 4 }}
	
    {{ (.Files.Glob "secrets/*").AsSecrets }}

Beware, file access is not sanitized!

Usage: %[1]s [options...]
`
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, usage, filepath.Base(os.Args[0]), Version)
		flag.PrintDefaults()
	}
	// klog
	defer klog.Flush()
	klog.InitFlags(nil)
	flag.Set("alsologtostderr", "true")

	flag.Parse()

	log := klogr.New()

	if msg := validate(); len(msg) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}

	tl := tool.New(
		log,
		*kubeCtl,
		*kubeConfig,
		*kubeContext,
		os.Environ(),
		getMode(),
		*dryRun,
		*jobFile,
		*setFile,
	)
	err := tl.Run(os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Validate checks flags and environment variables.
// On validation failures it returns explanation(s).
func validate() string {
	var r string

	if *jobFile == "" {
		r = r + "-job-file should be defined.\n"
	}

	if getMode() == tool.ModeUnknown {
		r = r + "-m should be one of 'generate', 'apply'.\n"
	}

	return r
}

// GetMode returns tool.Mode based on the mode flag.
func getMode() tool.Mode {
	switch *mode {
	case "apply":
		return tool.ModeApply
	case "generate":
		return tool.ModeGenerate
	}
	return tool.ModeUnknown
}

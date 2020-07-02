package kubectl

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe"
)

// Opt are the command options.
type Opt struct {
	// ExeOpt optionally set a working directory, environment.
	ExeOpt *exe.Opt
	// Kubectl optionally selects the executable to use.
	KubeCtl string
	// KubeConfig optionally sets the kubeconfig file to use.
	KubeConfig string
	// KubeContext optionally sets the context to use.
	KubeContext string
	// Perform a dry-run.
	DryRun bool
}

// Run executes kubectl with 'stdin', 'args' and 'options' and returns stdout and stderr.
// Ctx is optional.
func Run(ctx context.Context, log logr.Logger, options *Opt, stdin string, args ...string) (stdout string, stderr string, err error) {
	c := "kubectl"
	var o *exe.Opt
	var a []string

	if options != nil {
		if options.ExeOpt != nil {
			o = options.ExeOpt
		}

		if options.KubeCtl != "" {
			c = options.KubeCtl
		}

		if options.KubeConfig != "" {
			a = append(a, "--kubeconfig", options.KubeConfig)
		}
		if options.KubeContext != "" {
			a = append(a, "--context", options.KubeContext)
		}
	}
	a = append(a, args...)

	if options != nil && options.DryRun {
		a = append(a, "--server-dry-run")
	}

	// run cmd.
	stdout, stderr, err = exe.Run(ctx, log, o, stdin, c, a...)

	return
}

func copySS(ss []string) []string {
	r := make([]string, len(ss))
	copy(r, ss)
	return r
}

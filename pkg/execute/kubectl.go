package execute

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe/kubectl"
)

// Kubectl can run kubectl cli.
type Kubectl struct {
	// Environ are the environment variables on kubectl invocation.
	Environ []string
	// Kubectl binary name and global arguments.
	KubeCtl, KubeConfig, KubeContext string

	Log logr.Logger
}

// Run kubectl.
func (k Kubectl) Run(ctx context.Context, stdin string, args ...string) (string, string, error) {
	return kubectl.Run(ctx, k.Log, k.kubectlOpt(), stdin, args...)
}

func (k Kubectl) kubectlOpt() *kubectl.Opt {
	return &kubectl.Opt{
		ExeOpt: &exe.Opt{
			Env: k.Environ,
		},
		KubeCtl:     k.KubeCtl,
		KubeConfig:  k.KubeConfig,
		KubeContext: k.KubeContext,
	}
}

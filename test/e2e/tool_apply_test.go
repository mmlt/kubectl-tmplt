package e2e_test

import (
	"github.com/go-logr/logr"
	"github.com/mmlt/kubectl-tmplt/pkg/execute"
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe/kubectl"
	"github.com/stretchr/testify/assert"
	"k8s.io/klog/klogr"
	"strings"
	"testing"
)

// TestApply runs the tool in 'apply' mode against a local cluster and check post conditions.
// kube/config current context selects the cluster to use.
func TestApply(t *testing.T) {
	log := klogr.New()

	var tests = []struct {
		// it describes what the test proves.
		it string
		// setup are the kubectl commands to prepare the target cluster for the test.
		setup []string
		// subject is what is tested.
		subject tool.Tool
		// postConditions that should be met upon completion.
		// See kubectl wait.
		postConditions []string
		// resources is a comma separated list of the required external resources to run a test
		resources string
	}{
		{
			it: "should_deploy_and_configure_vault_with_values_from_filevault",
			setup: []string{
				// test with a freshly created Vault (beware; deleting and recreating vault takes minutes)
				"-n kt-test delete vault vault",
				"-n kt-test wait --for=delete pod/vault-0",
				"-n kt-test delete pvc vault-file --wait",
				"-n kt-test delete secret vault-unseal-keys --wait",
			},
			subject: tool.Tool{
				Environ:     []string{},
				JobFilepath: "testdata/03/job.yaml",
				SetFilepath: "testdata/03/values.yaml",
				VaultPath:   "testdata/filevault",
				Execute: &execute.Execute{
					//TODO why is env needed? Environ:        []string{},
					Kubectl: execute.Kubectl{
						//TODO nil means use parent env
						//Environ:     []string{},
						Log: log,
					},
					//Out:            nil,
					Log: log,
				},
				Log: log,
			},
			postConditions: []string{
				//"wait pod -l app=example --for condition=Ready",
			},
			resources: "k8s",
		},
	}

	// prevent taking down a real cluster by accident.
	out, _, err := kubectl.Run(nil, log, &kubectl.Opt{}, "", "config", "current-context")
	assert.NoError(t, err)
	assert.Regexp(t, "minikube|microk8s|local", out, "current-context must refer to a local cluster")

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			if !available(tst.resources) {
				t.Skip("not all resource are available:", tst.resources)
				return
			}

			// clean
			setup(t, tst.setup, log)

			// run
			err = tst.subject.Run()
			assert.NoError(t, err)

			// check conditions.
			for _, cmd := range tst.postConditions {
				cmd := strings.Split(cmd, " ")
				sout, _, err := kubectl.Run(nil, log, nil, "", cmd...)
				assert.NoError(t, err)
				assert.Contains(t, sout, "condition met")
			}

			// leave stuff running as-is...
		})
	}
}

// Setup runs kubectl cmds against target cluster.
func setup(t *testing.T, cmds []string, log logr.Logger) {
	for _, cmd := range cmds {
		cmd := strings.Split(cmd, " ")
		_, _, err := kubectl.Run(nil, log, nil, "", cmd...)
		if err == nil || strings.Contains(err.Error(), "Error from server (NotFound):") {
			continue
		}
		assert.NoError(t, err)
	}
}

// test templates that use values from yaml file(s).
package e2e_test

import (
	"bytes"
	"flag"
	"github.com/mmlt/kubectl-tmplt/pkg/execute"
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"k8s.io/klog/klogr"
	"path/filepath"
	"strings"
	"testing"
)

// Resources is a list of external resources that are available for tests.
// Tests that require resources that are not listed in --resources are skipped.
var resources = flag.String("resources", "", `A comma separated list of external resources available to tests.
Valid values: 
    k8s - A local Kubernetes cluster (addressed by kube/config current-context)
    keyvault - An Azure KeyVault is configured in keyvault/. with the same secrets as those in filevault/ 
		(The Key Vault must be accessible for the user running the test.)
`)

const (
	resourceK8s      = "k8s"
	resourceKeyVault = "keyvault"
)

func hasResources(required string) bool {
	for _, req := range strings.Split(required, ",") {
		if !strings.Contains(*resources, req) {
			return false
		}
	}
	return true
}

// TestGenerate runs the tool in 'generate' mode and compares the generated output to golden files.
// Test for which not all resources are available are skipped, see --resources.
func TestGenerate(t *testing.T) {
	log := klogr.New()
	var got bytes.Buffer

	var tests = []struct {
		// it describes what the test proves.
		it string
		// setValues override all other values.
		setValues yamlx.Values
		// subject is what is tested.
		subject tool.Tool
		// errString is the expected error
		errString string
		// resources is a comma separated list of the required external resources to run a test
		resources string
	}{
		{
			it: "should_generate_output_for_the_example_template",
			subject: tool.Tool{
				Mode:          tool.ModeGenerate,
				Environ:       []string{},
				JobFilepath:   "testdata/00/simple-job.yaml",
				ValueFilepath: "testdata/00/values.yaml",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
		},
		{
			it: "should_generate_output_for_simple_cluster_config",
			subject: tool.Tool{
				Mode:          tool.ModeGenerate,
				Environ:       []string{},
				JobFilepath:   "testdata/01/cluster/example-job.yaml",
				ValueFilepath: "testdata/01/cluster/values.yaml",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
		},
		{
			it: "should_generate_output_with_filevault_secret",
			subject: tool.Tool{
				Mode:        tool.ModeGenerate,
				Environ:     []string{},
				JobFilepath: "testdata/00/vault-job.yaml",
				VaultPath:   "testdata/filevault",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
		},
		{
			it: "should_generate_output_with_keyvault_secret",
			subject: tool.Tool{
				Mode:        tool.ModeGenerate,
				Environ:     []string{},
				JobFilepath: "testdata/00/vault-job.yaml",
				VaultPath:   "testdata/keyvault",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
			resources: resourceKeyVault,
		},
		{
			it: "should_generate_errors_when_secret_or_secret_field_are_missing",
			subject: tool.Tool{
				Mode:        tool.ModeGenerate,
				Environ:     []string{},
				JobFilepath: "testdata/00/vault-error-job.yaml",
				VaultPath:   "testdata/filevault",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
			errString: `2 errors occurred:
	* not found: missing-secret
	* not found: missing

`,
		},
		{
			//it: "should_generate_output_for_all_steps_in_mode-generate-with-actions",
			it: "should_error_in_mode-generate-with-actions_because_no_cluster_is_available",
			subject: tool.Tool{
				Mode:          tool.ModeGenerateWithActions,
				Environ:       []string{},
				JobFilepath:   "testdata/03/job.yaml",
				ValueFilepath: "testdata/03/values.yaml",
				VaultPath:     "testdata/filevault",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
			errString: `expand act/set-vault-kv.yaml: execute: template: input:4:10: executing "input" at <index .Get "secret" .Values.namespace "vault-unseal-keys" "data" "vault-root">: error calling index: index of nil pointer`,
		},
		{
			it: "should_skip_actions_in_mode-generate",
			subject: tool.Tool{
				Mode:          tool.ModeGenerate,
				Environ:       []string{},
				JobFilepath:   "testdata/03/job.yaml",
				ValueFilepath: "testdata/03/values.yaml",
				VaultPath:     "testdata/filevault",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
		},
		{
			it: "should_expand_variables_in_job",
			subject: tool.Tool{
				Mode:          tool.ModeGenerate,
				Environ:       []string{},
				JobFilepath:   "testdata/04/job.yaml",
				ValueFilepath: "testdata/04/values.yaml",
				Execute: &execute.Execute{
					Kubectl: execute.Kubectl{
						Log: log,
					},
					Out: &got,
					Log: log,
				},
				Log: log,
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			if !hasResources(tst.resources) {
				t.Skip("this test requires resource(s) (check --resources flag):", tst.resources)
				return
			}

			got.Reset()

			err := tst.subject.Run(tst.setValues)
			if tst.errString != "" {
				if assert.Error(t, err) {
					assert.Equal(t, tst.errString, err.Error(), "expect error")
				}
				return
			}
			if assert.NoError(t, err) {
				// read golden file to compare got output with
				p := filepath.Join("testdata/golden", tst.it)
				gld, _ := ioutil.ReadFile(p + ".golden")
				if bytes.Compare(gld, got.Bytes()) != 0 {
					assert.True(t, false, "result doesn't match .golden file, please review %s and rename to .golden to approve", p)
					// write output
					err = ioutil.WriteFile(p, got.Bytes(), 0o666)
					assert.NoError(t, err)
				}
			}
		})
	}
}

// test templates that use values from yaml file(s).
package e2e_test

import (
	"bytes"
	"flag"
	"github.com/mmlt/kubectl-tmplt/pkg/execute"
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"k8s.io/klog/klogr"
	"path/filepath"
	"strings"
	"testing"
)

// Resources is a list of external resources that are available for tests.
var resources = flag.String("resources", "", `A comma separated list of external resources available to tests.
Valid values: 
    k8s - A local Kubernetes cluster (addressed by kube/config current-context)
    keyvault - An Azure KeyVault is configured in keyvault/. The KeyVault contains the same secrets as those in filevault/
`)

func available(required string) bool {
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
		// subject is what is tested.
		subject tool.Tool
		// resources is a comma separated list of the required external resources to run a test
		resources string
	}{
		{
			it: "should_generate_output_for_the_example_template",
			subject: tool.Tool{
				Environ:     []string{},
				JobFilepath: "testdata/00/job.yaml",
				SetFilepath: "testdata/00/values.yaml",
				//TODO Mode: tool.ModeGenerate,
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
				Environ:     []string{},
				JobFilepath: "testdata/01/cluster/example-job.yaml",
				SetFilepath: "testdata/01/cluster/values.yaml",
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
			it: "should_generate_output_for_all_step_and_action_types",
			subject: tool.Tool{
				Environ:     []string{},
				JobFilepath: "testdata/03/job.yaml",
				SetFilepath: "testdata/03/values.yaml",
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
			resources: "keyvault",
		},
	}

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			if !available(tst.resources) {
				t.Skip("not all resource are available:", tst.resources)
				return
			}

			got.Reset()

			err := tst.subject.Run()
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

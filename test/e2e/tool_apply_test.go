// e2e testing of tool using a local k8s cluster.
package e2e_test

import (
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"github.com/mmlt/kubectl-tmplt/pkg/util/exe/kubectl"
	"github.com/stretchr/testify/assert"
	"k8s.io/klog/klogr"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//
func TestApply(t *testing.T) {
	var tests = map[string]struct {
		// setup are the kubectl commands to prepare the target cluster for the test.
		setup []string
		// testdata subdirectory used by this test.
		testdata string
		// jobfile relative to testdata.
		jobFile string
		// valuesFile relative to testdata.
		valuesFile string
		// postConditions that should be met upon completion.
		// See kubectl wait.
		postConditions []string
	}{
		"example": {
			setup: []string{
				"delete pod -l app=example --wait",
			},
			testdata:   "00",
			jobFile:    "job.yaml",
			valuesFile: "values.yaml",
			postConditions: []string{
				"wait pod -l app=example --for condition=Ready",
			},
		},
		"create_ingress": {
			setup: []string{
				"delete ns ingress-nginx --wait",
			},
			testdata:   "01",
			jobFile:    "cluster/05-ingress.yaml",
			valuesFile: "cluster/values.yaml",
			postConditions: []string{
				"wait -n ingress-nginx deployment default-http-backend-in --for condition=Available",
				"wait -n ingress-nginx deployment default-http-backend-ex --for condition=Available",
				"wait -n ingress-nginx deployment nginx-ingress-controller-in --for condition=Available",
				"wait -n ingress-nginx deployment nginx-ingress-controller-ex --for condition=Available",
			},
		},
	}

	log := klogr.New()

	// safety check to prevent taking down a real cluster.
	out, _, err := kubectl.RunTxt(log, &kubectl.Opt{}, "", "config", "current-context")
	assert.NoError(t, err)
	assert.Regexp(t, "minikube|microk8s|local", out, "current-context must refer to a local cluster")

	for name, tst := range tests {
		t.Run(name, func(t *testing.T) {
			// create tmp directory for testdata.
			tf := testFilesNew()
			defer tf.MustRemoveAll()
			dir, err := os.Getwd()
			assert.NoError(t, err)
			tf.MustCopy(filepath.Join(dir, "testdata", tst.testdata), tst.testdata)

			// setup (ignoring 'not found' errors).
			for _, cmd := range tst.setup {
				cmd := strings.Split(cmd, " ")
				_, _, _ = kubectl.RunTxt(log, nil, "", cmd...)
			}

			// run tool.
			tl := tool.New(
				log,
				"", "", "",
				os.Environ(),
				tool.ModeApply,
				false,
				tf.Path(tst.testdata, tst.jobFile),
				tf.Path(tst.testdata, tst.valuesFile))
			err = tl.Run(os.Stdout)
			assert.NoError(t, err)

			// check conditions.
			for _, cmd := range tst.postConditions {
				cmd := strings.Split(cmd, " ")
				sout, _, err := kubectl.RunTxt(log, nil, "", cmd...)
				assert.NoError(t, err)
				assert.Contains(t, sout, "condition met")
			}

			// leave stuff running as-is...
		})
	}
}

/*// GetNamespaces
func getNamespaces(log logr.Logger) ([]string, error) {
	json, err := kubectl.Run(log, nil, "", "get", "ns")
	if err != nil {
		return nil, err
	}
	jq, err := gojq.Parse(".items[].metadata.name")
	if err != nil {
		return nil, err
	}

	var result []string
	itr := jq.Run(json)
	for v, ok := itr.Next(); ok; v, ok = itr.Next() {
		if err, ok := v.(error); ok {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}*/

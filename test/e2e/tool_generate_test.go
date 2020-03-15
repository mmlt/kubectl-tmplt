// test templates that use values from yaml file(s).
package e2e_test

import (
	"bytes"
	"github.com/go-logr/stdr"
	"github.com/mmlt/kubectl-tmplt/pkg/tool"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

//
func TestGenerate(t *testing.T) {
	var tests = map[string]struct {
		// testdata subdirectory used by this test.
		testdata string
		// jobfile relative to testdata.
		jobFile string
		// valuesFile relative to testdata.
		valuesFile string
		// wantOut is the expected output.
		wantOut string
	}{
		"Example": {
			testdata:   "00",
			jobFile:    "job.yaml",
			valuesFile: "values.yaml",
			wantOut: `---
##01: InstrApply [apply -f -] template.yaml
apiVersion: v1
kind: Pod
metadata:
  name: "test"
  namespace: "default"
  labels:
    app: example
spec:
  containers:
    - args: [sleep, "3600"]
      image: ubuntu
      name: ubuntu

---
##02: InstrWait [wait --for condition=Ready pod -l app=example] 

`,
		},
		"Simple": {
			testdata:   "01",
			jobFile:    "cluster/01-cpenamespaces.yaml",
			valuesFile: "cluster/values.yaml",
			wantOut: `---
##01: InstrApply [apply -f -] namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
  labels:      
    openpolicyagent.org/webhook: ignore


---
##02: InstrApply [apply -f -] namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: cpe-system
  labels:
    owner: example
    costcenter: tbd
    environment: local  
    mmlt.nl/gitops: k8s-clusters-addons.cpenamespaces      
    openpolicyagent.org/webhook: ignore


---
##03: InstrApply [apply -f -] namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: cpe
  labels:
    owner: example
    costcenter: tbd
    environment: local  
    mmlt.nl/gitops: k8s-clusters-addons.cpenamespaces      
    openpolicyagent.org/webhook: ignore


`,
		},
	}

	log := stdr.New(nil)

	for name, tst := range tests {
		t.Run(name, func(t *testing.T) {
			// create testdata in tmp.
			tf := testFilesNew()
			defer tf.MustRemoveAll()
			dir, err := os.Getwd()
			assert.NoError(t, err)
			tf.MustCopy(filepath.Join(dir, "testdata", tst.testdata), tst.testdata)

			// run tool.
			var out bytes.Buffer
			tl := tool.New(
				log,
				"", "", "",
				os.Environ(),
				tool.ModeGenerate,
				true,
				tf.Path(tst.testdata, tst.jobFile),
				tf.Path(tst.testdata, tst.valuesFile))
			err = tl.Run(&out)
			assert.NoError(t, err)

			// assert
			assert.Equal(t, tst.wantOut, out.String())
		})
	}
}

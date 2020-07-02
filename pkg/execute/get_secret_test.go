package execute

import (
	"context"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecute_getSecret(t *testing.T) {
	tests := []struct {
		it   string
		doc  string
		fake fakeKubectl
		want yamlx.Values
	}{
		{
			it: "should_get_a_secret_and_parse_output",
			doc: `type: getSecret
namespace: default
name: test
`,
			fake: fakeKubectl{
				stdout: testSecret("default", "test"),
			},
			want: yamlx.Values{
				"secret": map[string]interface{}{
					"default": map[string]interface{}{
						"test": map[string]interface{}{
							"data": map[string]interface{}{
								"een":  "Zmlyc3Qta3YtdmFsdWU=",
								"twee": "c2Vjb25kLWt2LXZhbHVl",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			x := &Execute{
				Kubectl: tt.fake,
				//Log:            tt.fields.Log,
			}
			passedValues := &yamlx.Values{}
			err := x.getSecret(0, "name", []byte(tt.doc), "", passedValues)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, *passedValues)
		})
	}
}

type fakeKubectl struct {
	stdout, stderr string
	err            error
}

func (k fakeKubectl) Run(ctx context.Context, stdin string, args ...string) (string, string, error) {
	return k.stdout, k.stderr, k.err
}

func testSecret(namespace, name string) string {
	return `{
    "apiVersion": "v1",
    "data": {
        "een": "Zmlyc3Qta3YtdmFsdWU=",
        "twee": "c2Vjb25kLWt2LXZhbHVl"
    },
    "kind": "Secret",
    "metadata": {
        "creationTimestamp": "2020-05-18T17:11:11Z",
        "name": name,
        "namespace": namespace,
        "resourceVersion": "1071507",
    },
    "type": "Opaque"
}`
}

package execute

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_updateObjectYaml(t *testing.T) {
	type args struct {
		doc    string
		labels map[string]string
	}
	tests := []struct {
		it      string
		args    args
		wantDoc string
		wantKNN KindNamespaceName
	}{
		{
			it: "should handle Secret with data",
			args: args{
				doc: `
apiVersion: v1
kind: Secret
metadata:
  name: envop-sp
type: Opaque
data:
  hello: d29ybGQ=`,
				labels: map[string]string{"key": "value"},
			},
			wantDoc: `apiVersion: v1
data:
  hello: d29ybGQ=
kind: Secret
metadata:
  labels:
    key: value
  name: envop-sp
type: Opaque
`,
			wantKNN: KindNamespaceName{GVK: metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, Namespace: "", Name: "envop-sp"},
		},
		{
			it: "should handle Secret with stringData",
			args: args{
				doc: `
apiVersion: v1
kind: Secret
metadata:
  name: envop-sp
type: Opaque
stringData:
  hello: world`,
				labels: map[string]string{"key": "value"},
			},
			wantDoc: `apiVersion: v1
kind: Secret
metadata:
  labels:
    key: value
  name: envop-sp
stringData:
  hello: world
type: Opaque
`,
			wantKNN: KindNamespaceName{GVK: metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, Namespace: "", Name: "envop-sp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			gotDoc, gotKNN, err := updateObjectYaml([]byte(tt.args.doc), tt.args.labels)
			if assert.NoError(t, err) {
				assert.Equal(t, tt.wantDoc, string(gotDoc))
				assert.Equal(t, tt.wantKNN, gotKNN)
			}
		})
	}
}

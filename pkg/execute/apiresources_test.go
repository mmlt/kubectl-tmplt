package execute

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_textToAPIResources(t *testing.T) {
	tests := []struct {
		it   string
		arg  string
		want []metav1.APIResource
	}{
		{
			it: "should read the output of v1.18 kubectl api-resources",
			arg: `NAME                              SHORTNAMES           APIGROUP                       NAMESPACED   KIND
bindings                                                                              true         Binding
componentstatuses                 cs                                                  false        ComponentStatus
pods                              po                                                  true         Pod
deployments                       deploy               apps                           true         Deployment
customresourcedefinitions         crd,crds             apiextensions.k8s.io           false        CustomResourceDefinition
`,
			want: []metav1.APIResource{
				{Name: "bindings", Namespaced: true, Group: "", Version: "", Kind: "Binding"},
				{Name: "componentstatuses", Namespaced: false, Group: "", Version: "", Kind: "ComponentStatus"},
				{Name: "pods", Namespaced: true, Group: "", Version: "", Kind: "Pod"},
				{Name: "deployments", Namespaced: true, Group: "apps", Version: "", Kind: "Deployment"},
				{Name: "customresourcedefinitions", Namespaced: false, Group: "apiextensions.k8s.io", Version: "", Kind: "CustomResourceDefinition"},
			},
		},
		{
			it: "should read the output of v1.20 kubectl api-resources",
			arg: `NAME                                      SHORTNAMES      APIVERSION                             NAMESPACED   KIND
bindings                                                  v1                                     true         Binding
componentstatuses                         cs              v1                                     false        ComponentStatus
pods                                      po              v1                                     true         Pod
deployments                               deploy          apps/v1                                true         Deployment
adcsrequests                                              adcs.certmanager.csf.nokia.com/v1      true         AdcsRequest
`,
			want: []metav1.APIResource{
				{Name: "bindings", Namespaced: true, Group: "", Version: "v1", Kind: "Binding"},
				{Name: "componentstatuses", Namespaced: false, Group: "", Version: "v1", Kind: "ComponentStatus"},
				{Name: "pods", Namespaced: true, Group: "", Version: "v1", Kind: "Pod"},
				{Name: "deployments", Namespaced: true, Group: "apps", Version: "v1", Kind: "Deployment"},
				{Name: "adcsrequests", Namespaced: true, Group: "adcs.certmanager.csf.nokia.com", Version: "v1", Kind: "AdcsRequest"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			got, err := textToAPIResources(tt.arg)
			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

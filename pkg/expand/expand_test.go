package expand

import (
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"github.com/stretchr/testify/assert"
	"testing"
	"text/template"
)

func TestRun(t *testing.T) {
	tests := []struct {
		it       string
		env      []string
		doc      string
		values   yamlx.Values
		passed   yamlx.Values
		customFn template.FuncMap
		want     string
	}{
		{
			it:  "can_use_a_passed_value_via_Get",
			doc: `{{ .Get.name }}`, //NB. key can not contain - (dash)
			passed: map[string]interface{}{
				"name": "hotstuff",
			},
			want: "hotstuff",
		},
		{
			it:  "can_use_key_containing_dash",
			doc: `{{ index .Values "dash-ed" "name" }}`,
			values: map[string]interface{}{
				"dash-ed": map[string]interface{}{
					"name": "peppers",
				},
			},
			want: "peppers",
		},
		{
			it:  "can_do_chained_lookups",
			doc: `{{ index .Get .Values.namespace "name" }}`,
			values: map[string]interface{}{
				"namespace": "dash-ed",
			},
			passed: map[string]interface{}{
				"dash-ed": map[string]interface{}{
					"name": "peppers",
				},
			},
			want: "peppers",
		},
		{
			it:  "can_use_indexOrDefault_to_lookup_name",
			doc: `{{ indexOrDefault "default" .Values "dash-ed" "name" }}`,
			values: map[string]interface{}{
				"dash-ed": map[string]interface{}{
					"name": "peppers",
				},
			},
			want: "peppers",
		},
		{
			it:  "returns_default_when_element_is_not_found",
			doc: `{{ indexOrDefault "default" .Values "does" "not" "exist" }}`,
			values: map[string]interface{}{
				"dash-ed": map[string]interface{}{
					"name": "peppers",
				},
			},
			want: "default",
		},
		{
			it:     "returns_default_when_value_is_nil",
			doc:    `{{ indexOrDefault "default" .Values "does" "not" "exist" }}`,
			values: nil,
			want:   "default",
		},
		{
			it:     "returns_default_when_value_is_nil_and_no_path_is_given",
			doc:    `{{ indexOrDefault "default" .Values }}`,
			values: nil,
			want:   "default",
		},
		{
			it:     "honours_escape_chars_in_default",
			doc:    `{{ indexOrDefault "\"\"" .Values "does" "not" "exist" }}`,
			values: nil,
			want:   "\"\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			got, err := Run(tt.env, "testdata", []byte(tt.doc), tt.values, tt.passed, tt.customFn)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

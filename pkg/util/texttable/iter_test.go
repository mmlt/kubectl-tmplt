package texttable

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTable_RowIterGetColByName(t *testing.T) {
	tests := []struct {
		it     string
		table  *Table
		column string
		want   []string
	}{
		{
			it: "should_handle_missing_columns",
			table: &Table{
				Header: Header{Pos{Text: "NAME"}, Pos{Text: "SHORTNAMES"}, Pos{Text: "APIGROUP"}, Pos{Text: "NAMESPACED"}, Pos{Text: "KIND"}},
				Rows: [][]string{
					{"configmaps", "cm", "", "true", "ConfigMap"},
					{"customresourcedefinitions", "crd,crds", "apiextensions.k8s.io", "false", "CustomResourceDefinition"},
					{"jobs", "", "batch", "true", "Job"},
					{"secrets", "", "", "true", "Secret"},
				},
			},
			column: "KIND",
			want:   []string{"ConfigMap", "CustomResourceDefinition", "Job", "Secret"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			iter := tt.table.RowIter()
			var got []string
			for iter.Next() {
				s, ok := iter.GetColByName("KIND")
				assert.True(t, ok, "GetColumnsByName")
				got = append(got, s)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

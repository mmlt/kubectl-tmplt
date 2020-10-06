package texttable

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	tests := []struct {
		it      string
		text    string
		minCols int
		want    *Table
	}{
		{
			it: "should_handle_missing_columns",
			text: `NAME                       SHORTNAMES   APIGROUP               NAMESPACED   KIND
configmaps                  cm                                  true         ConfigMap
customresourcedefinitions   crd,crds     apiextensions.k8s.io   false        CustomResourceDefinition
jobs                                     batch                  true         Job
secrets                                                         true         Secret
`,
			minCols: 1,
			want: &Table{
				Header: Header{Pos{Text: "NAME", Col: 26}, Pos{Text: "SHORTNAMES", Col: 39}, Pos{Text: "APIGROUP", Col: 62}, Pos{Text: "NAMESPACED", Col: 75}, Pos{Text: "KIND", Col: -1}},
				Rows: [][]string{
					{"configmaps", "cm", "", "true", "ConfigMap"},
					{"customresourcedefinitions", "crd,crds", "apiextensions.k8s.io", "false", "CustomResourceDefinition"},
					{"jobs", "", "batch", "true", "Job"},
					{"secrets", "", "", "true", "Secret"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			r := strings.NewReader(tt.text)
			got := Read(r, tt.minCols)
			assert.Equal(t, tt.want, got)
			//if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("Read() = %v, want %v", got, tt.want)
			//}
		})
	}
}

func TestWrite(t *testing.T) {
	tests := []struct {
		it      string
		table   *Table
		columns []int
		sep     string
		hdr     bool
		want    string
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
			columns: nil,
			sep:     "   ",
			hdr:     true,
			want: `NAME                        SHORTNAMES   APIGROUP               NAMESPACED   KIND
configmaps                  cm                                  true         ConfigMap
customresourcedefinitions   crd,crds     apiextensions.k8s.io   false        CustomResourceDefinition
jobs                                     batch                  true         Job
secrets                                                         true         Secret
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			b := &bytes.Buffer{}
			Write(tt.table, tt.columns, tt.sep, tt.hdr, b)
			got := b.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

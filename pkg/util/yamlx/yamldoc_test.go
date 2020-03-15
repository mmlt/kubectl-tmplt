package yamlx

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

func TestSplit(t *testing.T) {
	var tsts = []struct {
		in   string
		want [][]byte
	}{
		{
			in: `
aa
---
bb
`,
			want: [][]byte{
				[]byte(
					`
aa`),
				[]byte(
					`bb
`),
			},
		},
	}

	for _, tst := range tsts {
		got, err := SplitDoc([]byte(tst.in))
		assert.NoError(t, err)
		assert.Equal(t, tst.want, got)
	}
}

func TestIsEmpty(t *testing.T) {
	var tsts = []struct {
		in   string
		want bool
	}{
		{
			in: `
aa
---
bb
`,
			want: false,
		},
		{ // comment is no content.
			in: `#
`,
			want: true,
		},

		{ // separator is no content.
			in: `---
`,
			want: true,
		},
	}

	for _, tst := range tsts {
		got := IsEmpty([]byte(tst.in))
		assert.Equal(t, tst.want, got)
	}
}

func TestSplitLarge(t *testing.T) {
	var tsts = []struct {
		file    string
		wantDoc int
	}{
		{
			file:    "testdata/cert-manager.yaml",
			wantDoc: 50 + 1, //grep "\-\-\-" ./pkg/yamlx/testdata/cert-manager.yaml | wc
		},
	}
	for _, tst := range tsts {
		b, err := ioutil.ReadFile(tst.file)
		assert.NoError(t, err)

		ds, err := SplitDoc(b)
		assert.NoError(t, err)
		assert.Equal(t, tst.wantDoc, len(ds))
	}
}

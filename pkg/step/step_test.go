package step

import (
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIterator(t *testing.T) {
	in := []byte(`
steps:
- tmplt: "path/to/template"
- wait: "xyz"
defaults:
  one: 1
  two: 2
`)
	wantValues := yamlx.Values{
		"one": 1,
		"two": 2,
	}
	wantSteps := []interface{}{
		&Tmplt{Tmplt: "path/to/template"},
		&Wait{C: "xyz"},
	}

	itr, err := Iterator(in)
	assert.NoError(t, err)

	assert.Equal(t, wantValues, itr.Defaults())

	got := []interface{}{}
	for step := itr.Next(); step != nil; step = itr.Next() {
		got = append(got, step)
	}
	assert.Equal(t, wantSteps, got)
}

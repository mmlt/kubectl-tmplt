package tool

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

// See test/e2e for tests that use the file system.

var TestGenerateData = map[string]struct {
	values    string
	jobText   string
	templates map[string]string
	want      instructions
}{
	// SingleTmplWithTmplScopedValues is a job with one template step and only template scoped values.
	"SingleTmplWithTmplScopedValues": {
		jobText: `
steps:
- tmplt: tpl/example.txt
  values:
    audience: all
    team:
      lead: pipo`,
		templates: map[string]string{
			"tpl/example.txt": `
{{ .Values.team.lead }} says hello {{ .Values.audience }}!`,
		},
		want: instructions{
			items: []instruction{
				{
					typ: InstrApply,
					input: `
pipo says hello all!`,
					args:   []string{"apply", "-f", "-"},
					id:     1,
					origin: "example.txt",
				},
			},
			count: 1,
		},
	},

	// SingleTmplWithGlobalJobTmplScopedValues is a job with one template step and global, job and template scoped values.
	"SingleTmplWithGlobalJobTmplScopedValues": {
		values: `
audience: world
`,
		jobText: `
steps:
- tmplt: tpl/example.txt
  values:
    team:
      lead: pipo
defaults:
  audience: all
  team:
    lead: klukkluk`,
		templates: map[string]string{
			"tpl/example.txt": `
{{ .Values.team.lead }} says hello {{ .Values.audience }}!`,
		},
		want: instructions{
			items: []instruction{
				{
					typ: InstrApply,
					input: `
pipo says hello world!`,
					args:   []string{"apply", "-f", "-"},
					id:     1,
					origin: "example.txt",
				},
			},
			count: 1,
		},
	},
}

func TestGenerate(t *testing.T) {
	for name, tst := range TestGenerateData {
		t.Run(name, func(t *testing.T) {
			// create function to read template file content.
			readFile := func(path string) (string, []byte, error) {
				s, ok := tst.templates[path]
				if !ok {
					return "", nil, fmt.Errorf("not found: %s", path)
				}
				return path, []byte(s), nil
			}

			// run tool.
			tl := tool{
				environ:    []string{},
				readFileFn: readFile,
			}
			got, err := tl.generate([]byte(tst.values), []byte(tst.jobText))
			assert.NoError(t, err)
			assert.Equal(t, tst.want, got)
		})
	}
}

package tool

import (
	"fmt"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
)

// See test/e2e for tests that use the file system.

// TestTool tests without accessing filesystem, target cluster or master vault.
func TestTool(t *testing.T) {
	tests := []struct {
		it           string
		globalValues string
		job          string
		templates    map[string]string
		vault        getter
		want         *fakeDoer
	}{
		{
			it: "should_apply_one_doc_with_tmplt_scoped_values",
			job: `
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
			want: &fakeDoer{
				apply: []string{"\npipo says hello all!"},
			},
		},

		{
			it: "should_apply_one_doc_with_global_and_tmplt_scoped_values",
			globalValues: `
audience: world
`,
			job: `
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
			want: &fakeDoer{
				apply: []string{"\npipo says hello world!"},
			},
		},

		{
			it: "should_wait",
			job: `
steps:
- wait: --one 1 --two 2`,
			want: &fakeDoer{
				wait: []string{"--one 1 --two 2"},
			},
		},

		{
			it: "should_handle_action_with_portforward_arg",
			job: `
steps:
- action: action/get.txt
  portForward: --forward-flags
  values:
    action: getSecret`,
			templates: map[string]string{
				"action/get.txt": `
action: {{ .Values.action }}`,
			},
			want: &fakeDoer{
				action:       []string{"\naction: getSecret"},
				portForward:  []string{"--forward-flags"},
				passedValues: yamlx.Values{},
				actionTally:  1,
			},
		},

		{
			it: "should_handle_action_with_passed_values",
			job: `
steps:
- action: action/nop.txt
- action: action/value.txt`,
			templates: map[string]string{
				"action/nop.txt": `
no operation`,
				"action/value.txt": `
tally: {{ .Get.tally }}`,
			},
			want: &fakeDoer{
				action:       []string{"\nno operation", "\ntally: 1"},
				portForward:  []string{"", ""},
				passedValues: yamlx.Values{"tally": 1},
				actionTally:  2,
			},
		},

		{
			it: "should_handle_reads_from_vault",
			job: `
steps:
- tmplt: tpl/vault.txt`,
			templates: map[string]string{
				"tpl/vault.txt": `
secret: {{ vault "object" "field" }}`,
			},
			vault: &fakeVault{
				"object/field": "value",
			},
			want: &fakeDoer{
				apply: []string{"\nsecret: value"},
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.it, func(t *testing.T) {
			// create function to read template file content.
			readFile := func(path string) (string, []byte, error) {
				s, ok := tst.templates[path]
				if !ok {
					return "", nil, fmt.Errorf("not found: %s", path)
				}
				return path, []byte(s), nil
			}

			m := &fakeDoer{}

			tl := Tool{
				Environ:    []string{},
				Execute:    m,
				readFileFn: readFile,
				vault:      tst.vault,
			}

			err := tl.run([]byte(tst.globalValues), []byte(tst.job))
			assert.NoError(t, err)
			assert.Equal(t, tst.want, m)
		})
	}
}

//
type fakeDoer struct {
	wait         []string
	apply        []string
	action       []string
	portForward  []string
	passedValues yamlx.Values
	actionTally  int
}

var _ Executor = &fakeDoer{}

func (m *fakeDoer) Wait(id int, flags string) error {
	m.wait = append(m.wait, flags)
	return nil
}

func (m *fakeDoer) Apply(id int, name string, doc []byte) error {
	m.apply = append(m.apply, string(doc))
	return nil
}

func (m *fakeDoer) Action(id int, name string, doc []byte, portForward string, passedValues *yamlx.Values) error {
	m.action = append(m.action, string(doc))
	m.portForward = append(m.portForward, portForward)
	m.passedValues = *passedValues

	m.actionTally++
	*passedValues = yamlx.Values{
		"tally": m.actionTally,
	}

	return nil
}

//
type fakeVault map[string]string

var _ getter = fakeVault{}

func (m fakeVault) Get(key, field string) string {
	p := path.Join(key, field)
	return m[p]
}

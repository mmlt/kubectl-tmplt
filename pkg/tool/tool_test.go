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
		mode         Mode
		job          string
		setValues    yamlx.Values
		globalValues string
		templates    map[string]string
		vault        getter
		want         *fakeDoer
	}{
		{
			it:   "should_apply_one_doc_with_tmplt_scoped_values",
			mode: ModeGenerate,
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
			it:   "should_apply_one_doc_with_global_and_tmplt_scoped_values",
			mode: ModeGenerate,
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
			globalValues: `
audience: world
`,
			templates: map[string]string{
				"tpl/example.txt": `
{{ .Values.team.lead }} says hello {{ .Values.audience }}!`,
			},
			want: &fakeDoer{
				apply: []string{"\npipo says hello world!"},
			},
		},

		{
			it:   "should_apply_one_doc_with_setvalue_overriding_all_others",
			mode: ModeGenerate,
			job: `
steps:
- tmplt: tpl/example.txt
  values:
    name: pipo
defaults:
  name: klukkluk`,
			setValues: yamlx.Values{"name": "dikkedeur"},
			globalValues: `
name: mamaloe
`,
			templates: map[string]string{
				"tpl/example.txt": `
{{ .Values.name }}`,
			},
			want: &fakeDoer{
				apply: []string{"\ndikkedeur"},
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
			it:   "should_handle_action_with_portforward_arg",
			mode: ModeGenerateWithActions,
			job: `
steps:
- action: action/get.txt
  portForward: --forward-flags
  values:
    type: getSecret`,
			templates: map[string]string{
				"action/get.txt": `
type: {{ .Values.type }}`,
			},

			want: &fakeDoer{
				action:       []string{"\ntype: getSecret"},
				portForward:  []string{"--forward-flags"},
				passedValues: yamlx.Values{},
				actionTally:  1,
			},
		},

		{
			it:   "should_handle_action_with_passed_values",
			mode: ModeGenerateWithActions,
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
			it:   "should_handle_reads_from_vault",
			mode: ModeGenerateWithActions,
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

		{
			it:   "should_expand_variables_in_job",
			mode: ModeGenerate,
			job: `
steps:
- tmplt: tpl/example.txt
  values:
    text: "{{ .Values.first }}" # note the quotes to make this valid yaml (arguably)
defaults:
  first: "hello"
`,
			templates: map[string]string{
				"tpl/example.txt": `text={{ .Values.text }}`,
			},
			want: &fakeDoer{
				apply: []string{"text=hello"},
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
				Mode:       tst.mode,
				Environ:    []string{},
				Execute:    m,
				readFileFn: readFile,
				vault:      tst.vault,
			}

			err := tl.run(tst.setValues, []byte(tst.globalValues), []byte(tst.job))
			if assert.NoError(t, err) {
				assert.Equal(t, tst.want, m)
			}
		})
	}
}

// FakeDoer records calls and provides return values.
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

// FakeVault provides a map based vault.
type fakeVault map[string]string

var _ getter = fakeVault{}

func (m fakeVault) Get(key, field string) string {
	p := path.Join(key, field)
	return m[p]
}

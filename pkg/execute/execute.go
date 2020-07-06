package execute

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/kubectl-tmplt/pkg/util/backoff"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
	"io"
	"strconv"
	"strings"
	"time"
)

// Execute executes Action, Apply, Wait steps.
type Execute struct {
	// DryRun prevents making changes to the target cluster.
	DryRun bool

	// Environ are the environment variables on Tool invocation.
	Environ []string

	// Kubectl knows how to invoke 'kubectl'
	Kubectl Kubectler

	// Out is the stream to send steps to in a format that is 'kubectl apply -f -' consumable.
	// Setting Out prevents any other processing (like 'wait') to take place.
	Out io.Writer

	Log logr.Logger
}

// Kubectler provides methods to invoke kubectl.
type Kubectler interface {
	Run(ctx context.Context, stdin string, args ...string) (string, string, error)
}

// Wait waits for target cluster conditions specified by flags to become true.
func (x *Execute) Wait(id int, flags string) error {
	args := append([]string{"wait"}, strings.Split(flags, " ")...)

	if x.Out != nil {
		fmt.Fprintln(x.Out, "---")
		fmt.Fprintf(x.Out, "##%02d: %s %s\n", id+1, "InstrWait", args)
		return nil
	}

	if x.DryRun {
		return nil
	}

	x.log(strconv.Itoa(id), "", "wait", strings.Join(args, " "))

	var stdout string
	var err error
	end := time.Now().Add(10 * time.Minute) // TODO Make time configurable via flag or env?
	for exp := backoff.NewExponential(10 * time.Second); !time.Now().After(end); exp.Sleep() {
		stdout, _, err = x.Kubectl.Run(nil, "", args...)
		if strings.Contains(stdout, "condition met") {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("##%d tpl %s: %w", id, "name", err) //TODO should be more imformative then 'name'
	}

	return nil
}

// Apply applies the resource in b to the target cluster.
func (x *Execute) Apply(id int, name string, b []byte) error {
	docs, err := yamlx.SplitDoc(b)
	if err != nil {
		return err
	}

	for i, doc := range docs {
		if yamlx.IsEmpty(doc) {
			continue
		}

		id := fmt.Sprintf("%02d.%02d", id+1, i+1)

		args := []string{"apply", "-f", "-"}
		if x.DryRun {
			args = append(args, "--dry-run")
		}

		if x.Out != nil {
			fmt.Fprintln(x.Out, "---")
			fmt.Fprintf(x.Out, "##%s: %s %s %s\n", id, "InstrApply", args, name)
			fmt.Fprintln(x.Out, string(doc))

			continue // generate or apply
		}

		stdout, _, err := x.Kubectl.Run(nil, string(doc), args...)
		if err != nil {
			return fmt.Errorf("##%s tpl %s: %w", id, name, err)
		}

		x.log(id, name, "apply", stdout)
	}
	return nil
}

// Action performs an action on the target cluster.
func (x *Execute) Action(id int, name string, doc []byte, portForward string, passedValues *yamlx.Values) error {
	if x.Out != nil {
		var pf string
		if portForward != "" {
			pf = "portForward " + portForward
		}
		//TODO unify generated output
		fmt.Fprintln(x.Out, "---")
		fmt.Fprintf(x.Out, "##%02d: %s [%s] %s\n", id+1, "InstrAction", pf, name)
		scanner := bufio.NewScanner(bytes.NewReader(doc))
		for scanner.Scan() {
			fmt.Fprintln(x.Out, "#", scanner.Text())
		}

		return nil // don't apply
	}

	// dispatch action

	ac := &struct {
		Type string `yaml:"type"`
	}{}

	err := yaml2.Unmarshal(doc, ac)
	if err != nil {
		return err
	}

	x.log(strconv.Itoa(id), name, "action", ac.Type)

	switch ac.Type {
	case "getSecret":
		return x.getSecret(id, name, doc, portForward, passedValues)
	case "setVault":
		return x.setVault(id, name, doc, portForward, passedValues)
	default:
		return fmt.Errorf("unknown action type: %s", ac.Type)
	}
}

// Log a line for step name.
func (x *Execute) log(id, tpl, step, txt string) {
	x.Log.Info(step,
		"id", id,
		"txt", strings.TrimSuffix(txt, "\n"),
		"tpl", tpl)
}

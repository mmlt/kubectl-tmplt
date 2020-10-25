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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"strings"
	"time"
)

// Execute executes Action, Apply, Wait steps.
type Execute struct {
	// DryRun prevents making changes to the target cluster.
	DryRun bool

	// NoDelete prevents prune from deleting resources.
	NoDelete bool

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
		fmt.Fprintf(x.Out, "##%02d: %s %s\n", id, "InstrWait", args)
		return nil
	}

	if x.DryRun {
		return nil
	}

	x.log("wait", id, 0, "", strings.Join(args, " "))

	var stdout string
	var err error
	end := time.Now().Add(10 * time.Minute) // TODO Make time-out time configurable via flag or env?
	for exp := backoff.NewExponential(10 * time.Second); !time.Now().After(end); exp.Sleep() {
		stdout, _, err = x.Kubectl.Run(nil, "", args...)
		if strings.Contains(stdout, "condition met") {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("##%d tpl %s: %w", id, "name", err)
	}

	return nil
}

// Apply applies the yaml's in b to the target cluster.
func (x *Execute) Apply(id int, name string, labels map[string]string, b []byte) ([]KindNamespaceName, error) {
	docs, err := yamlx.SplitDoc(b)
	if err != nil {
		return nil, err
	}

	var resources []KindNamespaceName

	for i, doc := range docs {
		if yamlx.IsEmpty(doc) {
			continue
		}

		id2 := fmt.Sprintf("%02d.%02d", id, i+1)

		if len(labels) > 0 {
			// When labels are defined the doc must be a Kubernetes resource.
			d, knsn, err := updateObjectYaml(doc, labels)
			if err != nil {
				return nil, fmt.Errorf("##%s tpl %s: %w", id2, name, err)
			}
			resources = append(resources, knsn)
			doc = d
		}

		args := []string{"apply", "-f", "-"}
		if x.DryRun {
			args = append(args, "--dry-run")
		}

		if x.Out != nil {
			fmt.Fprintln(x.Out, "---")
			fmt.Fprintf(x.Out, "##%s: %s %s %s\n", id2, "InstrApply", args, name)
			fmt.Fprintln(x.Out, string(doc))

			continue // generate or apply
		}

		stdout, _, err := x.Kubectl.Run(nil, string(doc), args...)
		if err != nil {
			return nil, fmt.Errorf("##%s tpl %s: %w", id2, name, err)
		}

		x.log("apply", id, i+1, name, stdout)
	}

	return resources, nil
}

func (x *Execute) Prune(id int, deployed []KindNamespaceName, store Store) error {
	idmin := 0

	//TODO move to validation function (or separate validation tool?)
	for _, k := range duplicates(deployed) {
		idmin++
		// same group/kind namespace/name used multiple times
		x.log("prune WARNING; multiple deployments", id, idmin, "", k.String())
	}
	apiResources, err := x.getK8sAPIResources()
	if err != nil {
		return err
	}
	invalid := invalidNamespace(deployed, apiResources)
	if len(invalid) > 0 {
		b := asCSV(invalid)
		// namespace set on non-namespaced resource or namespace is missing (empty) on namespaced resource.
		return fmt.Errorf("namespace set when it's not needed or vise versa:\n%s", b.String())
	}

	// Read configmap with previously deployed resources.
	idmin++
	x.log("prune", id, idmin, "", "read store")
	cluster, err := x.readStore(store)
	if err != nil {
		if strings.Contains(err.Error(), "Error from server (NotFound):") {
			idmin++
			x.log("prune", id, idmin, "", "skipped: no store found")
		} else {
			return fmt.Errorf("prune read store: %w", err)
		}
	}

	// Diff what is in cluster but not in deployed.
	toDelete := subtract(cluster, deployed)

	// Delete objects in reverse order of creation.
	reverse(toDelete)
	// NB. an alternative is to use sortInDeleteOrder(toDelete)

	if x.Log.V(5).Enabled() {
		mustWriteAPIResourcesCSV(apiResources, "_apiresouces.txt")
		mustWriteCSV(deployed, "_deployed.txt")
		mustWriteCSV(cluster, "_cluster.txt")
		mustWriteCSV(toDelete, "_delete.txt")
	}

	// Delete
	for _, r := range toDelete {
		rn, err := resource(r.GVK, apiResources)
		if err != nil {
			return err
		}
		args := []string{"delete", rn, r.Name}
		if r.Namespace != "" {
			args = append(args, "-n", r.Namespace)
		}
		idmin++
		if x.NoDelete || x.DryRun {
			x.log("prune", id, idmin, "", "skipped "+strings.Join(args, " "))
			continue
		}
		x.log("prune", id, idmin, "", strings.Join(args, " "))
		_, _, err = x.Kubectl.Run(nil, "", args...)
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
	}

	// Write deployed to configmap
	err = x.writeStore(store, deployed)
	if err != nil {
		return err
	}

	idmin++
	x.log("prune", id, idmin, "", "done")
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
		fmt.Fprintf(x.Out, "##%02d: %s [%s] %s\n", id, "InstrAction", pf, name)
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

	x.log("action", id, 0, name, ac.Type)

	switch ac.Type {
	case "getSecret":
		return x.getSecret(id, name, doc, portForward, passedValues)
	case "setVault":
		return x.setVault(id, name, doc, portForward, passedValues)
	default:
		return fmt.Errorf("unknown action type: %s", ac.Type)
	}
}

// Log a line.
func (x *Execute) log(msg string, id, idmin int, tpl string, txt string) {
	var s string
	if id > 0 {
		s = fmt.Sprintf("%02d", id)
		if idmin > 0 {
			s = fmt.Sprintf("%s.%02d", s, idmin)
		}
	}
	x.Log.Info(msg,
		"id", s,
		"txt", strings.TrimSuffix(txt, "\n"),
		"tpl", tpl)
}

// UpdateObjectYaml adds labels to a k8s object and returns the updated yaml and its kind, namespace, name.
func updateObjectYaml(doc []byte, labels map[string]string) ([]byte, KindNamespaceName, error) {
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err := dec.Decode(doc, nil, obj)
	if err != nil {
		return nil, KindNamespaceName{}, err
	}

	l := obj.GetLabels()
	for k, v := range labels {
		if l == nil {
			l = map[string]string{}
		}
		l[k] = v
	}
	obj.SetLabels(l)

	b, err := yaml2.Marshal(obj.Object)
	if err != nil {
		return nil, KindNamespaceName{}, err
	}

	return b, NewKindNamespaceName(obj), nil
}

// GetK8sAPIResources returns all APIResources registered in the cluster.
func (x *Execute) getK8sAPIResources() ([]metav1.APIResource, error) {
	args := []string{"api-resources"}
	stdout, _, err := x.Kubectl.Run(nil, "", args...)
	if err != nil {
		return nil, fmt.Errorf("get api-resources: %w", err)
	}

	// process output
	// NB. kubectl doesn't support api-resources json output yet.
	return textToAPIResources(stdout)
}

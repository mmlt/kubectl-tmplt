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
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"sort"
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

// Prune diff deployed with the resources in namespaces selected by labels.
// The resources that are in cluster but not in deployed are deleted.
// NB. an empty string in namespaces selects non-namespaced resources.
func (x *Execute) Prune(id int, deployed []KindNamespaceName, labels map[string]string, namespaces []string) error {
	x.log("prune", id, 0, "", "query cluster")

	apiResources, err := x.getK8sAPIResources()
	if err != nil {
		return err
	}

	// Sanity checks.
	for _, k := range duplicates(deployed) {
		x.log("prune WARNING; multiple deployments", id, 0, "", k.String())
	}

	invalid := invalidNamespace(deployed, apiResources)
	if len(invalid) > 0 {
		b := asCSV(invalid)
		return fmt.Errorf("can't prune; namespace set when it's not needed or vise versa:\n%s", b.String())
	}

	// Get existing objects matching namespaces and labels.
	// An empty string in namespaces also includes non-namespaced resources in the result.
	cluster, err := x.getSelectedObjects(namespaces, labels, apiResources)
	if err != nil {
		return err
	}

	// Diff what is in cluster but not in apply.
	// Ignore Version since for example deploying a v1beta1 might result in metav1.
	toDelete := subtract(cluster, deployed)

	// For namespaced resources we're only interested in the ones which namespace is in namespaces.
	toDelete = keepNamespaced(toDelete, namespaces)

	// Namespace resources are considered non-namespaced resources.
	// But we don't want to delete a namespace that is not in namespaces.
	toDelete = keepNamespaces(toDelete, namespaces)

	// Sort so namespaced resources are before non-namespaced resources.
	sort.Slice(toDelete, func(i, j int) bool {
		//TODO (Mutating)WebhookConfiguration first
		// https://github.com/helm/helm/blob/release-2.16/pkg/tiller/kind_sorter.go
		return toDelete[i].Namespace > toDelete[j].Namespace
	})

	if x.Log.V(5).Enabled() {
		mustWriteAPIResourcesCSV(apiResources, "_apiresouces.txt")
		mustWriteCSV(deployed, "_deployed.txt")
		mustWriteCSV(cluster, "_cluster.txt")
		mustWriteCSV(toDelete, "_delete.txt")
	}

	if !(x.NoDelete || x.DryRun) {
		// Delete
		for i, r := range toDelete {
			rn, err := resource(r.GVK, apiResources)
			if err != nil {
				return err
			}
			args := []string{"delete", rn, r.Name}
			if r.Namespace != "" {
				args = append(args, "-n", r.Namespace)
			}
			x.log("prune", id, i+1, "", strings.Join(args, " "))
			_, _, err = x.Kubectl.Run(nil, "", args...)
			if err != nil {
				return fmt.Errorf("delete: %w", err)
			}
		}
	}

	x.log("prune", id, 0, "", "prune completed")
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

// GetSelectedObjects returns all resources in the specified namespaces that match labels.
// An empty string in namespaces selects non-namespaced resources.
func (x *Execute) getSelectedObjects(namespaces []string, labels map[string]string, apiResources []metav1.APIResource) ([]KindNamespaceName, error) {
	apiResources, err := filterAPIResources(apiResources)
	if err != nil {
		return nil, err
	}

	var resources []KindNamespaceName
	for _, ns := range namespaces {
		namespaced := ns != ""
		for _, ar := range apiResources {
			if ar.Namespaced == namespaced {
				rs, err := x.getObjects(fullAPIResourceName(ar), ns, labels)
				if err != nil {
					return nil, err
				}
				resources = append(resources, rs...)
			}
		}
	}
	return resources, nil
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

// GetObjects returns all instances of kind in namespace that match labels.
func (x *Execute) getObjects(kind, namespace string, labels map[string]string) ([]KindNamespaceName, error) {
	args := []string{"get", kind, "-l", joinLabels(labels), "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	stdout, _, err := x.Kubectl.Run(nil, "", args...)
	if err != nil {
		return nil, fmt.Errorf("get resources: %w", err)
	}

	// process output
	obj := &unstructured.Unstructured{}
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode([]byte(stdout), nil, obj)
	if err != nil {
		return nil, fmt.Errorf("get api-resources: %w", err)
	}

	var r []KindNamespaceName
	err = obj.EachListItem(func(o runtime.Object) error {
		ob, ok := o.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unknown runtime.Object: %v", o)
		}
		r = append(r, NewKindNamespaceName(ob))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get api-resources: %w", err)
	}

	return r, nil
}

// JoinLabels returns a comma separated string with k=v pairs.
func joinLabels(labels map[string]string) string {
	var ss []string
	for k, v := range labels {
		ss = append(ss, k+"="+v)
	}
	return strings.Join(ss, ",")
}

/*** KindNamespaceName TODO move to own file ***/

func NewKindNamespaceName(obj *unstructured.Unstructured) KindNamespaceName {
	gvk := obj.GroupVersionKind()
	return KindNamespaceName{
		GVK: metav1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// KindNamespaceName
// TODO Use core/v1/ObjectReference and remove this type.
// https://www.k8sref.io/docs/common-definitions/objectreference-/
type KindNamespaceName struct {
	GVK       metav1.GroupVersionKind
	Namespace string
	Name      string
}

// String makes the receiver implement Stringer.
func (k KindNamespaceName) String() string {
	return fmt.Sprintf(`%v, %v, %v, %v, %v`,
		k.GVK.Group, k.GVK.Version, k.GVK.Kind, k.Namespace, k.Name)
}
func kindNamespaceNameHeader() string {
	return "Group, Version, Kind, Namespace, Name"
}

// Subtract returns a - b
// Ignore GVK Version when comparing.
func subtract(a, b []KindNamespaceName) []KindNamespaceName {
	idx := make(map[KindNamespaceName]bool, len(b))
	for _, x := range b {
		x.GVK.Version = ""
		idx[x] = true
	}

	var r []KindNamespaceName
	for _, x := range a {
		x.GVK.Version = ""
		if _, inB := idx[x]; inB {
			continue
		}
		r = append(r, x)
	}

	return r
}

func duplicates(list []KindNamespaceName) []KindNamespaceName {
	idx := make(map[KindNamespaceName]int, len(list))
	for _, x := range list {
		x.GVK.Version = ""
		idx[x]++
	}

	var r []KindNamespaceName
	for k, v := range idx {
		if v <= 1 {
			continue
		}
		r = append(r, k)
	}

	return r
}

func asCSV(list []KindNamespaceName) bytes.Buffer {
	var b bytes.Buffer
	b.WriteString(kindNamespaceNameHeader())
	b.WriteString("\n")
	for _, x := range list {
		b.WriteString(x.String())
		b.WriteString("\n")
	}
	return b
}

func mustWriteCSV(list []KindNamespaceName, filename string) {
	b := asCSV(list)
	err := ioutil.WriteFile(filename, b.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}

// KeepNamespaced removes namespaced resources that are not in namespaces.
func keepNamespaced(list []KindNamespaceName, namespaces []string) []KindNamespaceName {
	idx := make(map[string]bool, len(namespaces))
	for _, x := range namespaces {
		idx[x] = true
	}

	var r []KindNamespaceName
	for _, x := range list {
		if _, included := idx[x.Namespace]; !included {
			continue
		}
		r = append(r, x)
	}

	return r
}

// KeepNamespaces removes Namespace resources (which are non-namespaced) that are not in namespaces.
func keepNamespaces(list []KindNamespaceName, namespaces []string) []KindNamespaceName {
	idx := make(map[string]bool, len(namespaces))
	for _, x := range namespaces {
		idx[x] = true
	}

	var r []KindNamespaceName
	for _, x := range list {
		if _, included := idx[x.Name]; x.GVK.Kind == "Namespace" && !included {
			continue
		}
		r = append(r, x)
	}

	return r
}

// InvalidNamespace returns the members of list that either have;
// 1) a namespace set while it's a non-namespaced resource
// 2) doesn't have a namespace set while it's a namespaced resource (we don't want
// to rely on the namespace defaulting to what's in kubeconfig)
func invalidNamespace(list []KindNamespaceName, resources []metav1.APIResource) []KindNamespaceName {
	idx := make(map[metav1.GroupVersionKind]bool, len(resources))
	for _, x := range resources {
		idx[metav1.GroupVersionKind{Group: x.Group, Kind: x.Kind}] = x.Namespaced
	}

	var r []KindNamespaceName
	for _, x := range list {
		i := x.GVK
		i.Version = ""
		namespaced := idx[i]
		hasNamespace := x.Namespace != ""
		if !(namespaced == hasNamespace) {
			r = append(r, x)
		}
	}
	return r
}

package execute

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/mmlt/kubectl-tmplt/pkg/util/backoff"
	"github.com/mmlt/kubectl-tmplt/pkg/util/texttable"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
	"io"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		x.log("apply", id, 0, name, stdout)
	}

	return resources, nil
}

// Prune diff deployed with the resources in namespaces selected by labels.
// The resources that are in cluster but not in deployed are deleted.
// NB. an empty string in namespaces selects non-namespaced resources.
func (x *Execute) Prune(id int, deployed []KindNamespaceName, labels map[string]string, namespaces []string) error {
	x.log("prune", id, 0, "", "query cluster")

	// Get existing objects matching namespaces and labels.
	cluster, err := x.getSelectedObjects(namespaces, labels)
	if err != nil {
		return err
	}

	// diff what is in cluster but not in apply.
	toDelete := subtract(cluster, deployed)

	// TODO remove namespace that are not in namespaces
	// prevent deleting a namespace resources that match labels but are not in namespaces.
	// (namespace resources themselves are non-namespaced and therefore included when prune.namespaces contains "")
	toDelete = keepNamespaces(toDelete, namespaces)

	// sort so namespaced resources are before non-namespaced resources.
	sort.Slice(toDelete, func(i, j int) bool {
		return toDelete[i].Namespace > toDelete[j].Namespace
	})

	// delete
	for i, r := range toDelete {
		args := []string{"delete", r.Resource(), r.Name}
		if r.Namespace != "" {
			args = append(args, "-n", r.Namespace)
		}
		x.log("prune", id, i+1, "", strings.Join(args, " "))
		_, _, err := x.Kubectl.Run(nil, "", args...)
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
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
	s := fmt.Sprintf("%02d", id)
	if idmin > 0 {
		s = fmt.Sprintf("%s.%02d", s, idmin)
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

// GetSelectedObjects returns all resources in the specified namespaces matching labels.
func (x *Execute) getSelectedObjects(namespaces []string, labels map[string]string) ([]KindNamespaceName, error) {
	apiResources, err := x.getK8sAPIResources()
	if err != nil {
		return nil, err
	}

	apiResources, err = filterAPIResources(apiResources)
	if err != nil {
		return nil, err
	}

	var resources []KindNamespaceName
	for _, ns := range namespaces {
		namespaced := ns != ""
		for _, ar := range apiResources {
			if ar.Namespaced == namespaced {
				rs, err := x.getObjects(ar.Kind, ns, labels)
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
func (x *Execute) getK8sAPIResources() ([]v1.APIResource, error) {
	args := []string{"api-resources"}
	stdout, _, err := x.Kubectl.Run(nil, "", args...)
	if err != nil {
		return nil, fmt.Errorf("get api-resources: %w", err)
	}

	// process output
	// NB. kubectl doesn't support api-resources json output yet.
	return textToAPIResources(stdout)
}

// GetObjects returns all instances of kind TODO(shouldn't this be GKV?) in namespace matching labels.
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

// DeleteObjects deletes all resources in list.
func (x *Execute) deleteObjects(list []KindNamespaceName) error {
	for _, r := range list {
		args := []string{"delete", r.Resource(), r.Name}
		if r.Namespace != "" {
			args = append(args, "-n", r.Namespace)
		}
		_, _, err := x.Kubectl.Run(nil, "", args...)
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
	}
	return nil
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
		GVK: v1.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// KindNamespaceName TODO call it K8sResourceRef? K8sObjectRef
type KindNamespaceName struct {
	GVK       v1.GroupVersionKind
	Namespace string
	Name      string
}

// String makes the receiver implement Stringer.
func (k KindNamespaceName) String() string {
	return fmt.Sprintf("%s %s/%s", k.GVK.String(), k.Namespace, k.Name)
}

// Resource returns the name of the resource like; pod, configmap, xyz.constraints.gatekeeper.sh
func (k KindNamespaceName) Resource() string {
	r := strings.ToLower(k.GVK.Kind)
	if len(k.GVK.Group) > 0 {
		r += "." + k.GVK.Group
	}
	return r
}

// Subtract returns a - b
func subtract(a, b []KindNamespaceName) []KindNamespaceName {
	idx := make(map[KindNamespaceName]bool, len(b))
	for _, x := range b {
		idx[x] = true
	}

	var r []KindNamespaceName
	for _, x := range a {
		if _, inB := idx[x]; inB {
			continue
		}
		r = append(r, x)
	}

	return r
}

// KeepNamespaces removes any Namespace resource that's not in namespaces.
func keepNamespaces(list []KindNamespaceName, namespaces []string) []KindNamespaceName {
	idx := make(map[string]bool, len(namespaces))
	for _, x := range namespaces {
		idx[x] = true
	}

	var r []KindNamespaceName
	for _, x := range list {
		if _, included := idx[x.Name]; x.Namespace == "" && !included {
			continue
		}
		r = append(r, x)
	}

	return r
}

/*** APIResource helpers ***/

func filterAPIResources(list []v1.APIResource) ([]v1.APIResource, error) {
	const remove = `NAME                              SHORTNAMES   APIGROUP                       NAMESPACED   KIND
apiservices                                    apiregistration.k8s.io         false        APIService
bindings                                                                      true         Binding
certificatesigningrequests        csr          certificates.k8s.io            false        CertificateSigningRequest
componentstatuses                 cs                                          false        ComponentStatus
controllerrevisions                            apps                           true         ControllerRevision
csinodes                                       storage.k8s.io                 false        CSINode
endpoints                         ep                                          true         Endpoints
events                            ev                                          true         Event
limitranges                       limits                                      true         LimitRange
localsubjectaccessreviews                      authorization.k8s.io           true         LocalSubjectAccessReview
nodes                             no                                          false        Node
podtemplates                                                                  true         PodTemplate
runtimeclasses                                 node.k8s.io                    false        RuntimeClass
selfsubjectaccessreviews                       authorization.k8s.io           false        SelfSubjectAccessReview
selfsubjectrulesreviews                        authorization.k8s.io           false        SelfSubjectRulesReview
storageclasses                    sc           storage.k8s.io                 false        StorageClass
subjectaccessreviews                           authorization.k8s.io           false        SubjectAccessReview
tokenreviews                                   authentication.k8s.io          false        TokenReview
volumeattachments                              storage.k8s.io                 false        VolumeAttachment
`
	rlist, err := textToAPIResources(remove)
	if err != nil {
		return nil, err
	}
	var result []v1.APIResource
	for _, ar := range list {
		ok := true
		for _, rar := range rlist {
			if ar.Name == rar.Name {
				ok = false
				break
			}
		}
		if ok {
			result = append(result, ar)
		}
	}
	return result, nil
}

func textToAPIResources(txt string) ([]v1.APIResource, error) {
	t := texttable.Read(strings.NewReader(txt), 3)
	iter := t.RowIter()
	var result []v1.APIResource
	for iter.Next() {
		ar := v1.APIResource{}
		//TODO handle ok from GetColByName
		s, _ := iter.GetColByName("NAME")
		ar.Name = s
		s, _ = iter.GetColByName("APIGROUP")
		//if s == "" {
		//	// use the group of the containing resource (APIResourceList)
		//	s = "v1"
		//}
		ar.Group = s
		s, _ = iter.GetColByName("NAMESPACED")
		ar.Namespaced = (s == "true")
		s, _ = iter.GetColByName("KIND")
		ar.Kind = s
		result = append(result, ar)
	}
	return result, nil
}

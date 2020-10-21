package execute

import (
	"bytes"
	"fmt"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// KindNamespaceName
// TODO Use core/v1/ObjectReference and remove this type.
// https://www.k8sref.io/docs/common-definitions/objectreference-/
type KindNamespaceName struct {
	GVK       metav1.GroupVersionKind
	Namespace string
	Name      string
}

// NewKindNamespaceName returns a KindNamespaceName from any kubernetes object.
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

// Duplicates returns list items that use the same GroupKind/namespace/name (IOW overwrite each other).
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

// MustWriteCSV writes a CSV file with list for debugging purposes.
func mustWriteCSV(list []KindNamespaceName, filename string) {
	b := asCSV(list)
	err := ioutil.WriteFile(filename, b.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}

// AsCSV turns list into a CSV formatted text.
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

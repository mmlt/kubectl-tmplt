package execute

import (
	"bytes"
	"fmt"
	"github.com/mmlt/kubectl-tmplt/pkg/util/texttable"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// APIResource helpers

// FilterAPIResources removes resources that are not deployable.
func filterAPIResources(list []metav1.APIResource) ([]metav1.APIResource, error) {
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
subjectaccessreviews                           authorization.k8s.io           false        SubjectAccessReview
tokenreviews                                   authentication.k8s.io          false        TokenReview
volumeattachments                              storage.k8s.io                 false        VolumeAttachment
`
	rlist, err := textToAPIResources(remove)
	if err != nil {
		return nil, err
	}
	var result []metav1.APIResource
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

// TextToAPIResources turns text into APIResource objects.
func textToAPIResources(txt string) ([]metav1.APIResource, error) {
	t := texttable.Read(strings.NewReader(txt), 3)
	iter := t.RowIter()
	var result []metav1.APIResource
	for iter.Next() {
		var ok bool
		ar := metav1.APIResource{}
		//TODO handle ok from GetColByName
		s, _ := iter.GetColByName("NAME")
		ar.Name = s
		s, _ = iter.GetColByName("NAMESPACED")
		ar.Namespaced = (s == "true")
		s, _ = iter.GetColByName("KIND")
		ar.Kind = s

		// APIVERSION contains "v1" or "apps/v1", APIGROUP contains "apps"
		if s, ok = iter.GetColByName("APIVERSION"); ok {
			gv := strings.Split(s, "/")
			if len(gv) == 1 {
				ar.Version = s
			} else {
				ar.Group = gv[0]
				ar.Version = gv[1]
			}
		} else if s, ok = iter.GetColByName("APIGROUP"); ok {
			ar.Group = s
		}

		result = append(result, ar)
	}
	return result, nil
}

// MustWriteAPIResourcesCSV writes a CSV file with list for debugging purposes.
func mustWriteAPIResourcesCSV(list []metav1.APIResource, filename string) {
	b := asAPIResourceCSV(list)
	err := ioutil.WriteFile(filename, b.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}

// AsAPIResourceCSV turns list into a CSV formatted text.
func asAPIResourceCSV(list []metav1.APIResource) bytes.Buffer {
	var b bytes.Buffer
	b.WriteString("Group,Version,Kind,Name,Namespaced\n")
	for _, x := range list {
		b.WriteString(fmt.Sprintf("%v,%v,%v,%v,%v", x.Group, x.Version, x.Kind, x.Name, x.Namespaced))
		b.WriteString("\n")
	}
	return b
}

// Resource returns the name of the resource like; pod, configmap, xyz.constraints.gatekeeper.sh
func resource(gvk metav1.GroupVersionKind, resources []metav1.APIResource) (string, error) {
	for _, r := range resources {
		if r.Kind == gvk.Kind && r.Group == gvk.Group {
			return fullAPIResourceName(r), nil
		}
	}
	return "", fmt.Errorf("no api-resource for %s", gvk.String())
}

// FullAPIResourceName returns name.group or name when group is empty.
func fullAPIResourceName(apiResource metav1.APIResource) string {
	return strings.Join([]string{apiResource.Name, apiResource.Group}, ".")
}

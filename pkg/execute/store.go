package execute

import (
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Store config.
type Store struct {
	// Namespace were data is stored.
	Namespace string
	// Name of ConfigMap were data is stored.
	Name string
	// Xtra key/values to store.
	X map[string]string
}

// ReadStore reads deployed objects from store.
func (x *Execute) readStore(store Store) ([]KindNamespaceName, error) {
	args := []string{"-n", store.Namespace, "get", "configmap", store.Name, "-o", "json"}
	stdout, _, err := x.Kubectl.Run(nil, "", args...)
	if err != nil {
		return nil, fmt.Errorf("get configmap: %w", err)
	}

	// process output
	cm := &corev1.ConfigMap{}
	err = json.Unmarshal([]byte(stdout), cm)
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s: %w", store.Namespace, store.Name, err)
	}

	s, ok := cm.Data["deployed"]
	if !ok {
		return nil, fmt.Errorf("get configmap %s/%s: no field 'deployed'", store.Namespace, store.Name)
	}

	r := &[]KindNamespaceName{}
	err = json.Unmarshal([]byte(s), r)
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s 'deployed' field: %w", store.Namespace, store.Name, err)
	}

	return *r, nil
}

// WriteStore writes deployed objects to store.
func (x *Execute) writeStore(store Store, deployed []KindNamespaceName) error {
	b, err := json.Marshal(deployed)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      store.Name,
			Namespace: store.Namespace,
		},
		Data: map[string]string{
			"deployed": string(b),
		},
	}

	// add store X kv's
	for k, v := range store.X {
		if _, ok := cm.Data[k]; ok {
			// don't overwrite entries
			continue
		}
		cm.Data[k] = v
	}

	d, err := json.Marshal(cm)
	if err != nil {
		return err
	}

	args := []string{"apply", "-f", "-"}
	if x.DryRun {
		args = append(args, "--dry-run")
	}
	_, _, err = x.Kubectl.Run(nil, string(d), args...)
	if err != nil {
		return err
	}

	return nil
}

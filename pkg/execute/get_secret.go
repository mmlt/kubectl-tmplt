package execute

import (
	"encoding/json"
	"fmt"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
)

// GetSecret is an Action to read a Kubernetes Secret from the target cluster.
func (x *Execute) getSecret(id int, name string, doc []byte, portForward string, passedValues *yamlx.Values) error {
	// get action arguments
	ac := &actionSecret{}
	err := yaml2.Unmarshal(doc, ac)
	if err != nil {
		return fmt.Errorf("getSecret: %w", err)
	}

	// run action
	args := []string{"-n", ac.Namespace, "get", "secret", ac.Name, "-o", "json"}

	stdout, _, err := x.Kubectl.Run(nil, "", args...)
	if err != nil {
		return fmt.Errorf("get secret: %w", err)
	}

	// process output
	secret := &yamlx.Values{}
	err = json.Unmarshal([]byte(stdout), secret)
	if err != nil {
		return fmt.Errorf("get secret %s/%s response: %w", ac.Namespace, ac.Name, err)
	}

	if _, ok := (*secret)["data"]; !ok {
		return fmt.Errorf("get secret %s/%s: no data", ac.Namespace, ac.Name)
	}

	(*passedValues)["secret"] = map[string]interface{}{
		ac.Namespace: map[string]interface{}{
			ac.Name: map[string]interface{}{
				"data": (*secret)["data"],
			},
		},
	}

	return nil
}

// ActionSecret contains the parameters for a getSecret action.
type actionSecret struct {
	Type      string `yaml:"type"`
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`
}

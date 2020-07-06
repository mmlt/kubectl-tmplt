package execute

import (
	"encoding/json"
	"fmt"
	"github.com/mmlt/kubectl-tmplt/pkg/util/backoff"
	"github.com/mmlt/kubectl-tmplt/pkg/util/texpr"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
	"time"
)

// GetSecret is an Action to read a Kubernetes Secret from the target cluster.
func (x *Execute) getSecret(id int, name string, doc []byte, portForward string, passedValues *yamlx.Values) error {
	// get action arguments
	ac := &actionSecret{}
	err := yaml2.Unmarshal(doc, ac)
	if err != nil {
		return fmt.Errorf("getSecret: %w", err)
	}

	pc, err := texpr.Parse(ac.PostCondition, "true")
	if err != nil {
		return fmt.Errorf("parse postCondition: %w", err)
	}

	var obj *yamlx.Values
	var r string
	for exp := backoff.NewExponential(10 * time.Second); exp.Retries() < 10; exp.Sleep() {
		obj, err = x.getK8sSecret(ac.Namespace, ac.Name)
		if err != nil {
			continue
		}
		r, err = pc.Evaluate(obj)
		if err != nil {
			err = fmt.Errorf("evaluate postCondition: %w", err)
			continue
		}
		if r == "true" {
			break
		}
	}
	if err != nil {
		return err
	}
	if r != "true" {
		return fmt.Errorf("timeout waiting for postCondition: %s", ac.PostCondition)
	}

	(*passedValues)["secret"] = map[string]interface{}{
		ac.Namespace: map[string]interface{}{
			ac.Name: map[string]interface{}{
				"data": (*obj)["data"],
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
	// PostCondition is a text/template expression that must evaluate to 'true' for the action to be successful.
	PostCondition string `yaml:"postCondition"`
}

func (x *Execute) getK8sSecret(namespace, name string) (*yamlx.Values, error) {
	args := []string{"-n", namespace, "get", "secret", name, "-o", "json"}
	stdout, _, err := x.Kubectl.Run(nil, "", args...)
	if err != nil {
		return nil, fmt.Errorf("get secret: %w", err)
	}

	// process output
	secret := &yamlx.Values{}
	err = json.Unmarshal([]byte(stdout), secret)
	if err != nil {
		return nil, fmt.Errorf("get secret %s/%s response: %w", namespace, name, err)
	}

	return secret, nil
}

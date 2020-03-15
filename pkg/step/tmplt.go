package step

import "github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"

// Tmplt is a step that;
// 1) reads a yaml file containing one or more YAML docs
// 2) expands the file contents using defaults.
// 3) applies the result to a k8s cluster.
type Tmplt struct {
	// Tmplt is a relative filepath to the template file.
	Tmplt string `yaml:"tmplt"`
	// Values are the template scoped variables.
	Values yamlx.Values `yaml:"values"`
}

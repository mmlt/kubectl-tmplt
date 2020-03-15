// Package step performs steps defined in a yaml text.
package step

// Job YAML format.
// Steps can be 'tmplt' or 'wait'.
//
// 		steps:
//		- tmplt: "path/to/file"
//		  values:
//		    one: 1
// 		- wait: "some wait condition"
//
//		defaults:
//		  two: 2

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	yaml2 "gopkg.in/yaml.v2"
)

// Iterator parses a yaml text with "steps" and "defaults" fields.
// It returns an iterator object that visits "steps" field.
func Iterator(steps []byte) (*Itr, error) {
	x := &struct {
		Steps    []yamlx.Values `yaml:"steps"`
		Defaults yamlx.Values   `yaml:"defaults"`
	}{}

	err := yaml2.Unmarshal(steps, x)
	if err != nil {
		return nil, err
	}

	return &Itr{
		steps:    x.Steps,
		defaults: x.Defaults,
	}, nil
}

// Itr contains the iterator state.
type Itr struct {
	steps    []yamlx.Values
	index    int
	defaults yamlx.Values
}

// Next returns the next item from the yaml 'steps' array; either a Tmplt or a Wait.
// It returns nil when iteration is complete.
func (itr *Itr) Next() interface{} {
	if itr.index >= len(itr.steps) {
		return nil
	}

	// get next object
	obj := itr.steps[itr.index]
	itr.index++

	// decide type of object based on a field name.
	_, isTmplt := obj["tmplt"]
	_, isCondition := obj["wait"]

	// map object to struct.
	cfg := &mapstructure.DecoderConfig{TagName: "yaml"}
	switch {
	case isTmplt:
		cfg.Result = &Tmplt{}
	case isCondition:
		cfg.Result = &Wait{}
	default:
		return fmt.Errorf("expected 'step' or 'wait' object, got %#v", obj)
	}
	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return err
	}
	err = dec.Decode(obj)
	if err != nil {
		return err
	}

	return cfg.Result
}

// Defaults returns the default values as defined in the job.
func (itr *Itr) Defaults() yamlx.Values {
	return itr.defaults
}

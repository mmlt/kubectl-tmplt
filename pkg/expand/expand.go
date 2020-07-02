package expand

import (
	"bytes"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/mmlt/kubectl-tmplt/pkg/expand/files"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"path/filepath"
	"strings"
	"text/template"
)

// Run expands a template text with values and returns the resulting text.
// Path is used to support {{ .Files }}.
// See https://golang.org/pkg/text/template/
func Run(environ []string, path string, text []byte, values, passed yamlx.Values, customFn template.FuncMap) ([]byte, error) {
	env := OSEnvironment(environ)

	// get template functions
	functions := getDefaultFunctions()
	// override Sprig function to make sure a sanitized environment is used.
	functions["env"] = func(s string) string { return env[s] }
	functions["expandenv"] = func(s string) string { return "<expandenv is not supported>" }
	// add custom functions
	for n, f := range customFn {
		functions[n] = f
	}

	// params contains values and methods that are accessed via {{ .Values }}, {{ .Get }} and {{ .Files }}
	var params = struct {
		Values yamlx.Values
		Get    yamlx.Values
		Files  files.Dir
	}{
		Values: values,
		Get:    passed,
		Files:  files.Dir(filepath.Dir(path)),
	}

	return expand(path, text, functions, params)
}

// Expand expands a template text with functions and params and returns the resulting text.
// Missing keys result in an error.
func expand(path string, text []byte, functions template.FuncMap, params interface{}) ([]byte, error) {
	// Create template with functions and text.
	tmpl, err := template.New("input").Funcs(functions).Option("missingkey=invalid").Parse(string(text))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var out bytes.Buffer
	err = tmpl.Execute(&out, &params)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	return out.Bytes(), nil
}

// GetDefaultFunctions returns a map with functions that are commonly used in templates.
// It consists of Sprig (generic)functions and TOML,JSON,YAML conversion functions.
func getDefaultFunctions() template.FuncMap {
	answer := sprig.TxtFuncMap()

	// add extra functionality
	answer["toToml"] = files.ToToml
	answer["toYaml"] = files.ToYaml
	answer["fromYaml"] = files.FromYaml
	answer["toJson"] = files.ToJson
	answer["fromJson"] = files.FromJson
	answer["indexOrDefault"] = indexOrDefault

	// add functions that sprig doesn't implement cross-platform (that don't work on windows)
	answer["filebase"] = filepath.Base
	answer["filedir"] = filepath.Dir
	answer["fileclean"] = filepath.Clean
	answer["fileext"] = filepath.Ext

	return answer
}

// OSEnvironment converts a []string with "key=value" items to a map["key"]="value".
func OSEnvironment(environ []string) map[string]string {
	result := make(map[string]string)
	for _, s := range environ {
		sl := strings.SplitN(s, "=", 2)
		if len(sl) != 2 {
			continue
		}
		result[sl[0]] = sl[1]
	}
	return result
}

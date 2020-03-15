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

// Run expands a template text with values. Path is used to support {{ .Files }}.
// The result is returned.
// See https://golang.org/pkg/text/template/
func Run(environ []string, path string, text []byte, values yamlx.Values) ([]byte, error) {
	env := OSEnvironment(environ)

	// get template functions
	functions := getDefaultFunctions()
	// override Sprig function to make sure a sanitized environment is used.
	functions["env"] = func(s string) string { return env[s] }
	functions["expandenv"] = func(s string) string { return "<expandenv is not supported>" }

	//err = expandAll(bag, functions, cliValues, filepath.Dir(all), out)
	return expand(path, text, functions, values)
}

// Expand accepts a template text and provides values and functions when expanding it.
// The result is returned.
func expand(path string, text []byte, functions template.FuncMap, values yamlx.Values) ([]byte, error) {
	// Create template with functions and text.
	tmpl, err := template.New("input").Funcs(functions).Parse(string(text))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	// Expand...
	// params contains values and methods that are accessed via {{ .Values }} and {{ .Files }}
	var params = struct {
		Values yamlx.Values
		Files  files.Dir
	}{
		Values: values,
		Files:  files.Dir(filepath.Dir(path)),
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

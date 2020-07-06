package texpr

import (
	"bytes"
	"text/template"
)

// Parse returns a parsed expr.
// An empty expr evaluates to def.
// Expressions use https://golang.org/pkg/text/template/ syntax without the curlies.
func Parse(expr, def string) (*Expr, error) {
	t := def
	if expr != "" {
		t = "{{ " + expr + " }}"
	}

	tmplt, err := template.New("expr").Parse(t)
	if err != nil {
		return nil, err
	}

	return &Expr{t: tmplt}, nil
}

// Expr can be evaluated.
type Expr struct {
	t *template.Template
}

// Evaluate the receiver with data.
func (ex Expr) Evaluate(data interface{}) (string, error) {
	var b bytes.Buffer
	err := ex.t.Execute(&b, data)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

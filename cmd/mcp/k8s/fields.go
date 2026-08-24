// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
)

// FieldPath is a kubectl JSONPath expression used to select
// values from an exported Kubernetes object.
type FieldPath struct {
	// expr is the expression in its bare form without the surrounding
	// braces and the leading dot, used as the key of the selected values.
	expr string

	// prefix holds the leading field names of the expression,
	// used to determine which fields are referenced by the expression.
	// It is empty when the expression starts with a wildcard
	// or with a recursive descent.
	prefix []string

	// jsonPath is the parsed expression used to select the values.
	jsonPath *jsonpath.JSONPath
}

// FieldPaths is a list of kubectl JSONPath expressions
// used to project exported Kubernetes objects.
type FieldPaths []FieldPath

// ParseFieldPaths parses the given kubectl JSONPath expressions.
// The curly braces and the leading dot are optional, so the expressions
// spec.chart.spec.version, .status.conditions[*].type and
// {.status.conditions[?(@.type=="Ready")].message} are all valid
// and are normalised to their bare form e.g. status.conditions[*].type.
// Each field must contain a single expression, the templates
// with multiple expressions are rejected.
func ParseFieldPaths(fields []string) (FieldPaths, error) {
	paths := make(FieldPaths, 0, len(fields))
	for _, field := range fields {
		expr := bareFieldPath(field)
		if expr == "" {
			return nil, errors.New("field path must not be empty")
		}

		text := "{." + expr + "}"
		if strings.HasPrefix(expr, "[") || strings.HasPrefix(expr, "..") {
			text = "{" + expr + "}"
		}

		parser, err := jsonpath.Parse("fields", text)
		if err != nil {
			return nil, fmt.Errorf("invalid field path '%s': %w", strings.TrimSpace(field), err)
		}

		prefix, err := fieldPrefix(parser.Root)
		if err != nil {
			return nil, fmt.Errorf("invalid field path '%s': %w", strings.TrimSpace(field), err)
		}

		jp := jsonpath.New("fields").AllowMissingKeys(true)
		if err := jp.Parse(text); err != nil {
			return nil, fmt.Errorf("invalid field path '%s': %w", strings.TrimSpace(field), err)
		}

		paths = append(paths, FieldPath{
			expr:     expr,
			prefix:   prefix,
			jsonPath: jp,
		})
	}
	return paths, nil
}

// bareFieldPath trims the given kubectl JSONPath expression and strips
// its surrounding braces and its leading dot, while preserving the
// recursive descent operator.
func bareFieldPath(field string) string {
	expr := strings.TrimSpace(field)
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	if strings.HasPrefix(expr, ".") && !strings.HasPrefix(expr, "..") {
		expr = expr[1:]
	}
	return expr
}

// fieldPrefix returns the leading field names of a parsed expression,
// or nil when the expression starts with a wildcard or with a recursive
// descent. It returns an error when the parsed template
// contains multiple expressions.
func fieldPrefix(root *jsonpath.ListNode) ([]string, error) {
	if root == nil || len(root.Nodes) != 1 {
		return nil, errors.New("must contain a single expression")
	}
	list, ok := root.Nodes[0].(*jsonpath.ListNode)
	if !ok {
		return nil, errors.New("must contain a single expression")
	}

	var prefix []string
	for _, node := range list.Nodes {
		field, ok := node.(*jsonpath.FieldNode)
		if !ok {
			break
		}
		prefix = append(prefix, field.Value)
	}
	return prefix, nil
}

// Covers reports whether the given field is referenced by any of the field paths,
// which is the case when the leading field names of an expression and the given
// field are a prefix of each other. An empty set of field paths covers everything,
// as do the expressions whose leading field names cannot be determined.
func (f FieldPaths) Covers(field ...string) bool {
	if len(f) == 0 {
		return true
	}
	for _, path := range f {
		if len(path.prefix) == 0 {
			return true
		}
		matched := true
		for i := 0; i < min(len(path.prefix), len(field)); i++ {
			if path.prefix[i] != field[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// Project returns a new object containing the apiVersion, kind, metadata.name
// and metadata.namespace (when present) of the given object, along with the
// values selected by the field paths keyed by their expression. A single selected
// value is set as is, multiple values are set as a list and expressions that
// select nothing are omitted. The selected values are not copied and reference
// the given object. When no field paths are set, the object is returned unchanged.
func (f FieldPaths) Project(obj map[string]any) map[string]any {
	if len(f) == 0 {
		return obj
	}

	result := make(map[string]any)
	for _, path := range [][]string{
		{"apiVersion"},
		{"kind"},
		{"metadata", "name"},
		{"metadata", "namespace"},
	} {
		if value, found, err := unstructured.NestedFieldCopy(obj, path...); err == nil && found {
			_ = unstructured.SetNestedField(result, value, path...)
		}
	}

	for _, path := range f {
		results, err := path.jsonPath.FindResults(obj)
		if err != nil {
			continue
		}

		values := make([]any, 0)
		for _, group := range results {
			for _, value := range group {
				if value.IsValid() && value.CanInterface() {
					values = append(values, value.Interface())
				}
			}
		}

		switch len(values) {
		case 0:
			continue
		case 1:
			result[path.expr] = values[0]
		default:
			result[path.expr] = values
		}
	}
	return result
}

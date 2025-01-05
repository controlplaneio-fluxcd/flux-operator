// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	sprig "github.com/go-task/slim-sprig/v3"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// BuildResourceSet builds a list of Kubernetes resources
// from a list of JSON templates using the provided inputs.
func BuildResourceSet(templates []*apix.JSON, inputs []map[string]string) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured
	for i, tmpl := range templates {
		if len(inputs) == 0 {
			object, err := BuildResource(tmpl, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to build resource: %w", err)
			}

			objects = append(objects, object)
			continue
		}

		for _, input := range inputs {
			object, err := BuildResource(tmpl, input)
			if err != nil {
				return nil, fmt.Errorf("failed to build resources[%d]: %w", i, err)
			}

			// exclude object based on annotations
			if val := object.GetAnnotations()[fluxcdv1.ReconcileAnnotation]; val == fluxcdv1.DisabledValue {
				continue
			}

			// deduplicate objects
			found := false
			for _, obj := range objects {
				if obj.GetAPIVersion() == object.GetAPIVersion() &&
					obj.GetKind() == object.GetKind() &&
					obj.GetNamespace() == object.GetNamespace() &&
					obj.GetName() == object.GetName() {
					found = true
					break
				}
			}

			if !found {
				objects = append(objects, object)
			}
		}
	}

	return objects, nil
}

// BuildResource builds a Kubernetes resource from a JSON template using the provided inputs.
// Template functions are provided by the slim-sprig library https://go-task.github.io/slim-sprig/.
func BuildResource(tmpl *apix.JSON, inputs map[string]string) (*unstructured.Unstructured, error) {
	ymlTemplate, err := yaml.JSONToYAML(tmpl.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert template to YAML: %w", err)
	}

	var fnInputs = template.FuncMap{"inputs": func() map[string]string {
		values := make(map[string]string)
		for k, v := range inputs {
			values[k] = v
		}
		return values
	}}

	tp, err := template.New("res").
		Delims("<<", ">>").
		Funcs(sprig.HermeticTxtFuncMap()).
		Funcs(fnInputs).
		Parse(string(ymlTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	b := &strings.Builder{}
	err = tp.Execute(b, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	object, err := ssautil.ReadObject(bytes.NewReader([]byte(b.String())))
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return object, nil
}

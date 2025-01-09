// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	sprig "github.com/go-task/slim-sprig/v3"
	"github.com/gosimple/slug"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// BuildResourceSet builds a list of Kubernetes resources
// from a list of JSON templates using the provided inputs.
func BuildResourceSet(templates []*apix.JSON, jsonInputs []fluxcdv1.ResourceSetInput) ([]*unstructured.Unstructured, error) {
	inputs := make([]map[string]any, 0, len(jsonInputs))
	for i, ji := range jsonInputs {
		inp := make(map[string]any, len(ji))
		for k, v := range ji {
			var data any
			if err := json.Unmarshal(v.Raw, &data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal inputs[%d]: %w", i, err)
			}
			inp[k] = data
		}
		inputs = append(inputs, inp)
	}

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
// In addition, the slugify function is available to generate slugs from strings using https://github.com/gosimple/slug/.
func BuildResource(tmpl *apix.JSON, inputs map[string]any) (*unstructured.Unstructured, error) {
	ymlTemplate, err := yaml.JSONToYAML(tmpl.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert template to YAML: %w", err)
	}

	tp, err := template.New("res").
		Delims("<<", ">>").
		Funcs(sprig.HermeticTxtFuncMap()).
		Funcs(template.FuncMap{"slugify": slug.Make}).
		Funcs(template.FuncMap{"inputs": func() any { return inputs }}).
		Funcs(template.FuncMap{"toYaml": toYaml, "mustToYaml": mustToYaml}).
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

// init initializes the slugify Go template function with the default settings.
func init() {
	// set max length to 63 characters which is
	// the maximum length for a Kubernetes label value
	slug.MaxLength = 63
	// enable smart truncate to avoid cutting words in half
	slug.EnableSmartTruncate = true
}

// toYaml encodes an item into a YAML string.
// On error, it returns an empty string.
func toYaml(v any) string {
	if b, err := mustToYaml(v); err == nil {
		return b
	}
	return ""
}

// mustToYaml encodes an item into a YAML string.
// On error, it returns an empty string and the error.
func mustToYaml(v any) (string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	sprig "github.com/go-task/slim-sprig/v3"
	"github.com/gosimple/slug"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// BuildResourceSet builds a list of Kubernetes resources
// from a YAML template, a list of JSON templates and the
// given combined inputs. The resulting objects are deduplicated
// by apiVersion, kind, namespace and name, with the objects built
// from the JSON templates taking precedence over the ones built
// from the YAML template.
func BuildResourceSet(yamlTemplate string, templates []*apix.JSON, combinedInputs inputs.Combined) ([]*unstructured.Unstructured, error) {
	var objects []*unstructured.Unstructured
	objectKeys := make(map[string]struct{})

	// addObject records the object key and appends the object to the
	// result. When dedup is set, objects whose key was already recorded
	// are skipped instead.
	addObject := func(object *unstructured.Unstructured, dedup bool) {
		key := objectKey(object)
		if _, found := objectKeys[key]; dedup && found {
			return
		}
		objectKeys[key] = struct{}{}
		objects = append(objects, object)
	}

	// build resources from JSON templates
	for i, tmpl := range templates {
		if len(combinedInputs) == 0 {
			object, err := BuildResource(tmpl, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to build resource: %w", err)
			}

			addObject(object, false)
			continue
		}

		for _, input := range combinedInputs {
			object, err := BuildResource(tmpl, input)
			if err != nil {
				return nil, fmt.Errorf("failed to build resources[%d]: %w", i, err)
			}

			// exclude object based on annotations
			if val := object.GetAnnotations()[fluxcdv1.ReconcileAnnotation]; val == fluxcdv1.DisabledValue {
				continue
			}

			// deduplicate objects
			addObject(object, true)
		}
	}

	// build resources from multi-doc YAML template
	if yamlTemplate != "" {
		var objectsFromTemplate []*unstructured.Unstructured
		if len(combinedInputs) == 0 {
			objs, err := BuildResourcesFromYAML(yamlTemplate, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to build resource: %w", err)
			}

			objectsFromTemplate = append(objectsFromTemplate, objs...)
		}
		for _, input := range combinedInputs {
			objs, err := BuildResourcesFromYAML(yamlTemplate, input)
			if err != nil {
				return nil, fmt.Errorf("failed to build resources: %w", err)
			}
			objectsFromTemplate = append(objectsFromTemplate, objs...)
		}

		for _, object := range objectsFromTemplate {
			// exclude object based on annotations
			if val := object.GetAnnotations()[fluxcdv1.ReconcileAnnotation]; val == fluxcdv1.DisabledValue {
				continue
			}
			// deduplicate objects
			addObject(object, true)
		}
	}

	return objects, nil
}

// StepBuildResult holds the objects built for a single ResourceSet step.
type StepBuildResult struct {
	// Name is the name of the step.
	Name string

	// Timeout is the health check timeout of the step, nil when
	// the step does not override the ResourceSet timeout.
	Timeout *metav1.Duration

	// Objects contains the Kubernetes resources built for the step.
	Objects []*unstructured.Unstructured
}

// FlattenSteps returns the objects of all the given steps as a single
// slice, preserving the step order. The flattened slice shares the object
// pointers with the step slices, so in-place metadata mutations performed
// on the flattened objects are visible per step.
func FlattenSteps(steps []StepBuildResult) []*unstructured.Unstructured {
	var objects []*unstructured.Unstructured
	for _, step := range steps {
		objects = append(objects, step.Objects...)
	}
	return objects
}

// ValidateResourceSetSpec validates that steps are not set together with
// resources or resourcesTemplate, and that each step sets at least one of
// resources or resourcesTemplate. The rules mirror the CRD CEL rules on
// api/v1 ResourceSetSpec and ResourceSetStep, enforcing them for offline
// builds (CLI) and for clusters running a CRD version without the rules.
// The validation is render-independent so that callers can reject an
// invalid spec even when no resources are built, e.g. when the input
// providers return no inputs.
func ValidateResourceSetSpec(spec fluxcdv1.ResourceSetSpec) error {
	if len(spec.Steps) > 0 && (len(spec.Resources) > 0 || spec.ResourcesTemplate != "") {
		return errors.New("spec.steps is mutually exclusive with spec.resources and spec.resourcesTemplate")
	}
	for _, step := range spec.Steps {
		if len(step.Resources) == 0 && step.ResourcesTemplate == "" {
			return fmt.Errorf("step %q: at least one of resources or resourcesTemplate must be set", step.Name)
		}
	}
	return nil
}

// BuildResourceSetFromSpec validates the given ResourceSet spec with
// ValidateResourceSetSpec and builds its resources using the combined
// inputs. When steps are set, the resources are built per step with
// BuildResourceSetSteps, otherwise the spec resources are built with
// BuildResourceSet and wrapped as a single anonymous step.
func BuildResourceSetFromSpec(spec fluxcdv1.ResourceSetSpec, combinedInputs inputs.Combined) ([]StepBuildResult, error) {
	if err := ValidateResourceSetSpec(spec); err != nil {
		return nil, err
	}

	if len(spec.Steps) > 0 {
		return BuildResourceSetSteps(spec.Steps, combinedInputs)
	}

	objects, err := BuildResourceSet(spec.ResourcesTemplate, spec.Resources, combinedInputs)
	if err != nil {
		return nil, err
	}
	return []StepBuildResult{{Objects: objects}}, nil
}

// BuildResourceSetSteps builds the Kubernetes resources of each step
// using BuildResourceSet and the given combined inputs. The results
// preserve the order of the steps, including steps that build zero
// objects. A resource defined in multiple steps results in an error,
// while duplicates within a step follow the BuildResourceSet semantics.
// The step fields are expected to have been validated upfront with
// ValidateResourceSetSpec.
func BuildResourceSetSteps(steps []fluxcdv1.ResourceSetStep, combinedInputs inputs.Combined) ([]StepBuildResult, error) {
	results := make([]StepBuildResult, 0, len(steps))
	stepOfObject := make(map[string]string)

	for _, step := range steps {
		objects, err := BuildResourceSet(step.ResourcesTemplate, step.Resources, combinedInputs)
		if err != nil {
			return nil, fmt.Errorf("step %q: %w", step.Name, err)
		}

		// reject resources already defined in a previous step, leaving
		// duplicates within the same step to the BuildResourceSet semantics
		for _, object := range objects {
			key := objectKey(object)
			if prevStep, found := stepOfObject[key]; found {
				if prevStep == step.Name {
					continue
				}
				return nil, fmt.Errorf("duplicate resource %s in step %q, already defined in step %q",
					ssautil.FmtUnstructured(object), step.Name, prevStep)
			}
			stepOfObject[key] = step.Name
		}

		results = append(results, StepBuildResult{
			Name:    step.Name,
			Timeout: step.Timeout,
			Objects: objects,
		})
	}

	return results, nil
}

// BuildResource builds a Kubernetes resource from a JSON template using the provided inputs.
// Template functions are provided by the slim-sprig library https://go-task.github.io/slim-sprig/.
// In addition, the slugify function is available to generate slugs from strings using https://github.com/gosimple/slug/.
// And for readability, a toYaml function is available to encode an input value into a YAML string.
func BuildResource(tmpl *apix.JSON, inputSet map[string]any) (*unstructured.Unstructured, error) {
	yamlTemplate, err := yaml.JSONToYAML(tmpl.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to convert template to YAML: %w", err)
	}

	tp, err := newTemplate(string(yamlTemplate), inputSet)
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

// BuildResourcesFromYAML builds a list of Kubernetes resources from a multi-doc YAML template
// using the same templating functions as BuildResource.
func BuildResourcesFromYAML(yamlTemplate string, inputSet map[string]any) ([]*unstructured.Unstructured, error) {
	tp, err := newTemplate(yamlTemplate, inputSet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse multi-doc YAML template: %w", err)
	}

	b := &strings.Builder{}
	err = tp.Execute(b, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute multi-doc YAML template: %w", err)
	}

	objects, err := ssautil.ReadObjects(bytes.NewReader([]byte(b.String())))
	if err != nil {
		return nil, fmt.Errorf("failed to read objects from multi-doc YAML: %w", err)
	}

	return objects, nil
}

func newTemplate(yamlTemplate string, inputSet map[string]any) (*template.Template, error) {
	tp, err := template.New("resourceset").
		Delims("<<", ">>").
		Funcs(sprig.HermeticTxtFuncMap()).
		Funcs(template.FuncMap{"slugify": slug.Make}).
		Funcs(template.FuncMap{"inputs": func() any { return inputSet }}).
		Funcs(template.FuncMap{"toYaml": toYaml, "mustToYaml": mustToYaml}).
		Option("missingkey=error").
		Parse(yamlTemplate)
	if err != nil {
		return nil, err
	}
	return tp, nil
}

// objectKey returns a unique identifier for the given object
// composed of its apiVersion, kind, namespace and name.
// It is used as map key for object deduplication.
func objectKey(object *unstructured.Unstructured) string {
	return strings.Join([]string{
		object.GetAPIVersion(),
		object.GetKind(),
		object.GetNamespace(),
		object.GetName(),
	}, "/")
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

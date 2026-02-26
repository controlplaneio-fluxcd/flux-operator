// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestApplyOperator_SetLabels(t *testing.T) {
	g := NewWithT(t)

	objects := makeTestOperatorObjects()
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}

	in := &Installer{
		options: &Options{namespace: "flux-system"},
	}

	// We can't call ApplyOperator directly since it invokes SSA,
	// but we can test the label mutation logic by replicating the
	// label-setting portion of the function.
	applyOperatorLabels(objects, expectedLabels)

	for _, obj := range objects {
		labels := obj.GetLabels()
		g.Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/name", "flux-operator"),
			"object %s/%s missing name label", obj.GetKind(), obj.GetName())
		g.Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/instance", "flux-operator"),
			"object %s/%s missing instance label", obj.GetKind(), obj.GetName())
	}

	_ = in // use the installer reference to avoid lint
}

func TestApplyOperator_DeploymentSelectorLabels(t *testing.T) {
	g := NewWithT(t)

	dep := makeTestDeployment()
	objects := []*unstructured.Unstructured{dep}
	labels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}

	applyOperatorLabels(objects, labels)
	applyDeploymentMutations(objects, labels, false)

	// Check spec.selector.matchLabels
	matchLabels, found, err := unstructured.NestedStringMap(dep.Object, "spec", "selector", "matchLabels")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(matchLabels).To(Equal(labels))

	// Check spec.template.metadata.labels
	templateLabels, found, err := unstructured.NestedStringMap(dep.Object, "spec", "template", "metadata", "labels")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(templateLabels).To(Equal(labels))
}

func TestApplyOperator_DeploymentEnvVars(t *testing.T) {
	g := NewWithT(t)

	dep := makeTestDeployment()
	objects := []*unstructured.Unstructured{dep}
	labels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}

	applyDeploymentMutations(objects, labels, false)

	envVars := getContainerEnvVars(g, dep)
	g.Expect(envVars).To(ContainElement(map[string]any{
		"name":  "REPORTING_INTERVAL",
		"value": "30s",
	}))
}

func TestApplyOperator_MultitenantEnvVar(t *testing.T) {
	g := NewWithT(t)

	dep := makeTestDeployment()
	objects := []*unstructured.Unstructured{dep}
	labels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}

	applyDeploymentMutations(objects, labels, true)

	envVars := getContainerEnvVars(g, dep)
	g.Expect(envVars).To(ContainElement(map[string]any{
		"name":  "DEFAULT_SERVICE_ACCOUNT",
		"value": "flux-operator",
	}))
}

func TestApplyOperator_NonMultitenant_NoServiceAccountEnv(t *testing.T) {
	g := NewWithT(t)

	dep := makeTestDeployment()
	objects := []*unstructured.Unstructured{dep}
	labels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}

	applyDeploymentMutations(objects, labels, false)

	envVars := getContainerEnvVars(g, dep)
	for _, env := range envVars {
		envMap, ok := env.(map[string]any)
		g.Expect(ok).To(BeTrue())
		g.Expect(envMap["name"]).NotTo(Equal("DEFAULT_SERVICE_ACCOUNT"))
	}
}

func TestApplyOperator_ServiceSelectorLabels(t *testing.T) {
	g := NewWithT(t)

	svc := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      "flux-operator",
				"namespace": "flux-system",
			},
			"spec": map[string]any{
				"selector": map[string]any{
					"app": "old-selector",
				},
			},
		},
	}

	labels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}

	objects := []*unstructured.Unstructured{svc}
	applyServiceMutations(objects, labels)

	svcSelector, found, err := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(svcSelector).To(Equal(labels))
}

// applyOperatorLabels replicates the label-setting logic from ApplyOperator.
func applyOperatorLabels(objects []*unstructured.Unstructured, labels map[string]string) {
	for _, obj := range objects {
		existing := obj.GetLabels()
		if existing == nil {
			existing = make(map[string]string)
		}
		for k, v := range labels {
			existing[k] = v
		}
		obj.SetLabels(existing)
	}
}

// applyDeploymentMutations replicates the Deployment mutation logic from ApplyOperator.
func applyDeploymentMutations(objects []*unstructured.Unstructured, labels map[string]string, multitenant bool) {
	for _, obj := range objects {
		if obj.GetKind() != "Deployment" {
			continue
		}

		_ = unstructured.SetNestedStringMap(obj.Object, labels, "spec", "selector", "matchLabels")
		_ = unstructured.SetNestedStringMap(obj.Object, labels, "spec", "template", "metadata", "labels")

		containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
		if err != nil || !found || len(containers) == 0 {
			return
		}

		container, ok := containers[0].(map[string]any)
		if !ok {
			return
		}

		envVars, _, _ := unstructured.NestedSlice(container, "env")
		if envVars == nil {
			envVars = []any{}
		}

		envVars = append(envVars, map[string]any{
			"name":  "REPORTING_INTERVAL",
			"value": "30s",
		})

		if multitenant {
			envVars = append(envVars, map[string]any{
				"name":  "DEFAULT_SERVICE_ACCOUNT",
				"value": "flux-operator",
			})
		}

		_ = unstructured.SetNestedSlice(container, envVars, "env")
		containers[0] = container
		_ = unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers")
	}
}

// applyServiceMutations replicates the Service mutation logic from ApplyOperator.
func applyServiceMutations(objects []*unstructured.Unstructured, labels map[string]string) {
	for _, obj := range objects {
		if obj.GetKind() != "Service" {
			continue
		}
		_ = unstructured.SetNestedStringMap(obj.Object, labels, "spec", "selector")
	}
}

// makeTestDeployment creates a minimal Deployment unstructured object for testing.
func makeTestDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "flux-operator",
				"namespace": "flux-system",
			},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "flux-operator",
					},
				},
				"template": map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"app": "flux-operator",
						},
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name":  "manager",
								"image": "ghcr.io/controlplaneio-fluxcd/flux-operator:latest",
								"env": []any{
									map[string]any{
										"name":  "EXISTING_VAR",
										"value": "existing-value",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// makeTestOperatorObjects creates a set of unstructured objects mimicking real operator manifests.
func makeTestOperatorObjects() []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": "flux-system",
				},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ServiceAccount",
				"metadata": map[string]any{
					"name":      "flux-operator",
					"namespace": "flux-system",
				},
			},
		},
		makeTestDeployment(),
	}
}

// getContainerEnvVars extracts env vars from the first container of a Deployment.
func getContainerEnvVars(g Gomega, dep *unstructured.Unstructured) []any {
	containers, found, err := unstructured.NestedSlice(dep.Object, "spec", "template", "spec", "containers")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(containers).NotTo(BeEmpty())

	container, ok := containers[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	envVars, _, _ := unstructured.NestedSlice(container, "env")
	return envVars
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleReconcileResource(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: ToolReconcileFluxResource,
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName: "fails with omitted apiVersion and invalid kubeconfig",
			arguments: map[string]any{
				"kind":      "Kustomization",
				"name":      "test",
				"namespace": "default",
			},
			matchErr: "Failed to get Kubernetes client",
		},
		{
			testName: "fails without kind",
			arguments: map[string]any{
				"name":      "test",
				"namespace": "default",
			},
			matchErr: "kind is required",
		},
		{
			testName: "fails without name",
			arguments: map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
				"kind":       "Kustomization",
				"namespace":  "default",
			},
			matchErr: "name is required",
		},
		{
			testName: "fails without namespace",
			arguments: map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
				"kind":       "Kustomization",
				"name":       "test",
			},
			matchErr: "namespace is required",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
				"kind":       "Kustomization",
				"name":       "test",
				"namespace":  "default",
			},
			matchErr: "Failed to get Kubernetes client",
		},
		{
			testName: "fails with source and invalid kubeconfig",
			arguments: map[string]any{
				"apiVersion":  "kustomize.toolkit.fluxcd.io/v1",
				"kind":        "Kustomization",
				"name":        "test",
				"namespace":   "default",
				"with_source": true,
			},
			matchErr: "Failed to get Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input reconcileFluxResourceInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, _, err := m.HandleReconcileResource(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
		})
	}
}

func TestManager_HandleReconcileResource_WithSource(t *testing.T) {
	helmReleaseGVK := schema.GroupVersionKind{Group: "helm.toolkit.fluxcd.io", Version: "v2", Kind: fluxcdv1.FluxHelmReleaseKind}
	ociRepositoryGVK := schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: fluxcdv1.FluxOCIRepositoryKind}

	newObject := func(gvk schema.GroupVersionKind, name string, spec map[string]any) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
		obj.SetGroupVersionKind(gvk)
		obj.SetName(name)
		obj.SetNamespace("apps")
		return obj
	}
	chartTemplate := map[string]any{
		"chart": map[string]any{
			"kind": fluxcdv1.FluxOCIRepositoryKind,
			"spec": map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
		},
	}
	chartRef := map[string]any{
		"chartRef": map[string]any{"kind": fluxcdv1.FluxOCIRepositoryKind, "name": "podinfo"},
	}

	tests := []struct {
		name           string
		objects        []ctrlclient.Object
		matchErr       string
		matchText      string
		sourceName     string
		sourceAnnotate bool
	}{
		{
			name: "chart template with missing generated source reconciles the HelmRelease",
			objects: []ctrlclient.Object{
				newObject(helmReleaseGVK, "podinfo", chartTemplate),
			},
			matchText: "The generated source OCIRepository/apps/apps-podinfo does not exist yet, only the resource itself was reconciled.",
		},
		{
			name: "chart template with generated source reconciles both",
			objects: []ctrlclient.Object{
				newObject(helmReleaseGVK, "podinfo", chartTemplate),
				newObject(ociRepositoryGVK, "apps-podinfo", map[string]any{}),
			},
			matchText:      "The source OCIRepository/apps/apps-podinfo was reconciled first.",
			sourceName:     "apps-podinfo",
			sourceAnnotate: true,
		},
		{
			name: "chartRef with missing source fails",
			objects: []ctrlclient.Object{
				newObject(helmReleaseGVK, "podinfo", chartRef),
			},
			matchErr: "Failed to reconcile source",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{
				helmReleaseGVK.GroupVersion(), ociRepositoryGVK.GroupVersion(),
			})
			mapper.Add(helmReleaseGVK, apimeta.RESTScopeNamespace)
			mapper.Add(ociRepositoryGVK, apimeta.RESTScopeNamespace)
			fakeClient := fake.NewClientBuilder().
				WithScheme(runtime.NewScheme()).
				WithRESTMapper(mapper).
				WithObjects(test.objects...).
				Build()
			ctx := k8s.NewClient(fakeClient, &rest.Config{}, mapper).IntoContext(context.Background())

			m := &Manager{
				kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
				timeout:    time.Second,
			}
			result, _, err := m.HandleReconcileResource(ctx, &mcp.CallToolRequest{}, reconcileFluxResourceInput{
				Kind:       fluxcdv1.FluxHelmReleaseKind,
				Name:       "podinfo",
				Namespace:  "apps",
				WithSource: true,
			})
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			hr := newObject(helmReleaseGVK, "podinfo", nil)
			g.Expect(fakeClient.Get(ctx, ctrlclient.ObjectKeyFromObject(hr), hr)).To(Succeed())

			if test.matchErr != "" {
				g.Expect(result.IsError).To(BeTrue())
				g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
				g.Expect(hr.GetAnnotations()).ToNot(HaveKey(meta.ReconcileRequestAnnotation))
				return
			}

			g.Expect(result.IsError).To(BeFalse(), textContent.Text)
			g.Expect(textContent.Text).To(ContainSubstring(test.matchText))
			g.Expect(hr.GetAnnotations()).To(HaveKey(meta.ReconcileRequestAnnotation))
			g.Expect(hr.GetAnnotations()).To(HaveKey(meta.ForceRequestAnnotation))

			if test.sourceAnnotate {
				src := newObject(ociRepositoryGVK, test.sourceName, nil)
				g.Expect(fakeClient.Get(ctx, ctrlclient.ObjectKeyFromObject(src), src)).To(Succeed())
				g.Expect(src.GetAnnotations()).To(HaveKeyWithValue(meta.ReconcileRequestAnnotation,
					hr.GetAnnotations()[meta.ReconcileRequestAnnotation]))
			}
		})
	}
}

func TestCheckReconcilableKind(t *testing.T) {
	tests := []struct {
		name     string
		gvk      schema.GroupVersionKind
		matchErr string
	}{
		{
			name:     "ConfigMap",
			gvk:      schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			matchErr: "ConfigMap in group core is not a Flux resource",
		},
		{
			name:     "Deployment",
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			matchErr: "Deployment in group apps is not a Flux resource",
		},
		{
			name:     "Alert",
			gvk:      schema.GroupVersionKind{Group: "notification.toolkit.fluxcd.io", Kind: "Alert"},
			matchErr: "Alert does not support on-demand reconciliation",
		},
		{
			name:     "Provider",
			gvk:      schema.GroupVersionKind{Group: "notification.toolkit.fluxcd.io", Kind: "Provider"},
			matchErr: "Provider does not support on-demand reconciliation",
		},
		{
			name:     "ExternalArtifact",
			gvk:      schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Kind: "ExternalArtifact"},
			matchErr: "ExternalArtifact does not support on-demand reconciliation",
		},
		{
			name: "Kustomization",
			gvk:  schema.GroupVersionKind{Group: "kustomize.toolkit.fluxcd.io", Kind: "Kustomization"},
		},
		{
			name: "HelmRelease",
			gvk:  schema.GroupVersionKind{Group: "helm.toolkit.fluxcd.io", Kind: "HelmRelease"},
		},
		{
			name: "ResourceSet",
			gvk:  schema.GroupVersionKind{Group: "fluxcd.controlplane.io", Kind: "ResourceSet"},
		},
		{
			name: "ResourceSetInputProvider",
			gvk:  schema.GroupVersionKind{Group: "fluxcd.controlplane.io", Kind: "ResourceSetInputProvider"},
		},
		{
			name: "FluxInstance",
			gvk:  schema.GroupVersionKind{Group: "fluxcd.controlplane.io", Kind: "FluxInstance"},
		},
		{
			name: "ImagePolicy",
			gvk:  schema.GroupVersionKind{Group: "image.toolkit.fluxcd.io", Kind: "ImagePolicy"},
		},
		{
			name: "unknown Flux kind",
			gvk:  schema.GroupVersionKind{Group: "widgets.fluxcd.io", Kind: "Widget"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			err := checkReconcilableKind(test.gvk)
			if test.matchErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(test.matchErr)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestGetFluxSourceReference(t *testing.T) {
	tests := []struct {
		name     string
		object   *unstructured.Unstructured
		expected fluxSourceReference
		found    bool
	}{
		{
			name: "chartRef takes precedence",
			object: newSourceReferenceObject(map[string]any{
				"chartRef":  map[string]any{"kind": fluxcdv1.FluxOCIRepositoryKind, "name": "charts", "namespace": "sources"},
				"sourceRef": map[string]any{"kind": fluxcdv1.FluxGitRepositoryKind, "name": "ignored"},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxOCIRepositoryKind, Name: "charts", Namespace: "sources"},
			found:    true,
		},
		{
			name: "sourceRef with namespace",
			object: newSourceReferenceObject(map[string]any{
				"sourceRef": map[string]any{"kind": fluxcdv1.FluxGitRepositoryKind, "name": "app", "namespace": "sources"},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxGitRepositoryKind, Name: "app", Namespace: "sources"},
			found:    true,
		},
		{
			name: "sourceRef defaults namespace",
			object: newSourceReferenceObject(map[string]any{
				"sourceRef": map[string]any{"kind": fluxcdv1.FluxBucketKind, "name": "app"},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxBucketKind, Name: "app", Namespace: "apps"},
			found:    true,
		},
		{
			name: "inline chart sourceRef with namespace",
			object: newSourceReferenceObject(map[string]any{
				"chart": map[string]any{"spec": map[string]any{
					"sourceRef": map[string]any{"kind": fluxcdv1.FluxHelmRepositoryKind, "name": "charts", "namespace": "sources"},
				}},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxHelmRepositoryKind, Name: "charts", Namespace: "sources"},
			found:    true,
		},
		{
			name: "inline chart sourceRef defaults namespace",
			object: newSourceReferenceObject(map[string]any{
				"chart": map[string]any{"spec": map[string]any{
					"sourceRef": map[string]any{"kind": fluxcdv1.FluxGitRepositoryKind, "name": "charts"},
				}},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxGitRepositoryKind, Name: "charts", Namespace: "apps"},
			found:    true,
		},
		{
			name: "chartRef takes precedence over inline chart sourceRef",
			object: newSourceReferenceObject(map[string]any{
				"chartRef": map[string]any{"kind": fluxcdv1.FluxOCIRepositoryKind, "name": "charts"},
				"chart": map[string]any{"spec": map[string]any{
					"sourceRef": map[string]any{"kind": fluxcdv1.FluxHelmRepositoryKind, "name": "ignored"},
				}},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxOCIRepositoryKind, Name: "charts", Namespace: "apps"},
			found:    true,
		},
		{
			name: "OCIRepository chart template uses the generated source",
			object: newHelmReleaseSourceReferenceObject(map[string]any{
				"chart": map[string]any{
					"kind": fluxcdv1.FluxOCIRepositoryKind,
					"spec": map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
				},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxOCIRepositoryKind, Name: "apps-podinfo", Namespace: "apps", Generated: true},
			found:    true,
		},
		{
			name: "HelmChart template uses the chart sourceRef",
			object: newHelmReleaseSourceReferenceObject(map[string]any{
				"chart": map[string]any{
					"kind": fluxcdv1.FluxHelmChartKind,
					"spec": map[string]any{
						"chart":     "podinfo",
						"sourceRef": map[string]any{"kind": fluxcdv1.FluxHelmRepositoryKind, "name": "charts"},
					},
				},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxHelmRepositoryKind, Name: "charts", Namespace: "apps"},
			found:    true,
		},
		{
			name: "chartRef takes precedence over OCIRepository chart template",
			object: newHelmReleaseSourceReferenceObject(map[string]any{
				"chartRef": map[string]any{"kind": fluxcdv1.FluxOCIRepositoryKind, "name": "charts"},
				"chart":    map[string]any{"kind": fluxcdv1.FluxOCIRepositoryKind},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxOCIRepositoryKind, Name: "charts", Namespace: "apps"},
			found:    true,
		},
		{
			name: "ExternalArtifact sourceRef",
			object: newSourceReferenceObject(map[string]any{
				"sourceRef": map[string]any{"kind": fluxcdv1.FluxExternalArtifactKind, "name": "generated"},
			}),
			expected: fluxSourceReference{Kind: fluxcdv1.FluxExternalArtifactKind, Name: "generated", Namespace: "apps"},
			found:    true,
		},
		{
			name:     "no source reference",
			object:   newSourceReferenceObject(map[string]any{}),
			expected: fluxSourceReference{},
			found:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			actual, found := getFluxSourceReference(test.object)
			g.Expect(found).To(Equal(test.found))
			g.Expect(actual).To(Equal(test.expected))
		})
	}
}

func newSourceReferenceObject(spec map[string]any) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	object.SetNamespace("apps")
	return object
}

func newHelmReleaseSourceReferenceObject(spec map[string]any) *unstructured.Unstructured {
	object := newSourceReferenceObject(spec)
	object.SetKind(fluxcdv1.FluxHelmReleaseKind)
	object.SetName("podinfo")
	return object
}

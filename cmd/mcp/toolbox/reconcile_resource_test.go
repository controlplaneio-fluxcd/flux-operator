// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

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

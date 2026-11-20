// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolReconcileFluxChain is the name of the reconcile_flux_chain tool.
	ToolReconcileFluxChain = "reconcile_flux_chain"
)

func init() {
	systemTools[ToolReconcileFluxChain] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// reconcileFluxChainInput defines the input parameters for reconciling a Flux Kustomization
// and its entire dependency chain.
type reconcileFluxChainInput struct {
	Name       string `json:"name" jsonschema:"The name of the target Flux Kustomization."`
	Namespace  string `json:"namespace" jsonschema:"The namespace of the target Flux Kustomization."`
	WithSource bool   `json:"with_source,omitempty" jsonschema:"If true the source will be reconciled first."`
}

// reconcileChainResult tracks the reconciliation of each layer.
type reconcileChainResult struct {
	Layer     int    `json:"layer"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// HandleReconcileChain is the handler function for the reconcile_flux_chain tool.
// It walks the dependency chain of a Kustomization and reconciles each layer
// from the root (no dependencies) to the target.
func (m *Manager) HandleReconcileChain(ctx context.Context, request *mcp.CallToolRequest, input reconcileFluxChainInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolReconcileFluxChain, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.Name == "" {
		return NewToolResultError("name is required")
	}
	if input.Namespace == "" {
		return NewToolResultError("namespace is required")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	// Build the dependency chain by walking dependsOn references
	chain, err := buildDependencyChain(ctx, kubeClient, input.Name, input.Namespace)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to build dependency chain", err)
	}

	var results []reconcileChainResult
	ts := time.Now().Format(time.RFC3339Nano)

	// Reconcile source first if requested
	if input.WithSource {
		target := chain[len(chain)-1]
		ks := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": fluxcdv1.FluxKustomizeGroup + "/v1",
				"kind":       fluxcdv1.FluxKustomizationKind,
			},
		}
		ks.SetName(target.name)
		ks.SetNamespace(target.namespace)

		if err := kubeClient.Get(ctx, ctrlclient.ObjectKeyFromObject(ks), ks); err != nil {
			return NewToolResultErrorFromErr("Failed to get target Kustomization", err)
		}

		sourceRefKind, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "kind")
		sourceRefName, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "name")
		sourceRefNamespace, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "namespace")
		if sourceRefNamespace == "" {
			sourceRefNamespace = input.Namespace
		}

		if sourceRefName != "" {
			sourceErr := reconcileSource(ctx, kubeClient, sourceRefKind, sourceRefName, sourceRefNamespace, ts)
			result := reconcileChainResult{
				Layer:     0,
				Name:      sourceRefName,
				Namespace: sourceRefNamespace,
				Status:    "reconciled",
			}
			if sourceErr != nil {
				result.Status = "error"
				result.Message = sourceErr.Error()
			}
			results = append(results, result)
		}
	}

	// Reconcile each layer of the dependency chain
	for i, dep := range chain {
		err := kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxKustomizeGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxKustomizationKind,
			},
			dep.name,
			dep.namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)

		result := reconcileChainResult{
			Layer:     i + 1,
			Name:      dep.name,
			Namespace: dep.namespace,
			Status:    "reconciled",
		}
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
		}
		results = append(results, result)
	}

	// Build output message
	var sb strings.Builder
	fmt.Fprintf(&sb, "Reconciliation triggered for %d Kustomization(s) in dependency order:\n\n", len(chain))

	for _, r := range results {
		status := "✓"
		if r.Status == "error" {
			status = "✗"
		}
		if r.Layer == 0 {
			fmt.Fprintf(&sb, "  [Source] %s %s/%s", status, r.Namespace, r.Name)
		} else {
			fmt.Fprintf(&sb, "  [Layer %d] %s %s/%s", r.Layer, status, r.Namespace, r.Name)
		}
		if r.Message != "" {
			fmt.Fprintf(&sb, " - %s", r.Message)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nTo verify, check that each Kustomization's status.lastHandledReconcileAt matches the requestedAt annotation.")

	return NewToolResultText(sb.String())
}

// dependencyNode represents a Kustomization in the dependency chain.
type dependencyNode struct {
	name      string
	namespace string
}

// buildDependencyChain walks the dependsOn references and returns the chain
// in reconciliation order (roots first, target last).
func buildDependencyChain(ctx context.Context, kubeClient *k8s.Client, targetName, targetNamespace string) ([]dependencyNode, error) {
	// Use a map to track visited nodes and detect cycles
	visited := make(map[string]bool)
	var chain []dependencyNode

	var walk func(name, namespace string) error
	walk = func(name, namespace string) error {
		key := fmt.Sprintf("%s/%s", namespace, name)
		if visited[key] {
			return nil // Already processed
		}
		visited[key] = true

		ks := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": fluxcdv1.FluxKustomizeGroup + "/v1",
				"kind":       fluxcdv1.FluxKustomizationKind,
			},
		}
		ks.SetName(name)
		ks.SetNamespace(namespace)

		if err := kubeClient.Get(ctx, ctrlclient.ObjectKeyFromObject(ks), ks); err != nil {
			return fmt.Errorf("failed to get Kustomization %s/%s: %w", namespace, name, err)
		}

		// Get dependsOn references
		dependsOn, _, _ := unstructured.NestedSlice(ks.Object, "spec", "dependsOn")
		for _, dep := range dependsOn {
			depMap, ok := dep.(map[string]any)
			if !ok {
				continue
			}
			depName, _ := depMap["name"].(string)
			depNamespace, _ := depMap["namespace"].(string)
			if depNamespace == "" {
				depNamespace = namespace
			}
			if depName != "" {
				if err := walk(depName, depNamespace); err != nil {
					return err
				}
			}
		}

		// Add this node after its dependencies
		chain = append(chain, dependencyNode{name: name, namespace: namespace})
		return nil
	}

	if err := walk(targetName, targetNamespace); err != nil {
		return nil, err
	}

	return chain, nil
}

// reconcileSource triggers reconciliation of a Flux source.
func reconcileSource(ctx context.Context, kubeClient *k8s.Client, kind, name, namespace, ts string) error {
	switch kind {
	case fluxcdv1.FluxGitRepositoryKind:
		return kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxGitRepositoryKind,
			},
			name, namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxBucketKind:
		return kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxBucketKind,
			},
			name, namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxOCIRepositoryKind:
		return kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1beta2",
				Kind:    fluxcdv1.FluxOCIRepositoryKind,
			},
			name, namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	default:
		return fmt.Errorf("unknown sourceRef kind %s", kind)
	}
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// ToolReconcileFluxResource is the name of the reconcile_flux_resource tool.
	ToolReconcileFluxResource = "reconcile_flux_resource"
)

func init() {
	systemTools[ToolReconcileFluxResource] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// reconcileFluxResourceInput defines the input parameters for reconciling a Flux resource.
type reconcileFluxResourceInput struct {
	APIVersion string `json:"apiVersion,omitempty" jsonschema:"The apiVersion of the Flux resource. Optional, when omitted it is resolved from the kind."`
	Kind       string `json:"kind" jsonschema:"The kind of the Flux resource e.g. Kustomization, HelmRelease, ResourceSet, ResourceSetInputProvider, FluxInstance, GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket, ImageRepository, ImagePolicy, ImageUpdateAutomation, Receiver."`
	Name       string `json:"name" jsonschema:"The name of the Flux resource."`
	Namespace  string `json:"namespace" jsonschema:"The namespace of the Flux resource."`
	WithSource bool   `json:"with_source,omitempty" jsonschema:"If true, the source referenced by the resource (spec.sourceRef, spec.chartRef or spec.chart.spec.sourceRef) is reconciled first. Applies to Kustomization and HelmRelease."`
}

// fluxSourceReference identifies a Flux source referenced by another resource.
type fluxSourceReference struct {
	Kind      string
	Name      string
	Namespace string
}

// HandleReconcileResource is the handler function for the reconcile_flux_resource tool.
func (m *Manager) HandleReconcileResource(ctx context.Context, request *mcp.CallToolRequest, input reconcileFluxResourceInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolReconcileFluxResource, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.Kind == "" {
		return NewToolResultError("kind is required")
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

	gvk, err := kubeClient.ResolveGroupVersionKind(input.APIVersion, input.Kind)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to resolve group version kind", err)
	}
	if err := checkReconcilableKind(gvk); err != nil {
		return NewToolResultError(err.Error())
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	object.SetName(input.Name)
	object.SetNamespace(input.Namespace)
	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(object), object); err != nil {
		return NewToolResultErrorFromErr(fmt.Sprintf("Failed to get %s", gvk.Kind), err)
	}

	ts := time.Now().Format(time.RFC3339Nano)
	var sourceRef *fluxSourceReference
	sourceReconciled := false
	if input.WithSource {
		if ref, found := getFluxSourceReference(object); found {
			sourceRef = &ref
			if ref.Kind != fluxcdv1.FluxExternalArtifactKind {
				if !isReconcilableSourceKind(ref.Kind) {
					return NewToolResultError(fmt.Sprintf("Unknown source kind %s", ref.Kind))
				}
				sourceGVK, err := kubeClient.ResolveGroupVersionKind("", ref.Kind)
				if err != nil {
					return NewToolResultErrorFromErr("Failed to resolve source kind", err)
				}
				if err := kubeClient.Annotate(ctx, sourceGVK, ref.Name, ref.Namespace,
					[]string{meta.ReconcileRequestAnnotation}, ts); err != nil {
					return NewToolResultErrorFromErr("Failed to reconcile source", err)
				}
				sourceReconciled = true
			}
		}
	}

	keys := []string{meta.ReconcileRequestAnnotation}
	forced := gvk.Kind == fluxcdv1.FluxHelmReleaseKind || gvk.Kind == fluxcdv1.ResourceSetInputProviderKind
	if forced {
		keys = append(keys, meta.ForceRequestAnnotation)
	}
	if err := kubeClient.Annotate(ctx, gvk, input.Name, input.Namespace, keys, ts); err != nil {
		return NewToolResultErrorFromErr(fmt.Sprintf("Failed to reconcile %s", gvk.Kind), err)
	}

	return NewToolResultText(reconcileResourceResult(gvk.Kind, input.WithSource, sourceRef, sourceReconciled, forced))
}

// checkReconcilableKind verifies that the kind belongs to a Flux API and supports on-demand reconciliation.
func checkReconcilableKind(gvk schema.GroupVersionKind) error {
	if !fluxcdv1.IsFluxAPI(gvk.Group) {
		group := gvk.Group
		if group == "" {
			group = "core"
		}
		return fmt.Errorf("%s in group %s is not a Flux resource, only Flux resources can be reconciled", gvk.Kind, group)
	}

	if kindInfo, err := fluxcdv1.FindFluxKindInfo(gvk.Kind); err == nil && !kindInfo.Reconcilable {
		return fmt.Errorf("%s does not support on-demand reconciliation", gvk.Kind)
	}

	return nil
}

// getFluxSourceReference returns the chartRef, sourceRef or inline chart sourceRef from a Flux resource, in that order.
func getFluxSourceReference(object *unstructured.Unstructured) (fluxSourceReference, bool) {
	for _, field := range [][]string{{"spec", "chartRef"}, {"spec", "sourceRef"}, {"spec", "chart", "spec", "sourceRef"}} {
		kind, found, err := unstructured.NestedString(object.Object, append(field, "kind")...)
		if err != nil || !found || kind == "" {
			continue
		}
		name, _, _ := unstructured.NestedString(object.Object, append(field, "name")...)
		namespace, _, _ := unstructured.NestedString(object.Object, append(field, "namespace")...)
		if namespace == "" {
			namespace = object.GetNamespace()
		}
		return fluxSourceReference{Kind: kind, Name: name, Namespace: namespace}, true
	}
	return fluxSourceReference{}, false
}

// isReconcilableSourceKind reports whether the source kind supports on-demand reconciliation.
func isReconcilableSourceKind(kind string) bool {
	switch kind {
	case fluxcdv1.FluxGitRepositoryKind,
		fluxcdv1.FluxOCIRepositoryKind,
		fluxcdv1.FluxBucketKind,
		fluxcdv1.FluxHelmChartKind,
		fluxcdv1.FluxHelmRepositoryKind:
		return true
	default:
		return false
	}
}

// reconcileResourceResult builds the reconciliation confirmation and verification hint.
func reconcileResourceResult(kind string, withSource bool, sourceRef *fluxSourceReference, sourceReconciled, forced bool) string {
	result := fmt.Sprintf("%s reconciliation triggered.", kind)
	if withSource {
		switch {
		case sourceRef == nil:
			result += " The resource has no source reference, only the resource itself was reconciled."
		case sourceRef.Kind == fluxcdv1.FluxExternalArtifactKind:
			result += fmt.Sprintf(" The source ExternalArtifact/%s/%s is generated and cannot be reconciled on demand, only the resource itself was reconciled.", sourceRef.Namespace, sourceRef.Name)
		case sourceReconciled:
			result += fmt.Sprintf(" The source %s/%s/%s was reconciled first.", sourceRef.Kind, sourceRef.Namespace, sourceRef.Name)
		}
	}
	if forced {
		return result + "\nTo verify check the status lastHandledForceAt field matches the forceAt annotation."
	}
	return result + "\nTo verify check the status lastHandledReconcileAt field matches the requestedAt annotation."
}

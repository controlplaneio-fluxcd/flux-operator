// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// ResourceHandler handles GET /api/v1/resource requests and returns a single Flux resource by kind, name and namespace.
// Query parameters: kind, name, namespace (all required)
// Example: /api/v1/resource?kind=FluxInstance&name=flux&namespace=flux-system
func (h *Handler) ResourceHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	queryParams := req.URL.Query()
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")

	// Validate required parameters
	if kind == "" || name == "" || namespace == "" {
		http.Error(w, "Missing required parameters: kind, name, namespace", http.StatusBadRequest)
		return
	}

	// Get the resource from the cluster
	resource, err := h.GetResource(req.Context(), kind, name, namespace)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get resource")
		switch {
		case errors.IsNotFound(err):
			// return empty response if resource not found
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case errors.IsForbidden(err):
			perms := user.Permissions(req.Context())
			msg := fmt.Sprintf("You do not have access to this resource. "+
				"Contact your administrator if you believe this is an error. "+
				"User: %s, Groups: [%s]",
				perms.Username, strings.Join(perms.Groups, ", "))
			http.Error(w, msg, http.StatusForbidden)
		default:
			http.Error(w, fmt.Sprintf("Failed to get resource: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Encode and send the response
	if err := json.NewEncoder(w).Encode(resource.Object); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetResource fetches a single Flux resource by kind, name and namespace,
// and injects the inventory into the .status.inventory field before returning it.
func (h *Handler) GetResource(ctx context.Context, kind, name, namespace string) (*unstructured.Unstructured, error) {
	kindInfo, err := findFluxKindInfo(kind)
	if err != nil {
		return nil, fmt.Errorf("unable to find Flux kind %s: %w", kind, err)
	}

	// Get the preferred GVK for the kind
	gvk, err := h.preferredFluxGVK(ctx, kindInfo.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get GVK for kind %s: %w", kind, err)
	}

	// Create an unstructured object to fetch the resource
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*gvk)

	// Create the object key
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	// Fetch the resource from the cluster
	if err := h.kubeClient.GetClient(ctx).Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("unable to get resource %s/%s in namespace %s: %w", kind, name, namespace, err)
	}

	// Inject the reconciler reference
	status := h.resourceStatusFromUnstructured(*obj)
	reconciler := getReconcilerRef(obj)
	reconcilerRef := map[string]any{
		"status":         status.Status,
		"message":        status.Message,
		"lastReconciled": status.LastReconciled.Format(time.RFC3339),
		"managedBy":      reconciler,
	}
	if err := unstructured.SetNestedMap(obj.Object, reconcilerRef, "status", "reconcilerRef"); err != nil {
		return nil, fmt.Errorf("unable to set reconcilerRef: %w", err)
	}

	// Get the inventory for this resource
	var inventoryError string
	inventory, err := h.getInventory(ctx, *obj)
	if err != nil {
		if !errors.IsForbidden(err) {
			return nil, err
		}
		log.FromContext(ctx).Error(err, "user does not have access to resource inventory")
		perms := user.Permissions(ctx)
		inventoryError = fmt.Sprintf("You do not have access to the inventory of this resource. "+
			"Contact your administrator if you believe this is an error. "+
			"User: %s, Groups: [%s]",
			perms.Username, strings.Join(perms.Groups, ", "))
	}

	// Inject/override the .status.inventory field with the extracted inventory
	if len(inventory) > 0 {
		// Convert inventory entries to the format expected in status.inventory.entries
		entries := make([]any, 0, len(inventory))
		for _, entry := range inventory {
			entries = append(entries, map[string]any{
				"name":       entry.Name,
				"namespace":  entry.Namespace,
				"kind":       entry.Kind,
				"apiVersion": entry.APIVersion,
			})
		}

		// Set the inventory in the status field
		if err := unstructured.SetNestedSlice(obj.Object, entries, "status", "inventory"); err != nil {
			return nil, fmt.Errorf("unable to set inventory in status: %w", err)
		}
	}

	// Inject inventory error if any.
	if inventoryError != "" {
		if err := unstructured.SetNestedField(obj.Object, inventoryError, "status", "inventoryError"); err != nil {
			return nil, fmt.Errorf("unable to set inventoryError in status: %w", err)
		}
	}

	// Get the source reference and inject the source details if available
	if source, err := h.getReconcilerSource(ctx, *obj); err == nil && source != nil {
		sourceMap := map[string]any{
			"kind":           source.Kind,
			"name":           source.Name,
			"namespace":      source.Namespace,
			"url":            source.URL,
			"originURL":      source.OriginURL,
			"originRevision": source.OriginRevision,
			"status":         source.Status,
			"message":        source.Message,
		}
		if err := unstructured.SetNestedMap(obj.Object, sourceMap, "status", "sourceRef"); err != nil {
			return nil, fmt.Errorf("unable to set source in spec: %w", err)
		}
	}

	// Get the input provider references for ResourceSet and inject into status
	if inputProviderRefs, err := h.getInputProviderRefs(ctx, *obj); err == nil && len(inputProviderRefs) > 0 {
		entries := make([]any, 0, len(inputProviderRefs))
		for _, entry := range inputProviderRefs {
			entries = append(entries, map[string]any{
				"name":      entry.Name,
				"namespace": entry.Namespace,
				"type":      entry.Kind,
			})
		}
		if err := unstructured.SetNestedSlice(obj.Object, entries, "status", "inputProviderRefs"); err != nil {
			return nil, fmt.Errorf("unable to set inputProviderRefs in status: %w", err)
		}
	}

	// Check if the user can perform actions on this resource (RBAC only)
	actionable := false
	canPatch, err := h.kubeClient.CanPatchResource(ctx, gvk.Group, kindInfo.Plural, namespace)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to check patch permission")
	} else {
		actionable = canPatch
	}
	if err := unstructured.SetNestedField(obj.Object, actionable, "status", "actionable"); err != nil {
		return nil, fmt.Errorf("unable to set actionable in status: %w", err)
	}

	cleanObjectForExport(obj, true)
	return obj, nil
}

// findFluxKindInfo searches for a FluxKindInfo in a case-insensitive way.
// Returns an error if the kind is not found in the fluxKinds list.
func findFluxKindInfo(kind string) (*fluxcdv1.FluxKindInfo, error) {
	fluxKinds := slices.Concat(fluxcdv1.FluxOperatorKinds, fluxcdv1.FluxKinds)
	for _, fluxKind := range fluxKinds {
		if strings.EqualFold(fluxKind.Name, kind) {
			return &fluxKind, nil
		}
		if strings.EqualFold(fluxKind.ShortName, kind) {
			return &fluxKind, nil
		}
	}
	return nil, fmt.Errorf("kind %s not found", kind)
}

// getReconcilerRef retrieves the Flux reconciler information
// from the labels of the provided unstructured object.
func getReconcilerRef(obj *unstructured.Unstructured) string {
	var kind, name, namespace string

	if obj.GetKind() == fluxcdv1.FluxExternalArtifactKind {
		if reconciler, found, _ := unstructured.NestedFieldCopy(obj.Object, "spec", "sourceRef"); found {
			if refMap, ok := reconciler.(map[string]any); ok {
				kindVal, kindFound := refMap["kind"].(string)
				nameVal, nameFound := refMap["name"].(string)
				namespaceVal, namespaceFound := refMap["namespace"].(string)
				if kindFound && nameFound && namespaceFound {
					return fmt.Sprintf("%s/%s/%s", kindVal, namespaceVal, nameVal)
				}
			}
		}
	}

	for k, v := range obj.GetLabels() {
		if k == "app.kubernetes.io/managed-by" && v == "flux-operator" {
			kind = "FluxInstance"
			name = obj.GetLabels()["fluxcd.controlplane.io/name"]
			namespace = obj.GetLabels()["fluxcd.controlplane.io/namespace"]
			break
		}

		if !fluxcdv1.IsFluxAPI(k) {
			continue
		}

		if strings.HasSuffix(k, "/name") {
			parts := strings.Split(k, ".")
			if len(parts) > 0 {
				switch parts[0] {
				case "kustomize":
					kind = fluxcdv1.FluxKustomizationKind
				case "helm":
					kind = fluxcdv1.FluxHelmReleaseKind
				case "resourceset":
					kind = fluxcdv1.ResourceSetKind
				}
			}
			name = v
		} else if strings.HasSuffix(k, "/namespace") {
			namespace = v
		}
	}

	if kind == "" || name == "" || namespace == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// getInputProviderRefs retrieves the list of ResourceSetInputProvider referenced by the given ResourceSet.
func (h *Handler) getInputProviderRefs(ctx context.Context, obj unstructured.Unstructured) ([]InventoryEntry, error) {
	if obj.GetKind() != fluxcdv1.ResourceSetKind {
		return nil, nil
	}

	inputsFrom, found, err := unstructured.NestedSlice(obj.Object, "spec", "inputsFrom")
	if err != nil || !found || len(inputsFrom) == 0 {
		return nil, nil
	}

	rsipGVK := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)
	providerMap := make(map[string]InventoryEntry)

	for _, inputSource := range inputsFrom {
		source, ok := inputSource.(map[string]any)
		if !ok {
			continue
		}

		// Get provider by name.
		if name, exists := source["name"].(string); exists && name != "" {
			mapKey := fmt.Sprintf("%s/%s/%s", rsipGVK.Kind, obj.GetNamespace(), name)
			if _, found := providerMap[mapKey]; found {
				continue
			}

			objKey := client.ObjectKey{
				Name:      name,
				Namespace: obj.GetNamespace(),
			}

			var rsip fluxcdv1.ResourceSetInputProvider
			if err := h.kubeClient.GetClient(ctx).Get(ctx, objKey, &rsip); err != nil {
				return nil, fmt.Errorf("failed to get provider %s/%s: %w", objKey.Namespace, objKey.Name, err)
			}

			providerMap[mapKey] = InventoryEntry{
				Name:      rsip.Name,
				Namespace: rsip.Namespace,
				Kind:      rsip.Spec.Type,
			}
			continue
		}

		// List providers by selector.
		if selector, exists := source["selector"].(map[string]any); exists && selector != nil {
			matchLabels, _ := selector["matchLabels"].(map[string]any)
			if matchLabels == nil {
				continue
			}

			labels := make(map[string]string)
			for k, v := range matchLabels {
				if strVal, ok := v.(string); ok {
					labels[k] = strVal
				}
			}

			var rsipList fluxcdv1.ResourceSetInputProviderList
			listOpts := []client.ListOption{
				client.InNamespace(obj.GetNamespace()),
				client.MatchingLabels(labels),
			}

			if err := h.kubeClient.GetClient(ctx).List(ctx, &rsipList, listOpts...); err != nil {
				return nil, fmt.Errorf("failed to list providers with selector: %w", err)
			}

			for _, rsip := range rsipList.Items {
				mapKey := fmt.Sprintf("%s/%s/%s", rsipGVK.Kind, rsip.Namespace, rsip.Name)
				if _, found := providerMap[mapKey]; found {
					continue
				}
				providerMap[mapKey] = InventoryEntry{
					Name:      rsip.Name,
					Namespace: rsip.Namespace,
					Kind:      rsip.Spec.Type,
				}
			}
		}
	}

	// Convert map to slice and sort by name
	result := make([]InventoryEntry, 0, len(providerMap))
	for _, entry := range providerMap {
		result = append(result, entry)
	}
	slices.SortFunc(result, func(a, b InventoryEntry) int {
		return strings.Compare(a.Name, b.Name)
	})

	return result, nil
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// ActionRequest represents the request body for POST /api/v1/action.
type ActionRequest struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Action    string `json:"action"` // "reconcile", "suspend", "resume"
}

// ActionResponse represents the response body for POST /api/v1/action.
type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ActionHandler handles POST /api/v1/action requests to perform actions on Flux resources.
func (h *Handler) ActionHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var actionReq ActionRequest
	if err := json.NewDecoder(req.Body).Decode(&actionReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if actionReq.Kind == "" || actionReq.Namespace == "" || actionReq.Name == "" || actionReq.Action == "" {
		http.Error(w, "Missing required fields: kind, namespace, name, action", http.StatusBadRequest)
		return
	}

	// Validate action type
	validActions := map[string]bool{"reconcile": true, "suspend": true, "resume": true}
	if !validActions[actionReq.Action] {
		http.Error(w, "Invalid action. Must be one of: reconcile, suspend, resume", http.StatusBadRequest)
		return
	}

	// Find the FluxKindInfo for validation
	kindInfo, err := findFluxKindInfo(actionReq.Kind)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown resource kind: %s", actionReq.Kind), http.StatusBadRequest)
		return
	}

	// Check if the kind supports reconciliation (only for reconcile action)
	if actionReq.Action == "reconcile" && !kindInfo.Reconcilable {
		http.Error(w, fmt.Sprintf("Resource kind %s does not support reconciliation", kindInfo.Name), http.StatusBadRequest)
		return
	}

	// Get the preferred GVK for the kind
	gvk, err := h.preferredFluxGVK(req.Context(), kindInfo.Name)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get GVK for kind", "kind", kindInfo.Name)
		http.Error(w, fmt.Sprintf("Unable to get resource type for kind %s", kindInfo.Name), http.StatusInternalServerError)
		return
	}

	ctx := req.Context()
	now := metav1.Now().Format(time.RFC3339Nano)

	var actionErr error
	var message string

	switch actionReq.Action {
	case "reconcile":
		annotations := map[string]string{
			meta.ReconcileRequestAnnotation: now,
		}
		// Add force annotation for HelmRelease and ResourceSetInputProvider
		if kindInfo.Name == fluxcdv1.FluxHelmReleaseKind || kindInfo.Name == fluxcdv1.ResourceSetInputProviderKind {
			annotations[meta.ForceRequestAnnotation] = now
		}
		actionErr = h.annotateResource(ctx, *gvk, actionReq.Name, actionReq.Namespace, annotations)
		message = fmt.Sprintf("Reconciliation triggered for %s/%s", actionReq.Namespace, actionReq.Name)

	case "suspend":
		actionErr = h.setSuspension(ctx, *gvk, actionReq.Name, actionReq.Namespace, now, true)
		message = fmt.Sprintf("Suspended %s/%s", actionReq.Namespace, actionReq.Name)

	case "resume":
		actionErr = h.setSuspension(ctx, *gvk, actionReq.Name, actionReq.Namespace, now, false)
		message = fmt.Sprintf("Resumed %s/%s", actionReq.Namespace, actionReq.Name)
	}

	if actionErr != nil {
		log.FromContext(ctx).Error(actionErr, "action failed",
			"action", actionReq.Action,
			"kind", kindInfo.Name,
			"name", actionReq.Name,
			"namespace", actionReq.Namespace)

		switch {
		case errors.IsNotFound(actionErr):
			http.Error(w, fmt.Sprintf("Resource %s/%s not found", actionReq.Namespace, actionReq.Name), http.StatusNotFound)
		case errors.IsForbidden(actionErr):
			perms := user.Permissions(ctx)
			http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to %s %s/%s",
				perms.Username, actionReq.Action, actionReq.Namespace, actionReq.Name), http.StatusForbidden)
		default:
			http.Error(w, fmt.Sprintf("Action failed: %v", actionErr), http.StatusInternalServerError)
		}
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	resp := ActionResponse{
		Success: true,
		Message: message,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// annotateResource annotates a resource with the provided map of annotations.
func (h *Handler) annotateResource(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, annotations map[string]string) error {
	kubeClient := h.kubeClient.GetClient(ctx)

	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err := kubeClient.Get(ctx, objectKey, resource); err != nil {
			return err
		}

		patch := client.MergeFrom(resource.DeepCopy())

		existingAnnotations := resource.GetAnnotations()
		if existingAnnotations == nil {
			existingAnnotations = make(map[string]string)
		}
		maps.Copy(existingAnnotations, annotations)
		resource.SetAnnotations(existingAnnotations)

		if err := kubeClient.Patch(ctx, resource, patch); err != nil {
			return err
		}
		return nil
	})
}

// setSuspension sets the suspension state of a Flux resource.
// For Flux Operator resources, it uses the reconcile annotation.
// For Flux resources, it patches the spec.suspend field.
func (h *Handler) setSuspension(ctx context.Context, gvk schema.GroupVersionKind, name, namespace, requestTime string, suspend bool) error {
	kubeClient := h.kubeClient.GetClient(ctx)

	// Handle Flux Operator resources using annotations.
	if gvk.GroupVersion() == fluxcdv1.GroupVersion {
		var annotations map[string]string
		if suspend {
			annotations = map[string]string{
				fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
			}
		} else {
			annotations = map[string]string{
				fluxcdv1.ReconcileAnnotation:    fluxcdv1.EnabledValue,
				meta.ReconcileRequestAnnotation: requestTime,
			}
		}

		return h.annotateResource(ctx, gvk, name, namespace, annotations)
	}

	// Handle Flux resources by patching the spec.suspend field.
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil {
			return fmt.Errorf("unable to read %s/%s/%s: %w", gvk.Kind, namespace, name, err)
		}

		patch := client.MergeFrom(resource.DeepCopy())

		if suspend {
			if err := unstructured.SetNestedField(resource.Object, suspend, "spec", "suspend"); err != nil {
				return fmt.Errorf("unable to set suspend field: %w", err)
			}
		} else {
			unstructured.RemoveNestedField(resource.Object, "spec", "suspend")

			annotations := resource.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			maps.Copy(annotations, map[string]string{
				meta.ReconcileRequestAnnotation: requestTime,
			})
			resource.SetAnnotations(annotations)
		}

		if err := kubeClient.Patch(ctx, resource, patch); err != nil {
			return err
		}

		return nil
	})
}

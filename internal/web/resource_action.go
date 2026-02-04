// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// ActionRequest represents the request body for POST /api/v1/resource/action.
type ActionRequest struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Action    string `json:"action"`
}

// ActionResponse represents the response body for POST /api/v1/resource/action.
type ActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ActionHandler handles POST /api/v1/resource/action requests to perform actions on Flux resources.
func (h *Handler) ActionHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if actions are enabled.
	if !h.conf.UserActionsEnabled() {
		http.Error(w, "User actions are disabled", http.StatusMethodNotAllowed)
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
	if !slices.Contains(fluxcdv1.AllUserActions, actionReq.Action) {
		http.Error(w, "Invalid action. Must be one of: reconcile, suspend, resume", http.StatusBadRequest)
		return
	}

	// Find the FluxKindInfo for validation
	kindInfo, err := fluxcdv1.FindFluxKindInfo(actionReq.Kind)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown resource kind: %s", actionReq.Kind), http.StatusBadRequest)
		return
	}

	// Check if the kind supports reconciliation (only for reconcile action)
	if actionReq.Action == fluxcdv1.UserActionReconcile && !kindInfo.Reconcilable {
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

	// Check custom RBAC.
	if allowed, err := h.kubeClient.CanActOnResource(ctx,
		actionReq.Action, gvk.Group, kindInfo.Plural, actionReq.Namespace, actionReq.Name); err != nil {
		log.FromContext(req.Context()).Error(err, "failed to check custom RBAC for action",
			"action", actionReq.Action, "kind", actionReq.Kind, "name", actionReq.Name, "namespace", actionReq.Namespace)
		http.Error(w, "Unable to verify permissions", http.StatusInternalServerError)
		return
	} else if !allowed {
		perms := user.Permissions(req.Context())
		http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to %s %s/%s/%s",
			perms.Username, actionReq.Action, actionReq.Kind, actionReq.Namespace, actionReq.Name), http.StatusForbidden)
		return
	}

	var actionErr error
	var message string

	var obj client.Object
	switch actionReq.Action {
	case fluxcdv1.UserActionReconcile:
		annotations := map[string]string{
			meta.ReconcileRequestAnnotation: now,
		}
		// Add force annotation for HelmRelease and ResourceSetInputProvider
		if kindInfo.Name == fluxcdv1.FluxHelmReleaseKind || kindInfo.Name == fluxcdv1.ResourceSetInputProviderKind {
			annotations[meta.ForceRequestAnnotation] = now
		}
		obj, actionErr = h.annotateResource(ctx, *gvk, actionReq.Name, actionReq.Namespace, annotations)
		message = fmt.Sprintf("Reconciliation triggered for %s/%s", actionReq.Namespace, actionReq.Name)

	case fluxcdv1.UserActionSuspend:
		obj, actionErr = h.setSuspension(ctx, *gvk, actionReq.Name, actionReq.Namespace, now, true)
		message = fmt.Sprintf("Suspended %s/%s", actionReq.Namespace, actionReq.Name)

	case fluxcdv1.UserActionResume:
		obj, actionErr = h.setSuspension(ctx, *gvk, actionReq.Name, actionReq.Namespace, now, false)
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

	// Send audit event.
	if h.eventRecorder != nil && slices.Contains(h.conf.UserActions.Audit, actionReq.Action) {
		const reason = "WebAction"

		// Get a privileged kube client for the notifier to ensure it can fetch the FluxInstance.
		// We need the FluxInstance to know the notification-controller address.
		kubeClient := h.kubeClient.GetClient(req.Context(), kubeclient.WithPrivileges())

		// Build annotations.
		perms := user.Permissions(req.Context())
		token := fmt.Sprintf("%s/%s", obj.GetObjectKind().GroupVersionKind().Group, eventv1.MetaTokenKey)
		annotations := map[string]string{
			eventv1.Group + "/action":   actionReq.Action,
			eventv1.Group + "/username": perms.Username,
			eventv1.Group + "/groups":   strings.Join(perms.Groups, ", "),
			token:                       uuid.NewString(), // Forces unique events (this is an audit feature).
		}

		// Send the event.
		notifier.
			New(req.Context(), h.eventRecorder,
				h.kubeClient.GetScheme(), notifier.WithClient(kubeClient)).
			AnnotatedEventf(obj, annotations, corev1.EventTypeNormal, reason,
				"User '%s' performed action '%s' on the web UI", perms.Username, actionReq.Action)
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
func (h *Handler) annotateResource(ctx context.Context, gvk schema.GroupVersionKind,
	name, namespace string, annotations map[string]string) (client.Object, error) {
	kubeClient := h.kubeClient.GetClient(ctx)

	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
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
	if err != nil {
		return nil, err
	}

	return resource, nil
}

// setSuspension sets the suspension state of a Flux resource.
// For Flux Operator resources, it uses the reconcile annotation.
// For Flux resources, it patches the spec.suspend field.
// When suspending, it sets the SuspendedBy annotation to track the user who performed the action.
// When resuming, it removes the SuspendedBy annotation if present.
func (h *Handler) setSuspension(ctx context.Context, gvk schema.GroupVersionKind,
	name, namespace, requestTime string, suspend bool) (client.Object, error) {
	kubeClient := h.kubeClient.GetClient(ctx)

	// Handle Flux Operator resources using annotations.
	if gvk.GroupVersion() == fluxcdv1.GroupVersion {
		resource := &metav1.PartialObjectMetadata{}
		resource.SetGroupVersionKind(gvk)
		resource.SetName(name)
		resource.SetNamespace(namespace)

		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, resource); err != nil {
				return err
			}

			// Check current state before creating the patch.
			annotations := resource.GetAnnotations()
			if suspend {
				// Skip if already suspended to preserve the original SuspendedBy annotation.
				if annotations[fluxcdv1.ReconcileAnnotation] == fluxcdv1.DisabledValue {
					return nil
				}
			}

			patch := client.MergeFrom(resource.DeepCopy())

			if annotations == nil {
				annotations = make(map[string]string)
			}
			if suspend {
				annotations[fluxcdv1.ReconcileAnnotation] = fluxcdv1.DisabledValue
				annotations[fluxcdv1.SuspendedByAnnotation] = user.Username(ctx)
			} else {
				annotations[fluxcdv1.ReconcileAnnotation] = fluxcdv1.EnabledValue
				annotations[meta.ReconcileRequestAnnotation] = requestTime
				delete(annotations, fluxcdv1.SuspendedByAnnotation)
			}
			resource.SetAnnotations(annotations)

			return kubeClient.Patch(ctx, resource, patch)
		})
		if err != nil {
			return nil, err
		}
		return resource, nil
	}

	// Handle Flux resources by patching the spec.suspend field.
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil {
			return fmt.Errorf("unable to read %s/%s/%s: %w", gvk.Kind, namespace, name, err)
		}

		patch := client.MergeFrom(resource.DeepCopy())

		annotations := resource.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		if suspend {
			// Skip if already suspended to preserve the original SuspendedBy annotation.
			alreadySuspended, _, _ := unstructured.NestedBool(resource.Object, "spec", "suspend")
			if alreadySuspended {
				return nil
			}
			if err := unstructured.SetNestedField(resource.Object, suspend, "spec", "suspend"); err != nil {
				return fmt.Errorf("unable to set suspend field: %w", err)
			}
			annotations[fluxcdv1.SuspendedByAnnotation] = user.Username(ctx)
		} else {
			unstructured.RemoveNestedField(resource.Object, "spec", "suspend")
			annotations[meta.ReconcileRequestAnnotation] = requestTime
			delete(annotations, fluxcdv1.SuspendedByAnnotation)
		}
		resource.SetAnnotations(annotations)

		if err := kubeClient.Patch(ctx, resource, patch); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resource, nil
}

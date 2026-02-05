// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// WorkloadActionRequest represents the request body for POST /api/v1/workload/action.
type WorkloadActionRequest struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Action    string `json:"action"`
}

// WorkloadActionResponse represents the response body for POST /api/v1/workload/action.
type WorkloadActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// workloadKindInfo holds the API group and resource plural for a workload kind.
type workloadKindInfo struct {
	group   string
	plural  string
	actions []string
}

// supportedWorkloadKinds maps workload kinds to their API group and supported actions.
var supportedWorkloadKinds = map[string]workloadKindInfo{
	"Deployment":  {group: "apps", plural: "deployments", actions: []string{fluxcdv1.UserActionRestart}},
	"StatefulSet": {group: "apps", plural: "statefulsets", actions: []string{fluxcdv1.UserActionRestart}},
	"DaemonSet":   {group: "apps", plural: "daemonsets", actions: []string{fluxcdv1.UserActionRestart}},
}

// WorkloadActionHandler handles POST /api/v1/workload/action requests to perform actions on workloads.
func (h *Handler) WorkloadActionHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if actions are enabled.
	if !h.conf.UserActionsEnabled() {
		http.Error(w, "User actions are disabled", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body.
	var actionReq WorkloadActionRequest
	if err := json.NewDecoder(req.Body).Decode(&actionReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields.
	if actionReq.Kind == "" || actionReq.Namespace == "" || actionReq.Name == "" || actionReq.Action == "" {
		http.Error(w, "Missing required fields: kind, namespace, name, action", http.StatusBadRequest)
		return
	}

	// Validate workload kind.
	kindInfo, ok := supportedWorkloadKinds[actionReq.Kind]
	if !ok {
		http.Error(w, fmt.Sprintf("Unsupported workload kind: %s. Supported kinds: Deployment, StatefulSet, DaemonSet", actionReq.Kind), http.StatusBadRequest)
		return
	}

	// Validate action for this kind.
	if !slices.Contains(kindInfo.actions, actionReq.Action) {
		http.Error(w, fmt.Sprintf("Action '%s' is not supported for kind '%s'", actionReq.Action, actionReq.Kind), http.StatusBadRequest)
		return
	}

	ctx := req.Context()

	// Check custom RBAC for the action.
	if allowed, err := h.kubeClient.CanActOnResource(ctx,
		actionReq.Action, kindInfo.group, kindInfo.plural, actionReq.Namespace, actionReq.Name); err != nil {
		log.FromContext(ctx).Error(err, "failed to check custom RBAC for workload action",
			"action", actionReq.Action, "kind", actionReq.Kind, "name", actionReq.Name, "namespace", actionReq.Namespace)
		http.Error(w, "Unable to verify permissions", http.StatusInternalServerError)
		return
	} else if !allowed {
		perms := user.Permissions(ctx)
		http.Error(w, fmt.Sprintf("Permission denied. User %s does not have access to %s %s/%s/%s",
			perms.Username, actionReq.Action, actionReq.Kind, actionReq.Namespace, actionReq.Name), http.StatusForbidden)
		return
	}

	// Fetch workload for audit if enabled.
	var workload *unstructured.Unstructured
	if h.isAuditEnabled(actionReq.Action) {
		workload = &unstructured.Unstructured{}
		workload.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: actionReq.Kind})
		if err := h.kubeClient.GetClient(ctx).Get(ctx, client.ObjectKey{
			Namespace: actionReq.Namespace, Name: actionReq.Name,
		}, workload); err != nil {
			http.Error(w, fmt.Sprintf("Failed to get %s/%s/%s",
				actionReq.Kind, actionReq.Namespace, actionReq.Name), http.StatusInternalServerError)
			return
		}
	}

	var actionErr error
	var message string

	switch actionReq.Action {
	case fluxcdv1.UserActionRestart:
		actionErr = h.restartWorkload(ctx, actionReq.Kind, actionReq.Namespace, actionReq.Name)
		message = fmt.Sprintf("Rollout restart triggered for %s/%s", actionReq.Namespace, actionReq.Name)
	default:
		http.Error(w, fmt.Sprintf("Unknown action: %s", actionReq.Action), http.StatusBadRequest)
		return
	}

	if actionErr != nil {
		log.FromContext(ctx).Error(actionErr, "workload action failed",
			"action", actionReq.Action,
			"kind", actionReq.Kind,
			"name", actionReq.Name,
			"namespace", actionReq.Namespace)

		switch {
		case errors.IsNotFound(actionErr):
			http.Error(w, fmt.Sprintf("Workload %s/%s not found", actionReq.Namespace, actionReq.Name), http.StatusNotFound)
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
	obj := &metav1.PartialObjectMetadata{}
	obj.SetNamespace(actionReq.Namespace)
	obj.SetName(actionReq.Name)
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   kindInfo.group,
		Version: "v1",
		Kind:    actionReq.Kind,
	})
	h.sendAuditEvent(req.Context(), actionReq.Action, obj, workload)

	// Return success response.
	w.Header().Set("Content-Type", "application/json")
	resp := WorkloadActionResponse{
		Success: true,
		Message: message,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// restartWorkload triggers a rollout restart by patching the pod template annotation
// using Server-Side Apply with the flux-operator-web field manager.
func (h *Handler) restartWorkload(ctx context.Context, kind, namespace, name string) error {
	kubeClient := h.kubeClient.GetClient(ctx)

	now := metav1.Now().Format(time.RFC3339Nano)

	// Build the patch object for Server-Side Apply.
	// This patches spec.template.metadata.annotations with the restart annotation.
	patch := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": now,
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	// Create the partial object metadata for patching.
	obj := &metav1.PartialObjectMetadata{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    kind,
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)

	// Apply the patch using Server-Side Apply with the Web UI field manager.
	err = kubeClient.Patch(ctx, obj, client.RawPatch(types.ApplyPatchType, patchBytes),
		client.ForceOwnership, client.FieldOwner(kubeclient.FieldOwner))
	if err != nil {
		return fmt.Errorf("failed to patch workload: %w", err)
	}

	return nil
}

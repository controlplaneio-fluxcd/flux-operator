// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// ObjectEditRequest represents the request body for PUT /api/v1/object.
type ObjectEditRequest struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
	YAML       string `json:"yaml"`
}

// ObjectEditResponse represents the response body for PUT /api/v1/object.
type ObjectEditResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ObjectEditHandler handles GET and PUT /api/v1/object requests for live Kubernetes objects.
func (h *Handler) ObjectEditHandler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet, http.MethodHead:
		h.ObjectGetHandler(w, req)
	case http.MethodPut:
		h.ObjectUpdateHandler(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ObjectGetHandler handles GET /api/v1/object requests and returns a live Kubernetes object.
func (h *Handler) ObjectGetHandler(w http.ResponseWriter, req *http.Request) {
	queryParams := req.URL.Query()
	apiVersion := queryParams.Get("apiVersion")
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")
	if apiVersion == "" || kind == "" || name == "" {
		http.Error(w, "Missing required parameters: apiVersion, kind, name", http.StatusBadRequest)
		return
	}

	obj, err := h.GetObject(req.Context(), apiVersion, kind, namespace, name)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get object")
		switch {
		case apierrors.IsNotFound(err):
			http.Error(w, err.Error(), http.StatusNotFound)
		case apierrors.IsForbidden(err):
			perms := user.Permissions(req.Context())
			http.Error(w, fmt.Sprintf("You do not have access to this object. User: %s, Groups: [%s]",
				perms.Username, strings.Join(perms.Groups, ", ")), http.StatusForbidden)
		default:
			http.Error(w, fmt.Sprintf("Failed to get object: %v", err), http.StatusInternalServerError)
		}
		return
	}

	obj.SetManagedFields(nil)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(obj.Object); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ObjectUpdateHandler handles PUT /api/v1/object requests to update live Kubernetes objects from YAML.
func (h *Handler) ObjectUpdateHandler(w http.ResponseWriter, req *http.Request) {
	var editReq ObjectEditRequest
	if err := json.NewDecoder(req.Body).Decode(&editReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(editReq.YAML) == "" {
		http.Error(w, "Missing required field: yaml", http.StatusBadRequest)
		return
	}

	updated, err := h.UpdateObjectFromYAML(req.Context(), editReq)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "object edit failed")
		switch {
		case isObjectValidationError(err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case apierrors.IsNotFound(err):
			http.Error(w, err.Error(), http.StatusNotFound)
		case apierrors.IsForbidden(err):
			perms := user.Permissions(req.Context())
			http.Error(w, fmt.Sprintf("Permission denied. User %s cannot update %s %s/%s",
				perms.Username, updatedKindFromErrorTarget(updated), updated.GetNamespace(), updated.GetName()), http.StatusForbidden)
		case apierrors.IsConflict(err):
			http.Error(w, err.Error(), http.StatusConflict)
		case apierrors.IsInvalid(err) || apierrors.IsBadRequest(err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, fmt.Sprintf("Object update failed: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := ObjectEditResponse{
		Success: true,
		Message: fmt.Sprintf("Updated %s %s/%s", updated.GetKind(), updated.GetNamespace(), updated.GetName()),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetObject fetches a single Kubernetes object by apiVersion, kind, name and namespace.
func (h *Handler) GetObject(ctx context.Context, apiVersion, kind, namespace, name string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	if err := h.kubeClient.GetClient(ctx).Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// UpdateObjectFromYAML updates an existing Kubernetes object from a user-edited YAML document.
func (h *Handler) UpdateObjectFromYAML(ctx context.Context, req ObjectEditRequest) (*unstructured.Unstructured, error) {
	edited := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(req.YAML), &edited.Object); err != nil {
		return edited, objectValidationError(fmt.Sprintf("Invalid YAML: %v", err))
	}

	if edited.GetAPIVersion() == "" {
		return edited, objectValidationError("apiVersion is required")
	}
	if edited.GetKind() == "" {
		return edited, objectValidationError("kind is required")
	}
	if edited.GetName() == "" {
		return edited, objectValidationError("metadata.name is required")
	}
	if err := validateObjectIdentity(req, edited); err != nil {
		return edited, err
	}

	kubeClient := h.kubeClient.GetClient(ctx)
	current := &unstructured.Unstructured{}
	current.SetAPIVersion(edited.GetAPIVersion())
	current.SetKind(edited.GetKind())
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: edited.GetNamespace(), Name: edited.GetName()}, current); err != nil {
		return edited, err
	}

	updated := mergeEditedObject(current, edited)
	if err := kubeClient.Update(ctx, updated); err != nil {
		return updated, err
	}

	return updated, nil
}

func validateObjectIdentity(req ObjectEditRequest, edited *unstructured.Unstructured) error {
	if req.APIVersion != "" && req.APIVersion != edited.GetAPIVersion() {
		return objectValidationError("apiVersion cannot be changed")
	}
	if req.Kind != "" && req.Kind != edited.GetKind() {
		return objectValidationError("kind cannot be changed")
	}
	if req.Namespace != "" && req.Namespace != edited.GetNamespace() {
		return objectValidationError("metadata.namespace cannot be changed")
	}
	if req.Name != "" && req.Name != edited.GetName() {
		return objectValidationError("metadata.name cannot be changed")
	}
	return nil
}

func mergeEditedObject(current, edited *unstructured.Unstructured) *unstructured.Unstructured {
	updated := current.DeepCopy()
	updated.SetLabels(edited.GetLabels())
	updated.SetAnnotations(edited.GetAnnotations())
	updated.SetFinalizers(edited.GetFinalizers())
	updated.SetOwnerReferences(edited.GetOwnerReferences())

	for key, value := range edited.Object {
		switch key {
		case "apiVersion", "kind", "metadata", "status":
			continue
		default:
			updated.Object[key] = value
		}
	}
	return updated
}

type objectValidationError string

func (e objectValidationError) Error() string {
	return string(e)
}

func isObjectValidationError(err error) bool {
	_, ok := err.(objectValidationError)
	return ok
}

func updatedKindFromErrorTarget(obj *unstructured.Unstructured) string {
	if obj == nil || obj.GetKind() == "" {
		return "object"
	}
	return obj.GetKind()
}

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

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// WorkloadHandler handles GET /api/v1/workload requests and returns a Kubernetes workload by kind, name and namespace.
// Query parameters: kind, name, namespace (all required).
// Supported workload kinds: Deployment, StatefulSet, DaemonSet.
// Example: /api/v1/workload?kind=Deployment&name=flux-operator&namespace=flux-system
func (r *Router) WorkloadHandler(w http.ResponseWriter, req *http.Request) {
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
	resource, err := r.GetWorkloadStatus(req.Context(), kind, name, namespace)
	if err != nil {
		r.log.Error(err, "failed to get workload", "url", req.URL.String(),
			"kind", kind, "name", name, "namespace", namespace)
		switch {
		case errors.IsNotFound(err):
			// return empty response if resource not found
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case errors.IsForbidden(err):
			perms := user.Permissions(req.Context())
			msg := fmt.Sprintf("You do not have access to this workload or for listing its pods. "+
				"Contact your administrator if you believe this is an error. "+
				"User: %s, Groups: [%s]",
				perms.Username, strings.Join(perms.Groups, ", "))
			http.Error(w, msg, http.StatusForbidden)
		default:
			http.Error(w, fmt.Sprintf("Failed to get workload: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Encode and send the response
	if err := json.NewEncoder(w).Encode(resource); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// WorkloadStatus represents the rollout status of a Kubernetes workload.
type WorkloadStatus struct {
	// Kind is the kind of the workload.
	Kind string `json:"kind"`

	// Name is the name of the workload.
	Name string `json:"name"`

	// Namespace is the namespace of the workload.
	Namespace string `json:"namespace"`

	// Status is the readiness status of the workload.
	// kstatus values: "InProgress", "Failed", "Current", "Terminating", "NotFound", "Unknown"
	Status string `json:"status"`

	// StatusMessage is a human-readable message indicating details
	// about the workload observed state.
	StatusMessage string `json:"statusMessage,omitempty"`

	// ContainerImages is the list of container images used by the workload.
	ContainerImages []string `json:"containerImages,omitempty"`

	// Pods is the list of pods managed by the workload.
	Pods []WorkloadPodStatus `json:"pods,omitempty"`
}

// WorkloadPodStatus represents the status of a pod managed by a workload.
type WorkloadPodStatus struct {
	// Name is the name of the pod.
	Name string `json:"name"`

	// Status is the readiness status of the pod.
	// kstatus values: "InProgress", "Failed", "Current", "Terminating", "NotFound", "Unknown"
	Status string `json:"status"`

	// StatusMessage is a human-readable message indicating details
	// about the pod observed state.
	StatusMessage string `json:"statusMessage,omitempty"`

	// Timestamp is the creation timestamp of the pod.
	Timestamp string `json:"timestamp"`
}

// GetWorkloadStatus should fetch the Deployment/StatefulSet/DaemonSet and return the WorkloadStatus.
func (r *Router) GetWorkloadStatus(ctx context.Context, kind, name, namespace string) (*WorkloadStatus, error) {
	// Create an unstructured object to fetch the resource
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    kind,
	})

	// Create the object key
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	// Fetch the resource from the cluster
	if err := r.kubeClient.GetClient(ctx).Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("unable to get resource %s/%s in namespace %s: %w", kind, name, namespace, err)
	}

	workload := &WorkloadStatus{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}

	// Extract container images
	workload.ContainerImages = extractContainerImages(obj)

	// Compute the workload status using kstatus
	res, err := status.Compute(obj)
	if err != nil {
		workload.Status = string(status.UnknownStatus)
		workload.StatusMessage = fmt.Sprintf("Failed to compute status: %s", err.Error())
		return workload, nil
	}
	workload.Status = string(res.Status)
	workload.StatusMessage = res.Message

	// Get the pods managed by the workload
	podsStatus, err := r.GetWorkloadPods(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get pods for workload %s/%s: %w", namespace, name, err)
	}
	workload.Pods = podsStatus

	return workload, nil
}

func (r *Router) GetWorkloadPods(ctx context.Context, obj *unstructured.Unstructured) ([]WorkloadPodStatus, error) {
	podList := &corev1.PodList{}

	selector, found, err := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	if err != nil || !found {
		return nil, nil
	}

	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels(selector),
	}

	if err := r.kubeClient.GetClient(ctx).List(ctx, podList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list pods for workload %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}

	podsStatus := make([]WorkloadPodStatus, 0, len(podList.Items))
	for _, pod := range podList.Items {
		// check pod owner references to ensure it's managed by the workload
		isManaged := false
		for _, ownerRef := range pod.OwnerReferences {
			if ownerRef.APIVersion == "apps/v1" && strings.HasPrefix(ownerRef.Name, obj.GetName()) {
				isManaged = true
				break
			}
		}
		if !isManaged {
			continue
		}
		// convert pod to unstructured for kstatus computation
		rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		if err != nil {
			podsStatus = append(podsStatus, WorkloadPodStatus{
				Name:          pod.GetName(),
				Status:        string(status.UnknownStatus),
				StatusMessage: fmt.Sprintf("Failed to convert pod to unstructured: %s", err.Error()),
			})
			continue
		}
		unstructuredPod := unstructured.Unstructured{Object: rawMap}

		res, err := status.Compute(&unstructuredPod)
		if err != nil {
			podsStatus = append(podsStatus, WorkloadPodStatus{
				Name:          pod.GetName(),
				Status:        string(status.UnknownStatus),
				StatusMessage: fmt.Sprintf("Failed to compute status: %s", err.Error()),
			})
			continue
		}
		if res.Status != status.CurrentStatus {
			// get containerStatuses reason
			var reasons []string
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Waiting != nil {
					reasons = append(reasons, cs.State.Waiting.Reason)
				} else if cs.State.Terminated != nil {
					reasons = append(reasons, cs.State.Terminated.Reason)
				}
			}
			if len(reasons) > 0 {
				res.Message = fmt.Sprintf("%s. Reason: %s", res.Message, strings.Join(reasons, ", "))
			}
		}

		podsStatus = append(podsStatus, WorkloadPodStatus{
			Name:          pod.GetName(),
			Status:        string(res.Status),
			StatusMessage: res.Message,
			Timestamp:     pod.GetCreationTimestamp().Format(time.RFC3339),
		})
	}

	// Sort pods by name
	slices.SortStableFunc(podsStatus, func(i, j WorkloadPodStatus) int {
		return strings.Compare(i.Name, j.Name)
	})

	return podsStatus, nil
}

// extractContainerImages extracts container images from the given unstructured object.
func extractContainerImages(obj *unstructured.Unstructured) []string {
	containerImages := make([]string, 0)
	if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers"); found {
		for _, container := range containers {
			if image, found, _ := unstructured.NestedString(container.(map[string]any), "image"); found {
				if !slices.Contains(containerImages, image) {
					containerImages = append(containerImages, image)
				}
			}
		}
	}
	if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "initContainers"); found {
		for _, container := range containers {
			if image, found, _ := unstructured.NestedString(container.(map[string]any), "image"); found {
				if !slices.Contains(containerImages, image) {
					containerImages = append(containerImages, image)
				}
			}
		}
	}

	// Sort container images
	slices.Sort(containerImages)

	return containerImages
}

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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	workloadKindDeployment  = "Deployment"
	workloadKindStatefulSet = "StatefulSet"
	workloadKindDaemonSet   = "DaemonSet"
	workloadKindCronJob     = "CronJob"
	workloadKindPod         = "Pod"
)

// JobOwnerCronJobField is the field index key for querying Jobs by their CronJob owner.
// This index must be set up when creating the controller-runtime manager.
const JobOwnerCronJobField = ".metadata.ownerReferences.cronJob"

// WorkloadHandler handles GET /api/v1/workload requests and returns a Kubernetes workload by kind, name and namespace.
// Query parameters: kind, name, namespace (all required).
// Supported workload kinds: CronJob, DaemonSet, Deployment, StatefulSet.
// Example: /api/v1/workload?kind=Deployment&name=flux-operator&namespace=flux-system
func (h *Handler) WorkloadHandler(w http.ResponseWriter, req *http.Request) {
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
	resource, err := h.GetWorkloadStatus(req.Context(), kind, name, namespace)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get workload")
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

	// CreatedAt is the creation timestamp of the workload.
	CreatedAt time.Time `json:"createdAt"`

	// RestartedAt is the timestamp of the last rollout restart.
	// Extracted from the kubectl.kubernetes.io/restartedAt annotation.
	RestartedAt string `json:"restartedAt,omitempty"`

	// ContainerImages is the list of container images used by the workload.
	ContainerImages []string `json:"containerImages,omitempty"`

	// Pods is the list of pods managed by the workload.
	Pods []WorkloadPodStatus `json:"pods,omitempty"`

	// UserActions indicates which actions the user can perform on this workload.
	UserActions []string `json:"userActions,omitempty"`
}

// WorkloadPodStatus represents the status of a pod managed by a workload.
type WorkloadPodStatus struct {
	// Name is the name of the pod.
	Name string `json:"name"`

	// Status is the Kubernetes pod phase.
	// Values: "Pending", "Running", "Succeeded", "Failed", "Unknown"
	Status string `json:"status"`

	// StatusMessage is a human-readable message indicating details
	// about the pod observed state.
	StatusMessage string `json:"statusMessage,omitempty"`

	// CreatedAt is the creation timestamp of the pod.
	CreatedAt time.Time `json:"createdAt"`

	// CreatedBy is the user who triggered the pod creation.
	CreatedBy string `json:"createdBy,omitempty"`
}

// getWorkloadGVK returns the GroupVersionKind for a given workload kind.
// CronJob uses batch/v1, while Deployment, StatefulSet, and DaemonSet use apps/v1.
func getWorkloadGVK(kind string) schema.GroupVersionKind {
	if kind == workloadKindCronJob {
		return schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: kind}
	}
	return schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: kind}
}

// GetWorkloadStatus should fetch the Deployment/StatefulSet/DaemonSet/CronJob and return the WorkloadStatus.
func (h *Handler) GetWorkloadStatus(ctx context.Context, kind, name, namespace string) (*WorkloadStatus, error) {
	// Create an unstructured object to fetch the resource
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(getWorkloadGVK(kind))

	// Create the object key
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	// Fetch the resource from the cluster
	if err := h.kubeClient.GetClient(ctx).Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("unable to get resource %s/%s in namespace %s: %w", kind, name, namespace, err)
	}

	workload := &WorkloadStatus{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}

	// Extract creation timestamp
	creationTimestamp := obj.GetCreationTimestamp()
	if !creationTimestamp.IsZero() {
		workload.CreatedAt = creationTimestamp.Time
	}

	// Extract restartedAt annotation from pod template
	workload.RestartedAt = extractRestartedAt(obj)

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

	// For CronJob, override status based on suspend state and active jobs
	if kind == workloadKindCronJob {
		workload.Status, workload.StatusMessage = getCronJobWorkloadStatus(obj, res.Status, res.Message)
	}

	// For apps/v1 workloads, show replicas count when ready
	if kind != workloadKindCronJob && res.Status == status.CurrentStatus {
		workload.StatusMessage = getAppsWorkloadStatusMessage(obj, kind)
	}

	// Get the pods managed by the workload
	podsStatus, err := h.GetWorkloadPods(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get pods for workload %s/%s: %w", namespace, name, err)
	}
	workload.Pods = podsStatus

	// Check which actions the user can perform on this workload
	if h.conf.UserActionsEnabled() {
		kindInfo, ok := supportedWorkloadKinds[kind]
		if ok {
			for _, action := range kindInfo.actions {
				canAct, err := h.kubeClient.CanActOnResource(ctx, action, kindInfo.group, kindInfo.plural, namespace, name)
				if err != nil {
					log.FromContext(ctx).Error(err, "failed to check RBAC for action",
						"action", action,
						"kind", kind,
						"name", name,
						"namespace", namespace)
					continue
				}
				if canAct {
					workload.UserActions = append(workload.UserActions, action)
				}
			}
		}

		// Check if the user can delete pods in this namespace
		canDelete, err := h.kubeClient.CanActOnResource(ctx, fluxcdv1.UserActionDelete, "", "pods", namespace, "")
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to check delete pod RBAC",
				"namespace", namespace)
		} else if canDelete {
			workload.UserActions = append(workload.UserActions, fluxcdv1.UserActionDeletePods)
		}
	}

	return workload, nil
}

// GetWorkloadPods returns the pods managed by a workload (Deployment, StatefulSet, DaemonSet, or CronJob).
// For apps/v1 workloads, it uses the selector labels to find pods.
// For CronJobs, it delegates to getCronJobPods which traverses the CronJob -> Job -> Pod ownership chain.
func (h *Handler) GetWorkloadPods(ctx context.Context, obj *unstructured.Unstructured) ([]WorkloadPodStatus, error) {
	podList := &corev1.PodList{}

	// For CronJob, we need to find Jobs owned by the CronJob, then Pods owned by those Jobs
	if obj.GetKind() == workloadKindCronJob {
		return h.getCronJobPods(ctx, obj)
	}

	selector, found, err := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	if err != nil || !found {
		return nil, nil
	}

	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels(selector),
	}

	if err := h.kubeClient.GetClient(ctx).List(ctx, podList, listOpts...); err != nil {
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

		// Use Kubernetes pod phase as the status
		podStatus := string(pod.Status.Phase)
		podMessage := getPodStatusMessage(&pod)

		podsStatus = append(podsStatus, WorkloadPodStatus{
			Name:          pod.GetName(),
			Status:        podStatus,
			StatusMessage: podMessage,
			CreatedAt:     pod.GetCreationTimestamp().Time,
		})
	}

	// Sort pods by name
	slices.SortStableFunc(podsStatus, func(i, j WorkloadPodStatus) int {
		return strings.Compare(i.Name, j.Name)
	})

	return podsStatus, nil
}

// getCronJobPods returns the pods managed by a CronJob.
// CronJob ownership is cascading: CronJob -> Job -> Pod.
func (h *Handler) getCronJobPods(ctx context.Context, cronJob *unstructured.Unstructured) ([]WorkloadPodStatus, error) {
	// Query Jobs owned by this CronJob using the field index on the cluster's cached client.
	// We use WithPrivileges() because the user already has access to the CronJob,
	// and the index is only available on the privileged cache. This effectively results
	// in escalating read access on CronJobs to status-only read access on Pods created
	// by the CronJob, which we consider safe.
	jobList := &batchv1.JobList{}
	listOpts := []client.ListOption{
		client.InNamespace(cronJob.GetNamespace()),
		client.MatchingFields{JobOwnerCronJobField: cronJob.GetName()},
	}
	if err := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges()).List(ctx, jobList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list jobs for CronJob %s/%s: %w", cronJob.GetNamespace(), cronJob.GetName(), err)
	}

	if len(jobList.Items) == 0 {
		return nil, nil
	}

	// Collect job names for the pod label selector
	jobNameValues := make([]string, 0, len(jobList.Items))
	for _, job := range jobList.Items {
		jobNameValues = append(jobNameValues, job.Name)
	}

	// List only Pods that belong to the owned Jobs using the job-name label selector.
	// The Job controller adds a "job-name" label to all Pods it creates.
	jobNameReq, err := labels.NewRequirement("job-name", selection.In, jobNameValues)
	if err != nil {
		return nil, fmt.Errorf("failed to create label selector for CronJob %s/%s: %w", cronJob.GetNamespace(), cronJob.GetName(), err)
	}
	podList := &corev1.PodList{}
	podListOpts := []client.ListOption{
		client.InNamespace(cronJob.GetNamespace()),
		client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*jobNameReq)},
	}
	if err := h.kubeClient.GetClient(ctx).List(ctx, podList, podListOpts...); err != nil {
		return nil, fmt.Errorf("failed to list pods for CronJob %s/%s: %w", cronJob.GetNamespace(), cronJob.GetName(), err)
	}

	podsStatus := make([]WorkloadPodStatus, 0, len(podList.Items))
	for _, pod := range podList.Items {
		// Use Kubernetes pod phase as the status
		podStatus := string(pod.Status.Phase)
		podMessage := getPodStatusMessage(&pod)

		podsStatus = append(podsStatus, WorkloadPodStatus{
			Name:          pod.GetName(),
			Status:        podStatus,
			StatusMessage: podMessage,
			CreatedAt:     pod.GetCreationTimestamp().Time,
			CreatedBy:     pod.GetAnnotations()[fluxcdv1.CreatedByAnnotation],
		})
	}

	// Sort pods by name
	slices.SortStableFunc(podsStatus, func(i, j WorkloadPodStatus) int {
		return strings.Compare(i.Name, j.Name)
	})

	return podsStatus, nil
}

// extractContainerImages extracts container images from the given unstructured object.
// For CronJob, the path is spec.jobTemplate.spec.template.spec.containers.
// For Deployment/StatefulSet/DaemonSet, the path is spec.template.spec.containers.
func extractContainerImages(obj *unstructured.Unstructured) []string {
	containerImages := make([]string, 0)

	// Determine the path based on kind
	var containerPath, initContainerPath []string
	if obj.GetKind() == workloadKindCronJob {
		containerPath = []string{"spec", "jobTemplate", "spec", "template", "spec", "containers"}
		initContainerPath = []string{"spec", "jobTemplate", "spec", "template", "spec", "initContainers"}
	} else {
		containerPath = []string{"spec", "template", "spec", "containers"}
		initContainerPath = []string{"spec", "template", "spec", "initContainers"}
	}

	if containers, found, _ := unstructured.NestedSlice(obj.Object, containerPath...); found {
		for _, container := range containers {
			if image, found, _ := unstructured.NestedString(container.(map[string]any), "image"); found {
				if !slices.Contains(containerImages, image) {
					containerImages = append(containerImages, image)
				}
			}
		}
	}
	if containers, found, _ := unstructured.NestedSlice(obj.Object, initContainerPath...); found {
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

// extractRestartedAt extracts the kubectl.kubernetes.io/restartedAt annotation from the pod template.
// For CronJob, the path is spec.jobTemplate.spec.template.metadata.annotations.
// For Deployment/StatefulSet/DaemonSet, the path is spec.template.metadata.annotations.
func extractRestartedAt(obj *unstructured.Unstructured) string {
	var annotationsPath []string
	if obj.GetKind() == workloadKindCronJob {
		annotationsPath = []string{"spec", "jobTemplate", "spec", "template", "metadata", "annotations"}
	} else {
		annotationsPath = []string{"spec", "template", "metadata", "annotations"}
	}

	annotations, found, _ := unstructured.NestedStringMap(obj.Object, annotationsPath...)
	if !found {
		return ""
	}

	return annotations["kubectl.kubernetes.io/restartedAt"]
}

// getCronJobWorkloadStatus returns the status and message for a CronJob workload.
// It checks if the CronJob is suspended, has active jobs, or is idle.
func getCronJobWorkloadStatus(obj *unstructured.Unstructured, kstatus status.Status, kstatusMessage string) (string, string) {
	suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	if suspended {
		return "Suspended", "CronJob is suspended"
	}

	activeJobs, _, _ := unstructured.NestedSlice(obj.Object, "status", "active")
	if len(activeJobs) > 0 {
		return "Progressing", fmt.Sprintf("Active jobs: %d", len(activeJobs))
	}

	if kstatus == status.CurrentStatus {
		schedule, _, _ := unstructured.NestedString(obj.Object, "spec", "schedule")
		if schedule == "" {
			return "Idle", "CronJob is ready"
		}
		return "Idle", schedule
	}

	return string(kstatus), kstatusMessage
}

// getAppsWorkloadStatusMessage returns a status message showing replicas for apps/v1 workloads.
func getAppsWorkloadStatusMessage(obj *unstructured.Unstructured, kind string) string {
	var replicas int64
	var found bool

	switch kind {
	case workloadKindDeployment, workloadKindStatefulSet:
		replicas, found, _ = unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	case workloadKindDaemonSet:
		replicas, found, _ = unstructured.NestedInt64(obj.Object, "status", "numberReady")
	}

	if !found {
		replicas = 0
	}

	return fmt.Sprintf("Replicas: %d", replicas)
}

// getPodStatusMessage returns a human-readable message based on the pod phase and container statuses.
func getPodStatusMessage(pod *corev1.Pod) string {
	switch pod.Status.Phase {
	case corev1.PodPending:
		// Check for waiting reasons in container statuses
		var reasons []string
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				reasons = append(reasons, cs.State.Waiting.Reason)
			}
		}
		if len(reasons) > 0 {
			return fmt.Sprintf("Waiting: %s", strings.Join(reasons, ", "))
		}
		return "Pod is pending"

	case corev1.PodRunning:
		// Find when the pod started running
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return fmt.Sprintf("Started at %s", cond.LastTransitionTime.Format("2006-01-02 15:04:05 MST"))
			}
		}
		// Fallback to creation timestamp if Ready condition not found
		return fmt.Sprintf("Started at %s", pod.GetCreationTimestamp().Format("2006-01-02 15:04:05 MST"))

	case corev1.PodSucceeded:
		// Find the container termination time for the completion message
		var finishedAt time.Time
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil && !cs.State.Terminated.FinishedAt.IsZero() {
				finishedAt = cs.State.Terminated.FinishedAt.Time
				break
			}
		}
		if !finishedAt.IsZero() {
			return fmt.Sprintf("Completed at %s", finishedAt.Format("2006-01-02 15:04:05 MST"))
		}
		return "Pod completed successfully"

	case corev1.PodFailed:
		// Get the failure reason from container statuses
		var reasons []string
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
				reasons = append(reasons, cs.State.Terminated.Reason)
			}
		}
		if len(reasons) > 0 {
			return fmt.Sprintf("Reason: %s", strings.Join(reasons, ", "))
		}
		return "Pod failed"

	default:
		return "Unknown pod phase"
	}
}

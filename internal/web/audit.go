// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"slices"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// auditEventReason is the reason used for action audit events.
const auditEventReason = "WebAction"

// isAuditEnabled returns true if auditing is enabled for the given action.
func (h *Handler) isAuditEnabled(action string) bool {
	if h.eventRecorder == nil {
		return false
	}
	return slices.Contains(h.conf.UserActions.Audit, action) ||
		slices.Contains(h.conf.UserActions.Audit, "*")
}

// sendAuditEvent sends an audit event for a user action if auditing is enabled.
// If workload is non-nil, the audit event is associated with the managing Flux
// resource (extracted from the workload's labels) instead of obj.
func (h *Handler) sendAuditEvent(ctx context.Context, action string, obj client.Object, workload *unstructured.Unstructured) {
	if !h.isAuditEnabled(action) {
		return
	}

	// Use the privileged kube client to ensure we can fetch the workload reconciler
	// and the Flux instance to discover the notification-controller endpoint.
	kubeClient := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges())

	// Build the subject string before potentially fetching the workload reconciler.
	subject := fmt.Sprintf("%s/%s/%s",
		obj.GetObjectKind().GroupVersionKind().Kind,
		obj.GetNamespace(),
		obj.GetName())

	// Read the user permissions from the context.
	// This should always succeed since the audit event is only sent for authenticated users.
	perms := user.Permissions(ctx)

	// If workload is provided, extract the reconciler ref and fetch the Flux resource.
	// If the fetch fails, skip the audit event entirely and log the error.
	if workload != nil {
		if reconcilerRef := getReconcilerRef(workload); reconcilerRef != "" {
			fluxObj, err := h.fetchReconcilerRef(ctx, kubeClient, reconcilerRef)
			if err != nil {
				log.FromContext(ctx).Error(err, "skipping audit event, failed to fetch reconciler ref",
					"reconcilerRef", reconcilerRef,
					"subject", subject,
					"action", action,
					"user", perms.Username,
				)
				return
			}

			// Swap the object with the Flux resource managing it,
			// so the event is associated with the Flux resource instead of the workload.
			obj = fluxObj
		}
	}

	token := fmt.Sprintf("%s/%s", obj.GetObjectKind().GroupVersionKind().Group, eventv1.MetaTokenKey)
	annotations := map[string]string{
		eventv1.Group + "/action":   action,
		eventv1.Group + "/username": perms.Username,
		eventv1.Group + "/groups":   strings.Join(perms.Groups, ", "),
		token:                       uuid.NewString(), // Forces unique events (this is an audit feature).
	}

	if workload != nil {
		annotations[eventv1.Group+"/subject"] = subject
	}

	notifier.New(ctx, h.eventRecorder, h.kubeClient.GetScheme(), notifier.WithClient(kubeClient)).
		AnnotatedEventf(obj, annotations, corev1.EventTypeNormal, auditEventReason,
			"User '%s' performed action '%s' for '%s' on the web UI",
			perms.Username,
			action,
			subject,
		)
}

// fetchReconcilerRef parses a reconciler ref string (Kind/namespace/name)
// and fetches the corresponding Flux resource using the provided client.
func (h *Handler) fetchReconcilerRef(ctx context.Context, kubeClient client.Client, ref string) (client.Object, error) {
	parts := strings.Split(ref, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid reconciler ref: %s", ref)
	}
	kind, namespace, name := parts[0], parts[1], parts[2]

	gvk, err := h.preferredFluxGVK(ctx, kind)
	if err != nil {
		return nil, fmt.Errorf("unable to get GVK for kind %s: %w", kind, err)
	}

	obj := &metav1.PartialObjectMetadata{}
	obj.SetGroupVersionKind(*gvk)
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		return nil, fmt.Errorf("unable to fetch reconciler %s/%s/%s: %w", kind, namespace, name, err)
	}

	return obj, nil
}

// getPodOwnerWorkload resolves a pod's owner workload using the privileged client for audit purposes.
// It walks the owner chain: Pod -> ReplicaSet/Job -> Deployment/StatefulSet/DaemonSet/CronJob.
func (h *Handler) getPodOwnerWorkload(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	kubeClient := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges())

	// Fetch the pod.
	pod := &unstructured.Unstructured{}
	pod.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: workloadKindPod})
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pod); err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
	}

	// Find the controller owner of the pod.
	owner := getControllerOwner(pod)
	if owner == nil {
		return nil, fmt.Errorf("pod %s/%s has no controller owner", namespace, name)
	}

	// If the owner is a top-level workload, fetch and return it.
	switch owner.Kind {
	case workloadKindDeployment, workloadKindStatefulSet, workloadKindDaemonSet, workloadKindCronJob:
		return fetchOwner(ctx, kubeClient, owner, namespace)
	}

	// The owner is an intermediate resource (ReplicaSet, Job); fetch it and walk up.
	intermediate := &unstructured.Unstructured{}
	ownerGV, _ := schema.ParseGroupVersion(owner.APIVersion)
	intermediate.SetGroupVersionKind(ownerGV.WithKind(owner.Kind))
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: owner.Name}, intermediate); err != nil {
		return nil, fmt.Errorf("failed to get %s %s/%s: %w", owner.Kind, namespace, owner.Name, err)
	}

	topOwner := getControllerOwner(intermediate)
	if topOwner == nil {
		return nil, fmt.Errorf("%s %s/%s has no controller owner", owner.Kind, namespace, owner.Name)
	}

	return fetchOwner(ctx, kubeClient, topOwner, namespace)
}

// getControllerOwner returns the controller ownerReference from an unstructured object, or nil.
func getControllerOwner(obj *unstructured.Unstructured) *metav1.OwnerReference {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			return &ref
		}
	}
	return nil
}

// fetchOwner fetches an unstructured object based on an ownerReference.
func fetchOwner(ctx context.Context, kubeClient client.Client, ref *metav1.OwnerReference, namespace string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	gv, _ := schema.ParseGroupVersion(ref.APIVersion)
	obj.SetGroupVersionKind(gv.WithKind(ref.Kind))
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.Name}, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s %s/%s: %w", ref.Kind, namespace, ref.Name, err)
	}
	return obj, nil
}

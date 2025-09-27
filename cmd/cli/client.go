// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
	cliresource "k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// newKubeClient creates a new controller-runtime client using the local kubeconfig.
func newKubeClient() (client.Client, error) {
	cfg, err := kubeconfigArgs.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	// bump limits
	cfg.QPS = 100.0
	cfg.Burst = 300

	restMapper, err := kubeconfigArgs.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	scheme := apiruntime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := fluxcdv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	kubeClient, err := client.New(cfg, client.Options{Mapper: restMapper, Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return client.WithFieldOwner(kubeClient, "flux-operator-ctl"), nil
}

// annotateResource annotates a resource with the specified key and value.
func annotateResource(ctx context.Context, gvk schema.GroupVersionKind, name, namespace, key, val string) error {
	return annotateResourceWithMap(ctx, gvk, name, namespace, map[string]string{key: val})
}

// annotateResourceWithMap annotates a resource with the provided map of annotations.
func annotateResourceWithMap(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, m map[string]string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := kubeClient.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := client.MergeFrom(resource.DeepCopy())

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	maps.Copy(annotations, m)
	resource.SetAnnotations(annotations)

	if err := kubeClient.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}

// waitForResourceReconciliation waits for a resource to become ready after a reconciliation request.
func waitForResourceReconciliation(ctx context.Context, gvk schema.GroupVersionKind, name, namespace, requestTime string, timeout time.Duration) (string, error) {
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	kubeClient, err := newKubeClient()
	if err != nil {
		return "", fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true,
		isResourceReconciledFunc(kubeClient, resource, requestTime)); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "exceed context deadline") {
			return "", fmt.Errorf("timed out waiting for %s/%s to become ready", namespace, name)
		}

		return "", err
	}

	if res, err := status.GetObjectWithConditions(resource.Object); err == nil {
		for _, cond := range res.Status.Conditions {
			if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionTrue {
				return cond.Message, nil
			}
		}
	}

	return "Reconciliation completed successfully", nil
}

// isReady checks if a resource is ready by examining its Ready condition.
// It verifies that the observed generation matches the current generation and the condition status is True.
func isReady(obj *unstructured.Unstructured) (bool, error) {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found || err != nil {
		return false, nil
	}

	for _, conditionRaw := range conditions {
		condition, ok := conditionRaw.(map[string]any)
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condition, "type")
		if condType == meta.ReadyCondition {
			// Check if the observed generation matches the current generation
			observedGeneration, _, _ := unstructured.NestedInt64(condition, "observedGeneration")
			currentGeneration := obj.GetGeneration()
			if observedGeneration != currentGeneration {
				return false, nil
			}

			// Check the condition status and return the reason and message on failure
			condStatus, _, _ := unstructured.NestedString(condition, "status")
			switch condStatus {
			case string(corev1.ConditionTrue):
				return true, nil
			case string(corev1.ConditionUnknown):
				return false, nil
			case string(corev1.ConditionFalse):
				reason, _, _ := unstructured.NestedString(condition, "reason")
				message, _, _ := unstructured.NestedString(condition, "message")
				return false, fmt.Errorf("%s: %s", reason, message)
			}
		}
	}

	return false, nil
}

// isResourceReconciledFunc returns a function that checks if a resource has been reconciled and is ready.
func isResourceReconciledFunc(kubeClient client.Client, obj *unstructured.Unstructured, requestTime string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		err := kubeClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			if apierrors.IsNotFound(err) && requestTime == "" {
				// If the resource is not found and no request time is specified,
				// we wait for the resource to be created.
				return false, nil
			}
			return false, err
		}

		suspendedMsg := "Reconciliation is disabled"

		// Check if the resource is suspended via annotations
		if ssautil.AnyInMetadata(obj, map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
			return false, fmt.Errorf("%s for %s", suspendedMsg, obj.GetName())
		}

		// Check if the resource is suspended via spec.suspend field
		if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend && found && err == nil {
			return false, fmt.Errorf("%s for %s", suspendedMsg, obj.GetName())
		}

		// Check if the status.lastHandledReconcileAt matches the request time
		if requestTime != "" {
			lastHandledReconcileAt, _, _ := unstructured.NestedString(obj.Object, "status", "lastHandledReconcileAt")
			if lastHandledReconcileAt != requestTime {
				return false, nil
			}
		}

		// Check if the resource is ready
		return isReady(obj)
	}
}

// toggleSuspension toggles the suspension of a Flux resource by setting or removing the `spec.suspend` field.
// If the resource is a Flux Operator resource, it uses annotations instead.
// When resuming a resource, it also sets the ReconcileAt annotation.
func toggleSuspension(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, requestTime string, suspend bool) error {
	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

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

		return annotateResourceWithMap(ctx, gvk, name, namespace, annotations)
	}

	// Handle Flux resources by patching the spec.suspend field.
	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := client.MergeFrom(resource.DeepCopy())

	if suspend {
		err := unstructured.SetNestedField(resource.Object, suspend, "spec", "suspend")
		if err != nil {
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
		return fmt.Errorf("unable to patch %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}

// preferredFluxGVK returns the preferred GroupVersionKind for a given Flux kind.
func preferredFluxGVK(kind string, cf *cliopts.ConfigFlags) (*schema.GroupVersionKind, error) {
	gk, err := fluxcdv1.FluxGroupFor(kind)
	if err != nil {
		return nil, err
	}

	mapper, err := cf.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("unable to create REST mapper: %w", err)
	}

	mapping, err := mapper.RESTMapping(*gk)
	if err != nil {
		return nil, err
	}

	return &mapping.GroupVersionKind, nil
}

// getObjectByKindName retrieves a Kubernetes object by its kind and name
// from the cluster using the preferred API version and group for the specified kind.
func getObjectByKindName(args []string) (*unstructured.Unstructured, error) {
	r := cliresource.NewBuilder(kubeconfigArgs).
		Unstructured().
		NamespaceParam(*kubeconfigArgs.Namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(false, args...).
		ContinueOnError().
		Latest().
		Do()

	if err := r.Err(); err != nil {
		if cliresource.IsUsageError(err) {
			return nil, fmt.Errorf("either `<resource>/<name>` or `<resource> <name>` is required as an argument")
		}
		return nil, err
	}

	infos, err := r.Infos()
	if err != nil {
		return nil, fmt.Errorf("x: %v", err)
	}

	if len(infos) == 0 {
		return nil, fmt.Errorf("failed to find object: %s", strings.Join(args[:], " "))
	} else if len(infos) > 1 {
		return nil, fmt.Errorf("multiple objects found for: %s", strings.Join(args[:], " "))
	}

	obj := &unstructured.Unstructured{}
	obj.Object, err = apiruntime.DefaultUnstructuredConverter.ToUnstructured(infos[0].Object)
	return obj, err
}

// timeNow returns the current time in RFC3339Nano format.
func timeNow() string {
	return metav1.Now().Format(time.RFC3339Nano)
}

// ResourceStatus represents the status of a Flux resource.
type ResourceStatus struct {
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	LastReconciled string `json:"lastReconciled"`
	Ready          string `json:"ready"`
	ReadyMessage   string `json:"message"`
}

// resourceStatusFromUnstructured extracts the ResourceStatus from an unstructured Kubernetes object.
func resourceStatusFromUnstructured(obj unstructured.Unstructured) ResourceStatus {
	name := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	ready := "Unknown"
	readyMsg := "Not initialized"
	lastReconciled := "Unknown"
	if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && err == nil {
		for _, cond := range conditions {
			if condition, ok := cond.(map[string]any); ok && condition["type"] == meta.ReadyCondition {
				ready = condition["status"].(string)
				if msg, exists := condition["message"]; exists {
					readyMsg = msg.(string)
				}
				if lastTransitionTime, exists := condition["lastTransitionTime"]; exists {
					lastReconciled = lastTransitionTime.(string)
				}
			}
		}
	}

	if ssautil.AnyInMetadata(&obj,
		map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
		ready = "Suspended"
	}

	if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend && found && err == nil {
		ready = "Suspended"
	}

	return ResourceStatus{
		Kind:           obj.GetKind(),
		Name:           name,
		LastReconciled: lastReconciled,
		Ready:          ready,
		ReadyMessage:   readyMsg,
	}
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
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
		return "", err
	}

	if res, err := status.GetObjectWithConditions(resource.Object); err == nil {
		for _, cond := range res.Status.Conditions {
			if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionTrue {
				return cond.Message, nil
			}
		}
	}

	return "reconciliation completed successfully", nil
}

// isResourceReconciledFunc returns a function that checks if a resource has been reconciled and is ready.
func isResourceReconciledFunc(kubeClient client.Client, obj *unstructured.Unstructured, requestTime string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		err := kubeClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
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
		lastHandledReconcileAt, _, _ := unstructured.NestedString(obj.Object, "status", "lastHandledReconcileAt")
		if lastHandledReconcileAt != requestTime {
			return false, nil
		}

		// Check if the resource is ready
		if res, err := status.GetObjectWithConditions(obj.Object); err == nil {
			for _, cond := range res.Status.Conditions {
				if cond.Type == meta.ReadyCondition {
					switch cond.Status {
					case corev1.ConditionTrue:
						return true, nil
					case corev1.ConditionUnknown:
						return false, nil
					case corev1.ConditionFalse:
						return false, errors.New(cond.Message)
					}
				}
			}
		}

		return false, nil
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
	gk := schema.GroupKind{
		Kind: kind,
	}

	mapper, err := cf.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("unable to create REST mapper: %w", err)
	}

	switch kind {
	case "FluxInstance", "FluxReport", "ResourceSet", "ResourceSetInputProvider":
		gk.Group = fluxcdv1.GroupVersion.Group
	case "GitRepository", "OCIRepository", "Bucket", "HelmChart", "HelmRepository":
		gk.Group = "source.toolkit.fluxcd.io"
	case "Alert", "Provider", "Receiver":
		gk.Group = "notification.toolkit.fluxcd.io"
	case "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		gk.Group = "image.toolkit.fluxcd.io"
	case "Kustomization":
		gk.Group = "kustomize.toolkit.fluxcd.io"
	case "HelmRelease":
		gk.Group = "helm.toolkit.fluxcd.io"
	default:
		return nil, fmt.Errorf("unknown Flux kind %s", kind)
	}

	mapping, err := mapper.RESTMapping(gk)
	if err != nil {
		return nil, err
	}

	return &mapping.GroupVersionKind, nil
}

// timeNow returns the current time in RFC3339Nano format.
func timeNow() string {
	return metav1.Now().Format(time.RFC3339Nano)
}

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fluxcd/pkg/ssa"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

// uninstall deletes all the resources managed by the FluxInstance, removes the
// finalizers from the Flux custom resources and stops the reconciliation loop.
//
//nolint:unparam
func (r *FluxInstanceReconciler) uninstall(ctx context.Context,
	obj *fluxcdv1.FluxInstance) (ctrl.Result, error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	if obj.IsDisabled() || obj.Status.Inventory == nil || len(obj.Status.Inventory.Entries) == 0 {
		controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)
		return ctrl.Result{}, nil
	}

	resourceManager := ssa.NewResourceManager(r.Client, nil, ssa.Owner{
		Field: r.StatusManager,
		Group: fluxcdv1.GroupVersion.Group,
	})

	opts := ssa.DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
		Inclusions:        resourceManager.GetOwnerLabels(obj.Name, obj.Namespace),
		Exclusions: map[string]string{
			fluxcdv1.PruneAnnotation: fluxcdv1.DisabledValue,
		},
	}

	objects, _ := inventory.List(obj.Status.Inventory)

	// Step1: delete the deployments and wait for them to be removed.
	// This ensures that the controllers will not be running while we
	// remove the finalizers from the custom resources.
	deployments := []*unstructured.Unstructured{}
	for _, entry := range objects {
		if entry.GetKind() == "Deployment" {
			deployments = append(deployments, entry)
		}
	}
	_, err := resourceManager.DeleteAll(ctx, deployments, opts)
	if err != nil {
		log.Error(err, "deleting deployments failed")
	} else {
		if err := resourceManager.WaitForTermination(deployments, ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  5 * time.Minute,
		}); err != nil {
			log.Error(err, "waiting for deployments to be deleted failed")
		}
	}

	// Step2: discover the Flux custom resources in the cluster and remove their finalizers.
	// This ensures that the resources can be deleted without being blocked by finalizers.
	err = r.removeFluxFinalizers(ctx)
	if err != nil {
		log.Error(err, "removing finalizers failed")
	}

	// Step3: delete all the resources from the inventory.
	// This will also delete all Flux custom resources as the Kubernetes
	// garbage collector will take care of removing the resources owned by CRDs.
	changeSet, err := resourceManager.DeleteAll(ctx, objects, opts)
	if err != nil {
		log.Error(err, "pruning for deleted resource failed")
	}

	// Step4: wait for the CRDs to be deleted.
	// This will block until all Flux custom resources are deleted.
	crds := []*unstructured.Unstructured{}
	for _, entry := range objects {
		if entry.GetKind() == "CustomResourceDefinition" {
			crds = append(crds, entry)
		}
	}
	if err := resourceManager.WaitForTermination(crds, ssa.WaitOptions{
		Interval: 5 * time.Second,
		Timeout:  5 * time.Minute,
	}); err != nil {
		log.Error(err, "waiting for CRDs to be deleted failed")
	}

	// Step5: remove the finalizer from the FluxInstance.
	// The object will be deleted by Kubernetes once the finalizer is removed.
	controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)
	msg := uninstallMessage(reconcileStart)
	log.Info(msg, "output", changeSet.ToMap())

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

// removeFluxFinalizers removes the finalizers from the Flux custom resources.
func (r *FluxInstanceReconciler) removeFluxFinalizers(ctx context.Context) error {
	var errs []error
	versions, err := r.getInstalledGVKs()
	if err != nil {
		errs = append(errs, err)
	}

	for kind, apiVersion := range versions {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.removeFinalizersFor(ctx, apiVersion, kind+"List")
		})
		if err != nil {
			if !strings.Contains(err.Error(), "the server could not find the requested resource") &&
				!strings.Contains(err.Error(), "no matches for kind") {
				errs = append(errs, err)
			}
		} else {
			ctrl.LoggerFrom(ctx).Info("removed finalizers for " + kind)
		}
	}

	return errors.Join(errs...)
}

// getInstalledGVKs returns a map of installed Flux custom resource kinds to their preferred API versions.
func (r *FluxInstanceReconciler) getInstalledGVKs() (map[string]string, error) {
	var errs []error
	result := make(map[string]string)

	for _, kind := range fluxcdv1.FluxKinds {
		gk, err := fluxcdv1.FluxGroupFor(kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		mapping, err := r.RESTMapper().RESTMapping(*gk)
		if err != nil {
			if !strings.Contains(err.Error(), "no matches for kind") {
				errs = append(errs, err)
			}
			continue
		}

		result[kind] = mapping.GroupVersionKind.GroupVersion().String()
	}

	return result, errors.Join(errs...)
}

// removeFinalizersFor is generic function to remove finalizers from all resources.
func (r *FluxInstanceReconciler) removeFinalizersFor(ctx context.Context, apiVersion, listKind string) error {
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(apiVersion)
	list.SetKind(listKind)

	err := r.Client.List(ctx, list, client.InNamespace(""))
	if err != nil {
		return err
	}

	var errs []error
	for i := range list.Items {
		entry := list.Items[i]
		entry.SetFinalizers([]string{})
		if err := r.Client.Update(ctx, &entry); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

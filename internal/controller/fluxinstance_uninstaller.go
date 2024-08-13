// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"fmt"
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
	changeSet, err := resourceManager.DeleteAll(ctx, objects, opts)
	if err != nil {
		log.Error(err, "pruning for deleted resource failed")
	}

	err = r.removeFluxFinalizers(ctx)
	if err != nil {
		log.Error(err, "removing finalizers failed")
	}

	controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)
	msg := fmt.Sprintf("Uninstallation completed in %v", fmtDuration(reconcileStart))
	log.Info(msg, "output", changeSet.ToMap())

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

// removeFluxFinalizers removes the finalizers from the Flux custom resources.
func (r *FluxInstanceReconciler) removeFluxFinalizers(ctx context.Context) error {
	var errs []error
	versions := []struct {
		apiVersion string
		listKind   string
	}{
		{"kustomize.toolkit.fluxcd.io/v1", "KustomizationList"},
		{"helm.toolkit.fluxcd.io/v2beta1", "HelmReleaseList"},
		{"helm.toolkit.fluxcd.io/v2beta2", "HelmReleaseList"},
		{"helm.toolkit.fluxcd.io/v2", "HelmReleaseList"},
		{"source.toolkit.fluxcd.io/v1beta2", "HelmRepositoryList"},
		{"source.toolkit.fluxcd.io/v1", "HelmRepositoryList"},
		{"source.toolkit.fluxcd.io/v1beta2", "HelmChartList"},
		{"source.toolkit.fluxcd.io/v1", "HelmChartList"},
		{"source.toolkit.fluxcd.io/v1", "GitRepositoryList"},
		{"source.toolkit.fluxcd.io/v1beta2", "OCIRepositoryList"},
		{"source.toolkit.fluxcd.io/v1beta2", "BucketList"},
		{"notification.toolkit.fluxcd.io/v1", "ReceiverList"},
		{"notification.toolkit.fluxcd.io/v1beta2", "ProviderList"},
		{"notification.toolkit.fluxcd.io/v1beta3", "ProviderList"},
		{"notification.toolkit.fluxcd.io/v1beta2", "AlertList"},
		{"notification.toolkit.fluxcd.io/v1beta3", "AlertList"},
		{"image.toolkit.fluxcd.io/v1beta2", "ImageRepositoryList"},
		{"image.toolkit.fluxcd.io/v1beta2", "ImagePolicyList"},
		{"image.toolkit.fluxcd.io/v1beta1", "ImageUpdateAutomationList"},
		{"image.toolkit.fluxcd.io/v1beta2", "ImageUpdateAutomationList"},
	}

	for _, v := range versions {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.removeFinalizersFor(ctx, v.apiVersion, v.listKind)
		})
		if err != nil {
			if !strings.Contains(err.Error(), "the server could not find the requested resource") &&
				!strings.Contains(err.Error(), "no matches for kind") {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
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

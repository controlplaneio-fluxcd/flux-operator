// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fluxcd/pkg/ssa"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// migrateResources migrates the resources for the CRDs that match the given label selector
// to the latest storage version and updates the CRD status to contain only the latest storage version.
func (r *FluxInstanceReconciler) migrateResources(ctx context.Context, labelSelector client.MatchingLabels, force bool) error {
	var errs []error
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}

	if err := r.Client.List(ctx, crdList, labelSelector); err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	for _, crd := range crdList.Items {
		if err := r.migrateCRD(ctx, crd.Name, force); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// migrateCRD migrates the custom resources for the given CRD to the latest storage version
// and updates the CRD status to contain only the latest storage version.
func (r *FluxInstanceReconciler) migrateCRD(ctx context.Context, name string, force bool) error {
	log := ctrl.LoggerFrom(ctx)
	crd := &apiextensionsv1.CustomResourceDefinition{}

	if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, crd); err != nil {
		return fmt.Errorf("failed to get CRD %s: %w", name, err)
	}

	// get the latest storage version for the CRD
	storageVersion := r.getStorageVersion(crd)
	if storageVersion == "" {
		return fmt.Errorf("no storage version found for CRD %s", name)
	}

	// return early if the CRD has a single stored version and force is not set
	if !force && len(crd.Status.StoredVersions) == 1 && crd.Status.StoredVersions[0] == storageVersion {
		return nil
	}

	// migrate the resources for the CRD
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.migrateCR(ctx, crd, storageVersion)
	})
	if err != nil {
		return fmt.Errorf("failed to migrate resources for CRD %s: %w", name, err)
	}

	// patch the CRD status to update the stored version to the latest
	crd.Status.StoredVersions = []string{storageVersion}
	if err := r.Client.Status().Update(ctx, crd); err != nil {
		return fmt.Errorf("failed to update CRD %s status: %w", crd.Name, err)
	}

	log.Info("CRD migrated "+crd.Name, "storageVersion", storageVersion)

	return nil
}

// migrateCR migrates the resources for the given CRD to the specified version.
// If a resource contains managed fields with an older version, it will be patched to the latest version.
func (r *FluxInstanceReconciler) migrateCR(ctx context.Context, crd *apiextensionsv1.CustomResourceDefinition, version string) error {
	list := &unstructured.UnstructuredList{}

	apiVersion := crd.Spec.Group + "/" + version
	listKind := crd.Spec.Names.ListKind

	list.SetAPIVersion(apiVersion)
	list.SetKind(listKind)

	err := r.Client.List(ctx, list, client.InNamespace(""))
	if err != nil {
		return fmt.Errorf("failed to list resources for CRD %s: %w", crd.Name, err)
	}

	if len(list.Items) == 0 {
		return nil
	}

	for _, item := range list.Items {
		patches, err := ssa.PatchMigrateToVersion(&item, apiVersion)
		if err != nil {
			return fmt.Errorf("failed to create migration patch for %s/%s/%s: %w",
				item.GetKind(), item.GetNamespace(), item.GetName(), err)
		}

		if len(patches) == 0 {
			// patch the resource with an empty patch to update the version
			if err := r.Patch(
				ctx,
				&item,
				client.RawPatch(client.Merge.Type(), []byte("{}")),
			); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf(" %s/%s/%s failed to migrate: %w",
					item.GetKind(), item.GetNamespace(), item.GetName(), err)
			}
		} else {
			// patch the resource to migrate the managed fields to the latest apiVersion
			rawPatch, err := json.Marshal(patches)
			if err != nil {
				return fmt.Errorf("failed to marshal migration patch for %s/%s/%s: %w",
					item.GetKind(), item.GetNamespace(), item.GetName(), err)
			}
			if err := r.Patch(
				ctx,
				&item,
				client.RawPatch(types.JSONPatchType, rawPatch),
			); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf(" %s/%s/%s failed to migrate managed fields: %w",
					item.GetKind(), item.GetNamespace(), item.GetName(), err)
			}
		}
	}

	return nil
}

func (r *FluxInstanceReconciler) getStorageVersion(crd *apiextensionsv1.CustomResourceDefinition) string {
	var version string

	for _, v := range crd.Spec.Versions {
		if v.Storage {
			version = v.Name
			break
		}
	}

	return version
}

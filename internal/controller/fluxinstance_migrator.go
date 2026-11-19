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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// migrateListPageSize is the number of custom resources fetched per list call
// during API version migration. Paging keeps the memory used to decode the
// resources bounded on clusters with many large objects.
const migrateListPageSize = 500

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
	apiVersion := crd.Spec.Group + "/" + version
	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: version,
		Kind:    crd.Spec.Names.Kind,
	}

	// The continue token of a paginated list points at the snapshot taken when
	// the first page was read. Patching between pages can take longer than the
	// etcd compaction window on very large clusters, causing subsequent List
	// calls to return 410 Expired. Restart from the first page and skip the
	// objects that were already patched, so every attempt makes progress.
	patched := make(map[types.NamespacedName]struct{})
	for {
		progressed, err := r.migrateCRPages(ctx, gvk, apiVersion, patched)
		if err == nil {
			return nil
		}
		if !apierrors.IsResourceExpired(err) {
			return fmt.Errorf("failed to migrate resources for CRD %s: %w", crd.Name, err)
		}
		if !progressed {
			return fmt.Errorf("failed to migrate resources for CRD %s: %w", crd.Name, err)
		}
	}
}

// migrateCRPages migrates the given resource pages, skipping the objects that
// were already patched in a previous attempt. It reports whether any object
// was patched before the list expired.
func (r *FluxInstanceReconciler) migrateCRPages(ctx context.Context, gvk schema.GroupVersionKind,
	apiVersion string, patched map[types.NamespacedName]struct{}) (bool, error) {

	progressed := false
	continueToken := ""
	for {
		// Only the object metadata, including managedFields, is needed to
		// migrate the API version. Listing PartialObjectMetadata avoids
		// decoding the full spec and status of every resource.
		list := &metav1.PartialObjectMetadataList{}
		list.SetGroupVersionKind(gvk)

		listOpts := []client.ListOption{
			client.InNamespace(""),
			client.Limit(migrateListPageSize),
		}
		if continueToken != "" {
			listOpts = append(listOpts, client.Continue(continueToken))
		}

		if err := r.Client.List(ctx, list, listOpts...); err != nil {
			return progressed, err
		}

		for i := range list.Items {
			item := &list.Items[i]
			key := types.NamespacedName{Namespace: item.Namespace, Name: item.Name}
			if _, ok := patched[key]; ok {
				continue
			}

			// PatchMigrateToVersion only reads the managed fields, so build a
			// metadata-only object to compute the patch.
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(item.GroupVersionKind())
			obj.SetName(item.Name)
			obj.SetNamespace(item.Namespace)
			obj.SetManagedFields(item.GetManagedFields())

			patches, err := ssa.PatchMigrateToVersion(obj, apiVersion)
			if err != nil {
				return progressed, fmt.Errorf("failed to create migration patch for %s/%s/%s: %w",
					item.Kind, item.GetNamespace(), item.GetName(), err)
			}

			if len(patches) == 0 {
				// patch the resource with an empty patch to update the version
				if err := r.Patch(
					ctx,
					item,
					client.RawPatch(client.Merge.Type(), []byte("{}")),
				); err != nil && !apierrors.IsNotFound(err) {
					return progressed, fmt.Errorf(" %s/%s/%s failed to migrate: %w",
						item.Kind, item.GetNamespace(), item.GetName(), err)
				}
			} else {
				// patch the resource to migrate the managed fields to the latest apiVersion
				rawPatch, err := json.Marshal(patches)
				if err != nil {
					return progressed, fmt.Errorf("failed to marshal migration patch for %s/%s/%s: %w",
						item.Kind, item.GetNamespace(), item.GetName(), err)
				}
				if err := r.Patch(
					ctx,
					item,
					client.RawPatch(types.JSONPatchType, rawPatch),
				); err != nil && !apierrors.IsNotFound(err) {
					return progressed, fmt.Errorf(" %s/%s/%s failed to migrate managed fields: %w",
						item.Kind, item.GetNamespace(), item.GetName(), err)
				}
			}

			patched[key] = struct{}{}
			progressed = true
		}

		continueToken = list.GetContinue()
		if continueToken == "" {
			return progressed, nil
		}
	}
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

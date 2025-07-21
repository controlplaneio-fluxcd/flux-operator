// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inventory

import (
	"sort"

	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/pkg/ssa"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func New() *fluxcdv1.ResourceInventory {
	return &fluxcdv1.ResourceInventory{
		Entries: []fluxcdv1.ResourceRef{},
	}
}

// AddChangeSet extracts the metadata from the given objects and adds it to the inventory.
func AddChangeSet(inv *fluxcdv1.ResourceInventory, set *ssa.ChangeSet) error {
	if set == nil {
		return nil
	}

	for _, entry := range set.Entries {
		inv.Entries = append(inv.Entries, fluxcdv1.ResourceRef{
			ID:      entry.ObjMetadata.String(),
			Version: entry.GroupVersion,
		})
	}

	return nil
}

// List returns the inventory entries as unstructured.Unstructured objects.
func List(inv *fluxcdv1.ResourceInventory) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)

	if inv.Entries == nil {
		return objects, nil
	}

	for _, entry := range inv.Entries {
		objMetadata, err := object.ParseObjMetadata(entry.ID)
		if err != nil {
			return nil, err
		}

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   objMetadata.GroupKind.Group,
			Kind:    objMetadata.GroupKind.Kind,
			Version: entry.Version,
		})
		u.SetName(objMetadata.Name)
		u.SetNamespace(objMetadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(ssa.SortableUnstructureds(objects))
	return objects, nil
}

// ListMetadata returns the inventory entries as object.ObjMetadata objects.
// nolint:prealloc
func ListMetadata(inv *fluxcdv1.ResourceInventory) (object.ObjMetadataSet, error) {
	var metas []object.ObjMetadata
	for _, e := range inv.Entries {
		m, err := object.ParseObjMetadata(e.ID)
		if err != nil {
			return metas, err
		}
		metas = append(metas, m)
	}

	return metas, nil
}

// Diff returns the slice of objects that do not exist in the target inventory.
func Diff(inv *fluxcdv1.ResourceInventory, target *fluxcdv1.ResourceInventory) ([]*unstructured.Unstructured, error) {
	versionOf := func(i *fluxcdv1.ResourceInventory, objMetadata object.ObjMetadata) string {
		for _, entry := range i.Entries {
			if entry.ID == objMetadata.String() {
				return entry.Version
			}
		}
		return ""
	}

	objects := make([]*unstructured.Unstructured, 0)
	aList, err := ListMetadata(inv)
	if err != nil {
		return nil, err
	}

	bList, err := ListMetadata(target)
	if err != nil {
		return nil, err
	}

	list := aList.Diff(bList)
	if len(list) == 0 {
		return objects, nil
	}

	for _, metadata := range list {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   metadata.GroupKind.Group,
			Kind:    metadata.GroupKind.Kind,
			Version: versionOf(inv, metadata),
		})
		u.SetName(metadata.Name)
		u.SetNamespace(metadata.Namespace)
		objects = append(objects, u)
	}

	sort.Sort(ssa.SortableUnstructureds(objects))
	return objects, nil
}

// EntryFromObjMetadata converts an object.ObjMetadata to a ResourceRef entry.
func EntryFromObjMetadata(objMetadata object.ObjMetadata, version string) fluxcdv1.ResourceRef {
	return fluxcdv1.ResourceRef{
		ID:      objMetadata.String(),
		Version: version,
	}
}

// EntryToObjMetadata converts a ResourceRef entry to an object.ObjMetadata.
func EntryToObjMetadata(entry fluxcdv1.ResourceRef) (object.ObjMetadata, error) {
	return object.ParseObjMetadata(entry.ID)
}

// EntryToUnstructured converts a ResourceRef entry to an unstructured.Unstructured object.
func EntryToUnstructured(entry fluxcdv1.ResourceRef) (*unstructured.Unstructured, error) {
	objMetadata, err := EntryToObjMetadata(entry)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   objMetadata.GroupKind.Group,
		Kind:    objMetadata.GroupKind.Kind,
		Version: entry.Version,
	})
	u.SetName(objMetadata.Name)
	u.SetNamespace(objMetadata.Namespace)

	return u, nil
}

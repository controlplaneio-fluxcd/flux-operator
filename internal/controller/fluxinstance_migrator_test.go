// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// metadataMigrationClient simulates a paginated metadata-only list, including
// a continue token expiring once while patching the first page.
type metadataMigrationClient struct {
	client.Client

	pages      [][]metav1.PartialObjectMetadata
	expired    bool
	listCalls  int
	patched    []string
	patchTypes []types.PatchType
}

func (c *metadataMigrationClient) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	var listOpts client.ListOptions
	listOpts.ApplyOptions(opts)

	if listOpts.Limit != migrateListPageSize {
		return fmt.Errorf("expected list limit %d, got %d", migrateListPageSize, listOpts.Limit)
	}

	ml, ok := list.(*metav1.PartialObjectMetadataList)
	if !ok {
		return fmt.Errorf("expected PartialObjectMetadataList, got %T", list)
	}

	page := 0
	if listOpts.Continue != "" {
		n, err := strconv.Atoi(listOpts.Continue)
		if err != nil {
			return fmt.Errorf("invalid continue token %q: %w", listOpts.Continue, err)
		}
		page = n
	}
	if page >= len(c.pages) {
		return fmt.Errorf("page %d out of range", page)
	}

	// Simulate etcd compaction expiring the first attempt's snapshot.
	if page == 1 && !c.expired {
		c.expired = true
		return apierrors.NewResourceExpired("continue token expired")
	}

	ml.Items = c.pages[page]
	for i := range ml.Items {
		ml.Items[i].SetGroupVersionKind(schema.GroupVersionKind{
			Group:   fluxcdv1.FluxHelmGroup,
			Version: "v2",
			Kind:    fluxcdv1.FluxHelmReleaseKind,
		})
	}
	if page+1 < len(c.pages) {
		ml.SetContinue(strconv.Itoa(page + 1))
	}
	c.listCalls++
	return nil
}

func (c *metadataMigrationClient) Patch(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
	if _, ok := obj.(*metav1.PartialObjectMetadata); !ok {
		return fmt.Errorf("expected PartialObjectMetadata, got %T", obj)
	}
	c.patched = append(c.patched, obj.GetNamespace()+"/"+obj.GetName())
	c.patchTypes = append(c.patchTypes, patch.Type())
	return nil
}

func TestMigrateCRPaginatesMetadataAndRestartsOnExpiry(t *testing.T) {
	g := NewWithT(t)

	managedFields := []metav1.ManagedFieldsEntry{
		{
			Manager:    "test",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: "helm.toolkit.fluxcd.io/v2beta1",
			FieldsType: "FieldsV1",
			FieldsV1:   metav1.NewFieldsV1(`{"f:metadata":{}}`),
		},
	}
	item := func(name string) metav1.PartialObjectMetadata {
		return metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				Name:          name,
				Namespace:     "apps",
				ManagedFields: managedFields,
			},
		}
	}

	kc := &metadataMigrationClient{
		pages: [][]metav1.PartialObjectMetadata{
			{item("hr-1"), item("hr-2")},
			{item("hr-3")},
		},
	}
	r := &FluxInstanceReconciler{Client: kc}
	crd := &apiextensionsv1.CustomResourceDefinition{
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: fluxcdv1.FluxHelmGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     fluxcdv1.FluxHelmReleaseKind,
				ListKind: fluxcdv1.FluxHelmReleaseKind + "List",
			},
		},
	}

	err := r.migrateCR(context.Background(), crd, "v2")
	g.Expect(err).ToNot(HaveOccurred())

	// Page 1 is listed, patched, then page 2 expires. The retry lists page 1
	// again (skipping the patched objects) and patches page 2.
	g.Expect(kc.listCalls).To(Equal(3))
	g.Expect(kc.patched).To(ConsistOf("apps/hr-1", "apps/hr-2", "apps/hr-3"))
	g.Expect(kc.patchTypes).To(ConsistOf(types.JSONPatchType, types.JSONPatchType, types.JSONPatchType))
}

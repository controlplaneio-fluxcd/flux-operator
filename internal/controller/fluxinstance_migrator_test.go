// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	r := &FluxInstanceReconciler{Client: kc, APIReader: kc}
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

func TestMigrateCRDWithCachedClient(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(testCtx, timeout)
	defer cancel()

	group := "migration.fluxcd.controlplane.io"
	preserveUnknown := true
	crdVersion := func(name string, storage bool) apiextensionsv1.CustomResourceDefinitionVersion {
		return apiextensionsv1.CustomResourceDefinitionVersion{
			Name:    name,
			Served:  true,
			Storage: storage,
			Schema: &apiextensionsv1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type:                   "object",
					XPreserveUnknownFields: &preserveUnknown,
				},
			},
		}
	}
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "migrationtests." + group,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "MigrationTest",
				Plural: "migrationtests",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				crdVersion("v1beta1", false),
				crdVersion("v1", true),
			},
		},
	}
	g.Expect(testClient.Create(ctx, crd)).To(Succeed())
	defer func() {
		_ = testClient.Delete(context.Background(), crd)
	}()

	g.Eventually(func() bool {
		if err := testClient.Get(ctx, client.ObjectKeyFromObject(crd), crd); err != nil {
			return false
		}
		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				return true
			}
		}
		return false
	}, timeout, time.Second).Should(BeTrue())

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create the objects with v1beta1, so that their managed fields reference it.
	names := []string{"test-1", "test-2", "test-3"}
	for _, name := range names {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(group + "/v1beta1")
		obj.SetKind("MigrationTest")
		obj.SetName(name)
		obj.SetNamespace(ns.Name)
		g.Expect(unstructured.SetNestedField(obj.Object, name, "spec", "value")).To(Succeed())
		g.Expect(testClient.Create(ctx, obj)).To(Succeed())
	}

	crd.Status.StoredVersions = []string{"v1beta1", "v1"}
	g.Expect(testClient.Status().Update(ctx, crd)).To(Succeed())

	// migrateCRD reads the CRD from the cache and returns early on a stale copy.
	g.Eventually(func() []string {
		cached := &apiextensionsv1.CustomResourceDefinition{}
		if err := testEnv.Get(ctx, client.ObjectKeyFromObject(crd), cached); err != nil {
			return nil
		}
		return cached.Status.StoredVersions
	}, timeout, time.Second).Should(HaveLen(2))

	// Use the manager cached client as the operator does.
	r := &FluxInstanceReconciler{
		Client:    testEnv.GetClient(),
		APIReader: testClient,
	}
	g.Expect(r.migrateCRD(ctx, crd.Name, false)).To(Succeed())

	result := &apiextensionsv1.CustomResourceDefinition{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(crd), result)).To(Succeed())
	g.Expect(result.Status.StoredVersions).To(Equal([]string{"v1"}))

	for _, name := range names {
		obj := &metav1.PartialObjectMetadata{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: group, Version: "v1", Kind: "MigrationTest"})
		g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: name}, obj)).To(Succeed())
		g.Expect(obj.GetManagedFields()).ToNot(BeEmpty())
		for _, entry := range obj.GetManagedFields() {
			g.Expect(entry.APIVersion).To(Equal(group+"/v1"), "object %s manager %s", name, entry.Manager)
		}
	}
}

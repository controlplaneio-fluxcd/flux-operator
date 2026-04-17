// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const kubectlFieldManager = "kubectl"

func TestKindToManager(t *testing.T) {
	tests := []struct {
		kind        string
		expected    string
		expectError bool
	}{
		{kind: fluxcdv1.FluxHelmReleaseKind, expected: fluxcdv1.FluxHelmController},
		{kind: fluxcdv1.FluxKustomizationKind, expected: fluxcdv1.FluxKustomizeController},
		{kind: fluxcdv1.ResourceSetKind, expected: fluxcdv1.FluxOperator},
		{kind: fluxcdv1.FluxInstanceKind, expected: fluxcdv1.FluxOperator},
		{kind: "ConfigMap", expectError: true},
		{kind: "", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			g := NewWithT(t)
			got, err := kindToManager(tt.kind)
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(got).To(Equal(tt.expected))
		})
	}
}

func TestStaleFluxManagers(t *testing.T) {
	g := NewWithT(t)

	// Current owner's entries (both Apply and Update) must be excluded.
	stale := staleFluxManagers(fluxcdv1.FluxHelmController)
	for _, m := range stale {
		g.Expect(m.Name).ToNot(Equal(fluxcdv1.FluxHelmController))
		g.Expect(m.ExactMatch).To(BeTrue())
	}
	g.Expect(stale).To(HaveLen(len(allFluxManagers) - 2))

	all := staleFluxManagers("unknown-manager")
	g.Expect(all).To(HaveLen(len(allFluxManagers)))
}

type managedField struct {
	manager   string
	operation metav1.ManagedFieldsOperationType
}

// seedManagedFields replaces the managedFields on obj with the given entries
// via a JSON patch. This emulates a resource that has accumulated entries
// from several Flux appliers over time.
func seedManagedFields(ctx context.Context, g *WithT, obj client.Object, fields []managedField) {
	entries := make([]metav1.ManagedFieldsEntry, 0, len(fields))
	for _, f := range fields {
		entries = append(entries, metav1.ManagedFieldsEntry{
			Manager:    f.manager,
			Operation:  f.operation,
			APIVersion: "v1",
			FieldsType: "FieldsV1",
			FieldsV1: &metav1.FieldsV1{
				Raw: []byte(`{"f:metadata":{"f:labels":{}}}`),
			},
		})
	}
	patch := []map[string]any{
		{"op": "replace", "path": "/metadata/managedFields", "value": entries},
	}
	raw, err := json.Marshal(patch)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(testClient.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, raw))).To(Succeed())
}

func applyFields(managers ...string) []managedField {
	out := make([]managedField, 0, len(managers))
	for _, m := range managers {
		out = append(out, managedField{manager: m, operation: metav1.ManagedFieldsOperationApply})
	}
	return out
}

// inventoryIDForConfigMap formats a ResourceRef ID for a namespaced core/v1 ConfigMap.
func inventoryIDForConfigMap(namespace, name string) string {
	return fmt.Sprintf("%s_%s__ConfigMap", namespace, name)
}

// setResourceSetInventory writes the given configmap references as the
// ResourceSet's status.inventory.entries.
func setResourceSetInventory(ctx context.Context, g *WithT, rs *fluxcdv1.ResourceSet, cmRefs []fluxcdv1.ResourceRef) {
	rs.Status.Inventory = &fluxcdv1.ResourceInventory{Entries: cmRefs}
	g.Expect(testClient.Status().Update(ctx, rs)).To(Succeed())
}

func configMapManagers(ctx context.Context, g *WithT, namespace, name string) []string {
	cm := &corev1.ConfigMap{}
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)).To(Succeed())
	seen := map[string]struct{}{}
	for _, e := range cm.GetManagedFields() {
		seen[e.Manager] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	return out
}

func TestMigrateOwnerCmd_UnsupportedKind(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-unsupported")
	g.Expect(err).ToNot(HaveOccurred())
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: ns.Name},
	}
	g.Expect(testClient.Create(ctx, cm)).To(Succeed())

	kubeconfigArgs.Namespace = &ns.Name
	_, err = executeCommand([]string{"migrate", "owner", "configmap/target"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unsupported kind"))
}

func TestMigrateOwnerCmd_MissingInventory(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-noinv")
	g.Expect(err).ToNot(HaveOccurred())

	rs := newResourceSet(ns.Name)
	g.Expect(testClient.Create(ctx, rs)).To(Succeed())

	kubeconfigArgs.Namespace = &ns.Name
	_, err = executeCommand([]string{"migrate", "owner", "rset/owner"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no status.inventory entries"))
}

func TestMigrateOwnerCmd_ResourceSet(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-rset")
	g.Expect(err).ToNot(HaveOccurred())

	cmName := "app-config"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: ns.Name},
		Data:       map[string]string{"key": "value"},
	}
	g.Expect(testClient.Create(ctx, cm)).To(Succeed())
	seedManagedFields(ctx, g, cm, []managedField{
		{manager: fluxcdv1.FluxOperator, operation: metav1.ManagedFieldsOperationApply},
		{manager: fluxcdv1.FluxHelmController, operation: metav1.ManagedFieldsOperationApply},
		// helm-controller also writes Update entries; both variants must be stripped.
		{manager: fluxcdv1.FluxHelmController, operation: metav1.ManagedFieldsOperationUpdate},
		{manager: fluxcdv1.FluxKustomizeController, operation: metav1.ManagedFieldsOperationApply},
		{manager: kubectlFieldManager, operation: metav1.ManagedFieldsOperationUpdate},
	})

	rs := newResourceSet(ns.Name)
	g.Expect(testClient.Create(ctx, rs)).To(Succeed())
	setResourceSetInventory(ctx, g, rs, []fluxcdv1.ResourceRef{{
		ID: inventoryIDForConfigMap(ns.Name, cmName), Version: "v1",
	}})

	kubeconfigArgs.Namespace = &ns.Name
	output, err := executeCommand([]string{"migrate", "owner", "rset/owner"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("stripped managers"))
	g.Expect(output).To(ContainSubstring(fluxcdv1.FluxHelmController))
	g.Expect(output).To(ContainSubstring(fluxcdv1.FluxKustomizeController))
	g.Expect(output).To(ContainSubstring("cleaned 1/1 resources"))

	cm = &corev1.ConfigMap{}
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: cmName}, cm)).To(Succeed())
	for _, e := range cm.GetManagedFields() {
		g.Expect(e.Manager).ToNot(Equal(fluxcdv1.FluxHelmController))
		g.Expect(e.Manager).ToNot(Equal(fluxcdv1.FluxKustomizeController))
	}
	remaining := configMapManagers(ctx, g, ns.Name, cmName)
	g.Expect(remaining).To(ContainElement(fluxcdv1.FluxOperator))
	g.Expect(remaining).To(ContainElement(kubectlFieldManager))
}

func TestMigrateOwnerCmd_FluxInstance(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-fi")
	g.Expect(err).ToNot(HaveOccurred())

	cmName := "fi-config"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: ns.Name},
	}
	g.Expect(testClient.Create(ctx, cm)).To(Succeed())
	seedManagedFields(ctx, g, cm, applyFields(fluxcdv1.FluxOperator, fluxcdv1.FluxKustomizeController))

	fi := newFluxInstance(ns.Name)
	g.Expect(testClient.Create(ctx, fi)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(context.Background(), fi) })
	fi.Status.Inventory = &fluxcdv1.ResourceInventory{Entries: []fluxcdv1.ResourceRef{{
		ID: inventoryIDForConfigMap(ns.Name, cmName), Version: "v1",
	}}}
	g.Expect(testClient.Status().Update(ctx, fi)).To(Succeed())

	kubeconfigArgs.Namespace = &ns.Name
	output, err := executeCommand([]string{"migrate", "owner", "fluxinstance/flux"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("stripped managers"))

	remaining := configMapManagers(ctx, g, ns.Name, cmName)
	g.Expect(remaining).To(ContainElement(fluxcdv1.FluxOperator))
	g.Expect(remaining).ToNot(ContainElement(fluxcdv1.FluxKustomizeController))
}

func TestMigrateOwnerCmd_AlreadyClean(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-clean")
	g.Expect(err).ToNot(HaveOccurred())

	cmName := "clean"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: ns.Name},
	}
	g.Expect(testClient.Create(ctx, cm)).To(Succeed())
	seedManagedFields(ctx, g, cm, applyFields(fluxcdv1.FluxOperator, kubectlFieldManager))

	rs := newResourceSet(ns.Name)
	g.Expect(testClient.Create(ctx, rs)).To(Succeed())
	setResourceSetInventory(ctx, g, rs, []fluxcdv1.ResourceRef{{
		ID: inventoryIDForConfigMap(ns.Name, cmName), Version: "v1",
	}})

	kubeconfigArgs.Namespace = &ns.Name
	output, err := executeCommand([]string{"migrate", "owner", "rset/owner"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("already clean"))

	remaining := configMapManagers(ctx, g, ns.Name, cmName)
	g.Expect(remaining).To(ContainElement(fluxcdv1.FluxOperator))
	g.Expect(remaining).To(ContainElement(kubectlFieldManager))
}

func TestMigrateOwnerCmd_DryRun(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-dryrun")
	g.Expect(err).ToNot(HaveOccurred())

	cmName := "dry"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: ns.Name},
	}
	g.Expect(testClient.Create(ctx, cm)).To(Succeed())
	seedManagedFields(ctx, g, cm, applyFields(fluxcdv1.FluxOperator, fluxcdv1.FluxHelmController))

	rs := newResourceSet(ns.Name)
	g.Expect(testClient.Create(ctx, rs)).To(Succeed())
	setResourceSetInventory(ctx, g, rs, []fluxcdv1.ResourceRef{{
		ID: inventoryIDForConfigMap(ns.Name, cmName), Version: "v1",
	}})

	kubeconfigArgs.Namespace = &ns.Name
	output, err := executeCommand([]string{"migrate", "owner", "rset/owner", "--dry-run"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("would strip managers"))
	g.Expect(output).To(ContainSubstring(fluxcdv1.FluxHelmController))
	g.Expect(output).To(ContainSubstring("1/1 resources need cleanup"))

	// Dry-run must not mutate the target.
	remaining := configMapManagers(ctx, g, ns.Name, cmName)
	g.Expect(remaining).To(ContainElement(fluxcdv1.FluxHelmController))
}

func TestMigrateOwnerCmd_FreesOrphanedDataKeys(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "migrate-owner-data")
	g.Expect(err).ToNot(HaveOccurred())

	cmName := "app-data"

	// flux-kustomize-controller applies the ConfigMap with two data keys.
	ksApply := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"kept":   "v1",
			"orphan": "v2",
		},
	}
	g.Expect(testClient.Patch(ctx, ksApply, client.Apply,
		client.ForceOwnership, client.FieldOwner(fluxcdv1.FluxKustomizeController))).To(Succeed())

	// flux-operator takes ownership of both keys via a force apply so the
	// new owner is authoritative before migrate runs.
	foApply := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"kept":   "v1",
			"orphan": "v2",
		},
	}
	g.Expect(testClient.Patch(ctx, foApply, client.Apply,
		client.ForceOwnership, client.FieldOwner(fluxcdv1.FluxOperator))).To(Succeed())

	rs := newResourceSet(ns.Name)
	g.Expect(testClient.Create(ctx, rs)).To(Succeed())
	setResourceSetInventory(ctx, g, rs, []fluxcdv1.ResourceRef{{
		ID: inventoryIDForConfigMap(ns.Name, cmName), Version: "v1",
	}})

	kubeconfigArgs.Namespace = &ns.Name
	_, err = executeCommand([]string{"migrate", "owner", "rset/owner"})
	g.Expect(err).ToNot(HaveOccurred())

	// After the stale manager is stripped and the current owner reapplies
	// its desired state, the orphan data key must be deleted.
	foReapply := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ns.Name,
		},
		Data: map[string]string{"kept": "v1"},
	}
	g.Expect(testClient.Patch(ctx, foReapply, client.Apply,
		client.ForceOwnership, client.FieldOwner(fluxcdv1.FluxOperator))).To(Succeed())

	cm := &corev1.ConfigMap{}
	g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: cmName}, cm)).To(Succeed())
	g.Expect(cm.Data).To(HaveLen(1))
	g.Expect(cm.Data).To(HaveKeyWithValue("kept", "v1"))
	g.Expect(cm.Data).ToNot(HaveKey("orphan"))
}

func newResourceSet(namespace string) *fluxcdv1.ResourceSet {
	return &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: namespace},
		Spec: fluxcdv1.ResourceSetSpec{
			ResourcesTemplate: `apiVersion: v1
kind: ConfigMap
metadata:
  name: placeholder`,
		},
	}
}

func newFluxInstance(namespace string) *fluxcdv1.FluxInstance {
	return &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "flux", Namespace: namespace},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}
}

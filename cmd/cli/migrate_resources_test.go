// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	migrateTestGroup    = "migrate.example.com"
	migrateTestKind     = "Widget"
	migrateTestPlural   = "widgets"
	migrateTestCRDName  = migrateTestPlural + "." + migrateTestGroup
	migrateTestOldVer   = "v1beta1"
	migrateTestNewVer   = "v1"
	migrateFieldManager = "migrate-test"
)

func installMigrateTestCRD(ctx context.Context, g *WithT) func() {
	preserve := true
	schemaProps := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {
				Type:                   "object",
				XPreserveUnknownFields: &preserve,
			},
		},
	}
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: migrateTestCRDName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: migrateTestGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   migrateTestPlural,
				Singular: "widget",
				Kind:     migrateTestKind,
				ListKind: migrateTestKind + "List",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    migrateTestOldVer,
					Served:  true,
					Storage: false,
					Schema:  &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: schemaProps},
				},
				{
					Name:    migrateTestNewVer,
					Served:  true,
					Storage: true,
					Schema:  &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: schemaProps},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, crd)).To(Succeed())

	// Wait until the CRD is established and the API is discoverable.
	listGVK := schema.GroupVersionKind{Group: migrateTestGroup, Version: migrateTestNewVer, Kind: migrateTestKind + "List"}
	g.Eventually(func() error {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listGVK)
		return testClient.List(ctx, list)
	}, timeout, "500ms").Should(Succeed())

	return func() {
		_ = testClient.Delete(context.Background(), crd)
	}
}

// createStaleWidget creates a Widget at newVer and then rewrites every
// managed-fields entry's apiVersion to oldVer via a JSON patch, emulating
// a resource whose field managers are still pinned to the old API version.
func createStaleWidget(ctx context.Context, g *WithT, namespace, name string) {
	widget := &unstructured.Unstructured{}
	widget.SetGroupVersionKind(schema.GroupVersionKind{Group: migrateTestGroup, Version: migrateTestNewVer, Kind: migrateTestKind})
	widget.SetNamespace(namespace)
	widget.SetName(name)
	g.Expect(unstructured.SetNestedField(widget.Object, "bar", "spec", "foo")).To(Succeed())

	g.Expect(testClient.Patch(ctx, widget, client.Apply,
		client.ForceOwnership, client.FieldOwner(migrateFieldManager))).To(Succeed())

	// Rewrite every managed-fields entry's apiVersion to the old version
	// to simulate a stale state.
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(widget), widget)).To(Succeed())
	entries := widget.GetManagedFields()
	g.Expect(entries).ToNot(BeEmpty())
	oldAPIVersion := migrateTestGroup + "/" + migrateTestOldVer
	for i := range entries {
		entries[i].APIVersion = oldAPIVersion
	}
	widget.SetManagedFields(entries)
	g.Expect(testClient.Update(ctx, widget)).To(Succeed())
}

func getWidgetManagedFieldsVersions(ctx context.Context, g *WithT, namespace, name string) []string {
	widget := &unstructured.Unstructured{}
	widget.SetGroupVersionKind(schema.GroupVersionKind{Group: migrateTestGroup, Version: migrateTestNewVer, Kind: migrateTestKind})
	widget.SetNamespace(namespace)
	widget.SetName(name)
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(widget), widget)).To(Succeed())
	versions := make([]string, 0, len(widget.GetManagedFields()))
	for _, e := range widget.GetManagedFields() {
		versions = append(versions, e.APIVersion)
	}
	return versions
}

func TestMigrateResourcesCmd(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cleanup := installMigrateTestCRD(ctx, g)
	t.Cleanup(cleanup)

	newAPIVersion := migrateTestGroup + "/" + migrateTestNewVer
	oldAPIVersion := migrateTestGroup + "/" + migrateTestOldVer

	t.Run("missing --api-version", func(t *testing.T) {
		g := NewWithT(t)
		_, err := executeCommand([]string{"migrate", "resources", "--kind", migrateTestKind})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("--api-version is required"))
	})

	t.Run("missing --kind", func(t *testing.T) {
		g := NewWithT(t)
		_, err := executeCommand([]string{"migrate", "resources", "--api-version", newAPIVersion})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("--kind is required"))
	})

	t.Run("unknown kind", func(t *testing.T) {
		g := NewWithT(t)
		_, err := executeCommand([]string{
			"migrate", "resources",
			"--api-version", newAPIVersion,
			"--kind", "NotAKind",
			"-A",
		})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("version not in storedVersions", func(t *testing.T) {
		g := NewWithT(t)
		_, err := executeCommand([]string{
			"migrate", "resources",
			"--api-version", migrateTestGroup + "/v9",
			"--kind", migrateTestKind,
			"-A",
		})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("no resources found", func(t *testing.T) {
		g := NewWithT(t)
		ns, err := testEnv.CreateNamespace(ctx, "migrate-empty")
		g.Expect(err).ToNot(HaveOccurred())
		kubeconfigArgs.Namespace = &ns.Name
		_, err = executeCommand([]string{
			"migrate", "resources",
			"--api-version", newAPIVersion,
			"--kind", migrateTestKind,
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no resources of kind"))
	})

	t.Run("dry-run reports but does not patch", func(t *testing.T) {
		g := NewWithT(t)
		ns, err := testEnv.CreateNamespace(ctx, "migrate-dryrun")
		g.Expect(err).ToNot(HaveOccurred())
		kubeconfigArgs.Namespace = &ns.Name
		createStaleWidget(ctx, g, ns.Name, "widget-a")

		output, err := executeCommand([]string{
			"migrate", "resources",
			"--api-version", newAPIVersion,
			"--kind", migrateTestKind,
			"--dry-run",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(output).To(ContainSubstring("widget-a needs migration"))
		g.Expect(output).To(ContainSubstring("1/1 resources need migration"))

		// Verify the server-side managed fields were NOT rewritten.
		versions := getWidgetManagedFieldsVersions(ctx, g, ns.Name, "widget-a")
		g.Expect(versions).To(ContainElement(oldAPIVersion))
		g.Expect(versions).ToNot(ContainElement(newAPIVersion))
	})

	t.Run("migrates stale resources in namespace", func(t *testing.T) {
		g := NewWithT(t)
		ns, err := testEnv.CreateNamespace(ctx, "migrate-ns")
		g.Expect(err).ToNot(HaveOccurred())
		kubeconfigArgs.Namespace = &ns.Name
		createStaleWidget(ctx, g, ns.Name, "widget-b")

		output, err := executeCommand([]string{
			"migrate", "resources",
			"--api-version", newAPIVersion,
			"--kind", migrateTestKind,
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(output).To(ContainSubstring("widget-b migrated"))
		g.Expect(output).To(ContainSubstring("migrated 1/1 resources"))

		versions := getWidgetManagedFieldsVersions(ctx, g, ns.Name, "widget-b")
		g.Expect(versions).ToNot(BeEmpty())
		for _, v := range versions {
			g.Expect(v).To(Equal(newAPIVersion))
		}
	})

	t.Run("migrates across all namespaces", func(t *testing.T) {
		g := NewWithT(t)
		nsA, err := testEnv.CreateNamespace(ctx, "migrate-all-a")
		g.Expect(err).ToNot(HaveOccurred())
		nsB, err := testEnv.CreateNamespace(ctx, "migrate-all-b")
		g.Expect(err).ToNot(HaveOccurred())
		createStaleWidget(ctx, g, nsA.Name, "widget-a")
		createStaleWidget(ctx, g, nsB.Name, "widget-b")

		output, err := executeCommand([]string{
			"migrate", "resources",
			"--api-version", newAPIVersion,
			"--kind", migrateTestKind,
			"-A",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(output).To(ContainSubstring("widget-a migrated"))
		g.Expect(output).To(ContainSubstring("widget-b migrated"))

		for _, n := range []struct{ ns, name string }{{nsA.Name, "widget-a"}, {nsB.Name, "widget-b"}} {
			versions := getWidgetManagedFieldsVersions(ctx, g, n.ns, n.name)
			for _, v := range versions {
				g.Expect(v).To(Equal(newAPIVersion))
			}
		}
	})
}

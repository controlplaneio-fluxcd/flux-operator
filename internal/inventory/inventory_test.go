// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inventory

import (
	"os"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/cli-utils/pkg/object"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func Test_Inventory(t *testing.T) {
	g := NewWithT(t)

	set1, err := readManifest("testdata/inventory1.yaml")
	if err != nil {
		t.Fatal(err)
	}

	inv1 := New()
	err = AddChangeSet(inv1, set1)
	g.Expect(err).ToNot(HaveOccurred())

	set2, err := readManifest("testdata/inventory2.yaml")
	if err != nil {
		t.Fatal(err)
	}

	inv2 := New()
	err = AddChangeSet(inv2, set2)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("lists objects in inventory", func(t *testing.T) {
		unList, err := List(inv1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(len(unList)).To(BeIdenticalTo(len(inv1.Entries)))

		mList, err := ListMetadata(inv1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(len(mList)).To(BeIdenticalTo(len(inv1.Entries)))
	})

	t.Run("diff objects in inventory", func(t *testing.T) {
		unList, err := Diff(inv2, inv1)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(len(unList)).To(BeIdenticalTo(1))
		g.Expect(unList[0].GetName()).To(BeIdenticalTo("test2"))
	})
}

func Test_Merge(t *testing.T) {
	base := &fluxcdv1.ResourceInventory{
		Entries: []fluxcdv1.ResourceRef{
			{ID: "default_test2__ConfigMap", Version: "v1"},
			{ID: "default_test1__ConfigMap", Version: "v1"},
		},
	}
	overlay := &fluxcdv1.ResourceInventory{
		Entries: []fluxcdv1.ResourceRef{
			{ID: "default_test3_apps_Deployment", Version: "v1"},
			{ID: "default_test1__ConfigMap", Version: "v2"},
		},
	}

	t.Run("dedups entries with overlay version precedence sorted by ID", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(base, overlay)
		g.Expect(result.Entries).To(Equal([]fluxcdv1.ResourceRef{
			{ID: "default_test1__ConfigMap", Version: "v2"},
			{ID: "default_test2__ConfigMap", Version: "v1"},
			{ID: "default_test3_apps_Deployment", Version: "v1"},
		}))
	})

	t.Run("treats nil base as empty", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(nil, overlay)
		g.Expect(result.Entries).To(Equal([]fluxcdv1.ResourceRef{
			{ID: "default_test1__ConfigMap", Version: "v2"},
			{ID: "default_test3_apps_Deployment", Version: "v1"},
		}))
	})

	t.Run("treats nil overlay as empty", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(base, nil)
		g.Expect(result.Entries).To(Equal([]fluxcdv1.ResourceRef{
			{ID: "default_test1__ConfigMap", Version: "v1"},
			{ID: "default_test2__ConfigMap", Version: "v1"},
		}))
	})

	t.Run("returns empty inventory for nil inputs", func(t *testing.T) {
		g := NewWithT(t)

		result := Merge(nil, nil)
		g.Expect(result.Entries).To(BeEmpty())
	})
}

func readManifest(manifest string) (*ssa.ChangeSet, error) {
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}

	objects, err := ssautil.ReadObjects(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}

	cs := ssa.NewChangeSet()

	for _, o := range objects {
		cse := ssa.ChangeSetEntry{
			ObjMetadata:  object.UnstructuredToObjMetadata(o),
			GroupVersion: o.GroupVersionKind().Version,
			Subject:      ssautil.FmtUnstructured(o),
			Action:       ssa.CreatedAction,
		}
		cs.Add(cse)
	}

	return cs, nil
}

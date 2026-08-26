// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/wI2L/jsondiff"
)

func TestRenderDiffStatesAndSummary(t *testing.T) {
	g := NewWithT(t)
	output, err := renderDiff(diffResult{
		Owner:         &DiffOwnerRef{Kind: "Kustomization", Namespace: "apps", Name: "backend"},
		FieldManager:  "kustomize-controller",
		PruneEnabled:  true,
		FutureSuspend: true,
		Objects: []diffObjectResult{
			{Subject: "ConfigMap/apps/new", State: diffStateCreate},
			{Subject: "Deployment/apps/backend", State: diffStateUpdate, Patch: jsondiff.Patch{
				{Type: jsondiff.OperationReplace, Path: "/spec/replicas", Value: float64(3)},
			}},
			{Subject: "StatefulSet/apps/db", State: diffStateRecreate, Detail: "forced, not validated"},
			{Subject: "Service/apps/backend", State: diffStateUnchanged},
			{Subject: "Job/apps/migrate", State: diffStateSkipped, Detail: "kustomize.toolkit.fluxcd.io/ssa: ignore"},
			{Subject: "CustomThing/apps/foo", State: diffStateError, Detail: "no matches for kind"},
			{Subject: "Deployment/apps/legacy", State: diffStateUpdate,
				Hint: &DiffOwnerRef{Kind: "Kustomization", Namespace: "flux-system", Name: "legacy"}},
		},
		PruneObjects: []diffObjectResult{{Subject: "ConfigMap/apps/old", State: diffStateDelete}},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Diff for Kustomization/apps/backend (field manager: kustomize-controller, prune: enabled)"))
	g.Expect(output).To(ContainSubstring("suspended: changes are not applied until resumed"))
	g.Expect(output).To(ContainSubstring("- op: replace\n  path: /spec/replicas\n  value: 3"))
	g.Expect(output).To(ContainSubstring("StatefulSet/apps/db recreate (forced, not validated)"))
	g.Expect(output).To(ContainSubstring("Job/apps/migrate skipped (kustomize.toolkit.fluxcd.io/ssa: ignore)"))
	g.Expect(output).To(ContainSubstring("CustomThing/apps/foo error: no matches for kind"))
	g.Expect(output).To(ContainSubstring("currently managed by Kustomization/flux-system/legacy"))
	g.Expect(output).To(ContainSubstring("Not in the manifest (pruned if the manifest is complete):"))
	g.Expect(output).To(ContainSubstring("Summary: 1 create, 2 update, 1 recreate, 1 unchanged, 1 skipped, 1 delete, 1 error"))
}

func TestRenderDiffNoChangesAndResume(t *testing.T) {
	g := NewWithT(t)
	output, err := renderDiff(diffResult{
		Owner:        &DiffOwnerRef{Kind: "HelmRelease", Namespace: "flux-system", Name: "app"},
		FieldManager: "helm-controller",
		PruneEnabled: true,
		LiveSuspend:  true,
		Objects:      []diffObjectResult{{Subject: "Service/apps/app", State: diffStateUnchanged}},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("currently suspended; the proposed definition resumes it"))
	g.Expect(output).To(ContainSubstring("No changes detected"))
	g.Expect(output).NotTo(ContainSubstring("Summary:"))
}

func TestRenderDiffPruneStatuses(t *testing.T) {
	g := NewWithT(t)
	for _, status := range []string{"prune: unavailable", "prune: skipped (owner not in cluster)"} {
		output, err := renderDiff(diffResult{
			Owner:        &DiffOwnerRef{Kind: "ResourceSet", Namespace: "flux-system", Name: "apps"},
			FieldManager: "flux-operator",
			PruneEnabled: true,
			Objects:      []diffObjectResult{{Subject: "Service/apps/app", State: diffStateUnchanged}},
			PruneStatus:  status,
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(ContainSubstring(status))
	}
}

func TestRenderDiffDependencyWarningsWithoutOwner(t *testing.T) {
	g := NewWithT(t)
	output, err := renderDiff(diffResult{
		FieldManager: "kubectl-flux-mcp",
		Warnings: []string{
			"namespace apps-qa does not exist in the cluster, objects in it are not validated",
			"CRD for example.com/CustomThing does not exist in the cluster, objects of that kind are not validated",
		},
		Objects: []diffObjectResult{
			{Subject: "ConfigMap/apps-qa/backend", State: diffStateCreate,
				Detail: "not validated: namespace apps-qa does not exist yet"},
			{Subject: "CustomThing/example", State: diffStateCreate,
				Detail: "not validated: CRD for example.com/CustomThing does not exist yet"},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Diff for Kubernetes manifest"))
	g.Expect(output).To(ContainSubstring("namespace apps-qa does not exist in the cluster, objects in it are not validated"))
	g.Expect(output).To(ContainSubstring("CRD for example.com/CustomThing does not exist in the cluster, objects of that kind are not validated"))
	g.Expect(output).To(ContainSubstring("ConfigMap/apps-qa/backend create (not validated: namespace apps-qa does not exist yet)"))
	g.Expect(output).To(ContainSubstring("CustomThing/example create (not validated: CRD for example.com/CustomThing does not exist yet)"))
}

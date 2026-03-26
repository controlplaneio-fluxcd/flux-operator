// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"testing"
)

func TestFluxGroupFor(t *testing.T) {
	testCases := []struct {
		kind          string
		expectedGroup string
	}{
		// Flux Operator kinds
		{FluxInstanceKind, GroupVersion.Group},
		{FluxReportKind, GroupVersion.Group},
		{ResourceSetKind, GroupVersion.Group},
		{ResourceSetInputProviderKind, GroupVersion.Group},
		// Source kinds
		{FluxGitRepositoryKind, FluxSourceGroup},
		{FluxOCIRepositoryKind, FluxSourceGroup},
		{FluxBucketKind, FluxSourceGroup},
		{FluxHelmChartKind, FluxSourceGroup},
		{FluxHelmRepositoryKind, FluxSourceGroup},
		{FluxExternalArtifactKind, FluxSourceGroup},
		// Notification kinds
		{FluxAlertKind, FluxNotificationGroup},
		{FluxAlertProviderKind, FluxNotificationGroup},
		{FluxReceiverKind, FluxNotificationGroup},
		// Image kinds
		{FluxImageRepositoryKind, FluxImageGroup},
		{FluxImagePolicyKind, FluxImageGroup},
		{FluxImageUpdateAutomationKind, FluxImageGroup},
		// Kustomize kind
		{FluxKustomizationKind, FluxKustomizeGroup},
		// Helm kind
		{FluxHelmReleaseKind, FluxHelmGroup},
		// Source extensions kind
		{FluxArtifactGeneratorKind, FluxSourceExtensionsGroup},
	}

	for _, tc := range testCases {
		t.Run(tc.kind, func(t *testing.T) {
			gk, err := FluxGroupFor(tc.kind)
			if err != nil {
				t.Fatalf("unexpected error for kind %s: %v", tc.kind, err)
			}
			if gk.Group != tc.expectedGroup {
				t.Errorf("expected group %s, got %s", tc.expectedGroup, gk.Group)
			}
			if gk.Kind != tc.kind {
				t.Errorf("expected kind %s, got %s", tc.kind, gk.Kind)
			}
		})
	}
}

func TestFluxGroupFor_UnknownKind(t *testing.T) {
	unknownKinds := []string{
		"Deployment",
		"Service",
		"UnknownKind",
		"",
	}

	for _, kind := range unknownKinds {
		t.Run(kind, func(t *testing.T) {
			_, err := FluxGroupFor(kind)
			if err == nil {
				t.Error("expected error for unknown kind, got nil")
			}
		})
	}
}

func TestFindFluxKindInfo_ExactMatch(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Kustomization", FluxKustomizationKind},
		{"HelmRelease", FluxHelmReleaseKind},
		{"GitRepository", FluxGitRepositoryKind},
		{"ResourceSet", ResourceSetKind},
		{"FluxInstance", FluxInstanceKind},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			info, err := FindFluxKindInfo(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Name != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, info.Name)
			}
		})
	}
}

func TestFindFluxKindInfo_CaseInsensitive(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"kustomization", FluxKustomizationKind},
		{"KUSTOMIZATION", FluxKustomizationKind},
		{"KuStOmIzAtIoN", FluxKustomizationKind},
		{"helmrelease", FluxHelmReleaseKind},
		{"HELMRELEASE", FluxHelmReleaseKind},
		{"gitrepository", FluxGitRepositoryKind},
		{"resourceset", ResourceSetKind},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			info, err := FindFluxKindInfo(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Name != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, info.Name)
			}
		})
	}
}

func TestFindFluxKindInfo_ShortName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"ks", FluxKustomizationKind},
		{"hr", FluxHelmReleaseKind},
		{"gitrepo", FluxGitRepositoryKind},
		{"rset", ResourceSetKind},
		{"instance", FluxInstanceKind},
		{"hc", FluxHelmChartKind},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			info, err := FindFluxKindInfo(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Name != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, info.Name)
			}
		})
	}
}

func TestFindFluxKindInfo_ShortNameCaseInsensitive(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"KS", FluxKustomizationKind},
		{"Ks", FluxKustomizationKind},
		{"HR", FluxHelmReleaseKind},
		{"Hr", FluxHelmReleaseKind},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			info, err := FindFluxKindInfo(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Name != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, info.Name)
			}
		})
	}
}

func TestFindFluxKindInfo_NotFound(t *testing.T) {
	testCases := []string{
		"UnknownKind",
		"Deployment",
		"Service",
		"",
		"xyz",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			_, err := FindFluxKindInfo(tc)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestFindFluxKindInfo_ReturnsCorrectPlural(t *testing.T) {
	testCases := []struct {
		input          string
		expectedPlural string
	}{
		{"Kustomization", "kustomizations"},
		{"HelmRelease", "helmreleases"},
		{"GitRepository", "gitrepositories"},
		{"ResourceSet", "resourcesets"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			info, err := FindFluxKindInfo(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Plural != tc.expectedPlural {
				t.Errorf("expected %s, got %s", tc.expectedPlural, info.Plural)
			}
		})
	}
}

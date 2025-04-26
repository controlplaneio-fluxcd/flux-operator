// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package client

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParseGroupVersionKind(t *testing.T) {
	kubeClient := KubeClient{
		Client: fake.NewClientBuilder().WithScheme(NewTestScheme()).Build(),
	}

	tests := []struct {
		name       string
		apiVersion string
		kind       string
		result     string
		matchErr   string
	}{
		{
			name:       "valid inputs",
			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "ResourceSet",
			result:     "fluxcd.controlplane.io/v1, Kind=ResourceSet",
		},
		{
			name:       "invalid api version",
			apiVersion: "helm.toolkit.fluxcd.io/v1/v2",
			kind:       "HelmRelease",
			matchErr:   "unexpected",
		},
		{
			name:       "invalid kind",
			apiVersion: "fluxcd.controlplane.io/v1",
			matchErr:   "not specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			gvk, err := kubeClient.ParseGroupVersionKind(tt.apiVersion, tt.kind)
			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gvk.String()).To(Equal(tt.result))
			}
		})
	}

}

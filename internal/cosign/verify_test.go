// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package cosign_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/cosign"
)

func TestVerifyArtifact(t *testing.T) {
	ghActionsIdentity := `^https://github\.com/stefanprodan/podinfo/.*$`
	ghActionsIssuer := "https://token.actions.githubusercontent.com"

	t.Run("verifies container image signed with GH Actions", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			ghActionsIdentity,
			ghActionsIssuer,
		)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("verifies Helm chart signed with GH Actions", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/charts/podinfo:6.11.0",
			ghActionsIdentity,
			ghActionsIssuer,
		)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("fails with wrong identity", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			`^wrong-identity@example\.com$`,
			ghActionsIssuer,
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("signature verification failed"))
	})

	t.Run("fails with wrong issuer", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			ghActionsIdentity,
			"https://wrong-issuer.example.com",
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("signature verification failed"))
	})

	t.Run("fails for invalid reference", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"oci://invalid:ref:with:too:many:colons",
			cosign.DefaultCertIdentityRegexp,
			cosign.DefaultCertOIDCIssuer,
		)
		g.Expect(err).To(HaveOccurred())
	})
}

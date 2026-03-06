// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package cosign_test

import (
	"context"
	"os"
	"path/filepath"
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
			"",
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
			"",
		)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("verifies with trusted root file", func(t *testing.T) {
		g := NewWithT(t)

		// Fetch the trusted root from TUF and write it to a temp file.
		trustedRootPath := fetchTrustedRoot(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			ghActionsIdentity,
			ghActionsIssuer,
			trustedRootPath,
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
			"",
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
			"",
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("signature verification failed"))
	})

	t.Run("fails with empty identity", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			"",
			ghActionsIssuer,
			"",
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("certificate identity regexp must not be empty"))
	})

	t.Run("fails with empty issuer", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			ghActionsIdentity,
			"",
			"",
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("certificate OIDC issuer must not be empty"))
	})

	t.Run("fails with invalid trusted root path", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"ghcr.io/stefanprodan/podinfo:6.11.0",
			ghActionsIdentity,
			ghActionsIssuer,
			"/nonexistent/trusted_root.json",
		)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("loading trusted root"))
	})

	t.Run("fails for invalid reference", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := cosign.VerifyArtifact(ctx,
			"oci://invalid:ref:with:too:many:colons",
			cosign.DefaultCertIdentityRegexp,
			cosign.DefaultCertOIDCIssuer,
			"",
		)
		g.Expect(err).To(HaveOccurred())
	})
}

// fetchTrustedRoot fetches the Sigstore trusted root from TUF and writes
// it to a temporary file, returning the file path.
func fetchTrustedRoot(t *testing.T) string {
	t.Helper()
	g := NewWithT(t)

	tufClient, err := cosign.NewTUFClient()
	g.Expect(err).ToNot(HaveOccurred())

	trustedRootJSON, err := tufClient.GetTarget("trusted_root.json")
	g.Expect(err).ToNot(HaveOccurred())

	path := filepath.Join(t.TempDir(), "trusted_root.json")
	err = os.WriteFile(path, trustedRootJSON, 0o600)
	g.Expect(err).ToNot(HaveOccurred())

	return path
}

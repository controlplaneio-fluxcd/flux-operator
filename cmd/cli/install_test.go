// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
)

func TestPrepareOperatorManifests_DigestPinnedHandoff(t *testing.T) {
	const (
		mutableRef = "oci://ghcr.io/example/manifests:latest"
		pinnedRef  = "oci://ghcr.io/example/manifests@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	manifest := []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: flux-system\n")

	originalVerify := verifyInstallArtifact
	originalResolve := resolveInstallArtifact
	originalDownload := downloadInstallArtifact
	originalArgs := installArgs
	defer func() {
		verifyInstallArtifact = originalVerify
		resolveInstallArtifact = originalResolve
		downloadInstallArtifact = originalDownload
		installArgs = originalArgs
	}()

	t.Run("verified reference is downloaded", func(t *testing.T) {
		g := NewWithT(t)
		installArgs.verify = true
		installArgs.certIdentityRegexp = "identity"
		installArgs.certOIDCIssuer = "issuer"

		verifyInstallArtifact = func(_ context.Context, ref, identity, issuer, _ string, _ authn.Keychain) (string, error) {
			g.Expect(ref).To(Equal(mutableRef))
			g.Expect(identity).To(Equal("identity"))
			g.Expect(issuer).To(Equal("issuer"))
			return pinnedRef, nil
		}
		resolveInstallArtifact = func(context.Context, string, authn.Keychain) (string, error) {
			return "", errors.New("resolver must not run after verification")
		}
		downloadInstallArtifact = func(_ context.Context, ref, path string, _ authn.Keychain) ([]byte, error) {
			g.Expect(ref).To(Equal(pinnedRef))
			g.Expect(path).To(Equal("flux-operator/install.yaml"))
			return manifest, nil
		}

		objects, err := prepareOperatorManifests(context.Background(), mutableRef)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(objects).To(HaveLen(1))
	})

	t.Run("verification failure prevents download", func(t *testing.T) {
		g := NewWithT(t)
		installArgs.verify = true
		downloadCalled := false
		verifyInstallArtifact = func(context.Context, string, string, string, string, authn.Keychain) (string, error) {
			return "", errors.New("invalid signature")
		}
		downloadInstallArtifact = func(context.Context, string, string, authn.Keychain) ([]byte, error) {
			downloadCalled = true
			return nil, nil
		}

		_, err := prepareOperatorManifests(context.Background(), mutableRef)
		g.Expect(err).To(MatchError(ContainSubstring("invalid signature")))
		g.Expect(downloadCalled).To(BeFalse())
	})

	t.Run("unverified install resolves before download", func(t *testing.T) {
		g := NewWithT(t)
		installArgs.verify = false
		resolveInstallArtifact = func(_ context.Context, ref string, _ authn.Keychain) (string, error) {
			g.Expect(ref).To(Equal(mutableRef))
			return pinnedRef, nil
		}
		downloadInstallArtifact = func(_ context.Context, ref, _ string, _ authn.Keychain) ([]byte, error) {
			g.Expect(ref).To(Equal(pinnedRef))
			return manifest, nil
		}

		_, err := prepareOperatorManifests(context.Background(), mutableRef)
		g.Expect(err).NotTo(HaveOccurred())
	})
}

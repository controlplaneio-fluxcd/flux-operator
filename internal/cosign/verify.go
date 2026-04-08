// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package cosign

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	// DefaultCertIdentityRegexp is the default certificate identity regexp
	// matching the Flux Operator GitHub Organization.
	DefaultCertIdentityRegexp = `^https://github\.com/controlplaneio-fluxcd/.*$`

	// DefaultCertOIDCIssuer is the default OIDC issuer for GitHub Actions.
	DefaultCertOIDCIssuer = "https://token.actions.githubusercontent.com"

	// sigstoreBundleMediaTypePrefix is the common prefix for all sigstore bundle media types.
	sigstoreBundleMediaTypePrefix = "application/vnd.dev.sigstore.bundle"
)

// VerifyArtifact verifies the cosign signature on an OCI artifact using Sigstore's
// public good instance (Fulcio + Rekor). It checks that the signing certificate
// matches the given identity regexp and OIDC issuer.
// The verification process is compatible with cosign v3's default keyless
// verification and requires a minimum sigstore bundle version of v0.3.
// When trustedRootPath is set, the trusted root is loaded from the given file
// and TUF is bypassed entirely, enabling offline verification with no network
// calls beyond the OCI registry.
// When keychain is nil, authn.DefaultKeychain is used to resolve registry
// credentials for fetching the artifact descriptor, referrers index, and
// sigstore bundle.
func VerifyArtifact(ctx context.Context, ociRef string, certIdentityRegexp string, certOIDCIssuer string, trustedRootPath string, keychain authn.Keychain) error {
	if certIdentityRegexp == "" {
		return fmt.Errorf("certificate identity regexp must not be empty")
	}
	if certOIDCIssuer == "" {
		return fmt.Errorf("certificate OIDC issuer must not be empty")
	}
	if keychain == nil {
		keychain = authn.DefaultKeychain
	}

	// Strip oci:// prefix if present.
	ociRef = strings.TrimPrefix(ociRef, "oci://")

	// Resolve the image reference to a digest.
	ref, err := name.ParseReference(ociRef)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", ociRef, err)
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("fetching descriptor for %q: %w", ociRef, err)
	}

	repo := ref.Context()
	digest := repo.Digest(desc.Digest.String())
	remoteOpts := []remote.Option{remote.WithAuthFromKeychain(keychain), remote.WithContext(ctx)}

	// Query referrers for sigstore bundles.
	idx, err := remote.Referrers(digest, remoteOpts...)
	if err != nil {
		return fmt.Errorf("querying referrers for %s: %w", digest, err)
	}

	manifest, err := idx.IndexManifest()
	if err != nil {
		return fmt.Errorf("reading referrers index: %w", err)
	}

	// Find the first sigstore bundle among the referrers.
	bundleBytes, err := findSigstoreBundle(repo, manifest, remoteOpts...)
	if err != nil {
		return err
	}

	// Parse the sigstore bundle and check minimum version.
	var b bundle.Bundle
	if err := b.UnmarshalJSON(bundleBytes); err != nil {
		return fmt.Errorf("parsing sigstore bundle: %w", err)
	}
	if !b.MinVersion("v0.3") {
		return fmt.Errorf("unsupported sigstore bundle version (minimum v0.3 required)")
	}

	// Load the trusted root either from a local file or from TUF.
	trustedRoot, err := loadTrustedRoot(trustedRootPath)
	if err != nil {
		return err
	}

	// Create the verifier with SCT, integrated timestamps, and tlog requirements.
	// This matches cosign v3's default keyless verification options.
	sev, err := verify.NewVerifier(trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithIntegratedTimestamps(1),
		verify.WithTransparencyLog(1),
	)
	if err != nil {
		return fmt.Errorf("creating verifier: %w", err)
	}

	// Create the identity policy.
	certID, err := verify.NewShortCertificateIdentity(certOIDCIssuer, "", "", certIdentityRegexp)
	if err != nil {
		return fmt.Errorf("creating certificate identity: %w", err)
	}

	// Create the artifact digest policy using the resolved image digest.
	digestHex := desc.Digest.Hex
	digestBytes, err := hex.DecodeString(digestHex)
	if err != nil {
		return fmt.Errorf("decoding digest hex: %w", err)
	}

	policy := verify.NewPolicy(
		verify.WithArtifactDigest(desc.Digest.Algorithm, digestBytes),
		verify.WithCertificateIdentity(certID),
	)

	// Verify the bundle against the policy.
	_, err = sev.Verify(&b, policy)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// loadTrustedRoot loads the Sigstore trusted root material. When trustedRootPath
// is set, it loads from the given file (offline mode, no TUF calls). Otherwise,
// it fetches from the Sigstore public good TUF repository using an in-memory
// client to avoid writing cache files to disk.
func loadTrustedRoot(trustedRootPath string) (*root.TrustedRoot, error) {
	if trustedRootPath != "" {
		trustedRoot, err := root.NewTrustedRootFromPath(trustedRootPath)
		if err != nil {
			return nil, fmt.Errorf("loading trusted root from %q: %w", trustedRootPath, err)
		}
		return trustedRoot, nil
	}

	tufClient, err := NewTUFClient()
	if err != nil {
		return nil, fmt.Errorf("creating TUF client: %w", err)
	}

	trustedRootJSON, err := tufClient.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("fetching trusted root: %w", err)
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return nil, fmt.Errorf("parsing trusted root: %w", err)
	}
	return trustedRoot, nil
}

// NewTUFClient creates an in-memory Sigstore TUF client that does not
// write cache files to disk.
func NewTUFClient() (*tuf.Client, error) {
	tufOpts := tuf.DefaultOptions()
	tufOpts.WithDisableLocalCache()
	return tuf.New(tufOpts)
}

// findSigstoreBundle searches through all OCI referrers for a sigstore bundle.
// Following the same approach as cosign v3, it iterates all referrers, fetches
// each one, and checks the layer media type to identify sigstore bundles.
// Non-bundle referrers are silently skipped.
func findSigstoreBundle(repo name.Repository, manifest *v1.IndexManifest, opts ...remote.Option) ([]byte, error) {
	for _, m := range manifest.Manifests {
		ref := repo.Digest(m.Digest.String())
		img, err := remote.Image(ref, opts...)
		if err != nil {
			continue
		}
		layers, err := img.Layers()
		if err != nil || len(layers) != 1 {
			continue
		}
		mediaType, err := layers[0].MediaType()
		if err != nil || !strings.HasPrefix(string(mediaType), sigstoreBundleMediaTypePrefix) {
			continue
		}
		reader, err := layers[0].Uncompressed()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(reader)
		if closeErr := reader.Close(); closeErr != nil {
			return nil, fmt.Errorf("closing bundle layer reader: %w", closeErr)
		}
		if err != nil {
			return nil, fmt.Errorf("reading bundle content: %w", err)
		}
		return data, nil
	}

	return nil, fmt.Errorf("no sigstore bundle found in referrers")
}

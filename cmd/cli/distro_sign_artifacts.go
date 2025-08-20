// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	gcname "github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroSignArtifactsCmd = &cobra.Command{
	Use:   "artifacts",
	Short: "Issue a signed attestation for OCI artifacts",
	Example: `  # Attest artifacts by fetching digests from registry
  flux-operator distro sign artifacts \
  --key-set=/path/to/private.jwks \
  --attestation=fluxcd-v2.6.4.jwt \
  --url=ghcr.io/fluxcd/source-controller:v1.6.2 \
  --url=ghcr.io/fluxcd/kustomize-controller:v1.6.1 \
  --url=ghcr.io/fluxcd/notification-controller:v1.6.0 \
  --url=ghcr.io/fluxcd/helm-controller:v1.3.0 \
  --url=ghcr.io/fluxcd/image-reflector-controller:v0.35.2 \
  --url=ghcr.io/fluxcd/image-automation-controller:v0.41.2

  # Attest artifacts by reading the private key set from env
  export FLUX_DISTRO_PRIVATE_KEY_SET="$(cat /path/to/private.jwks)"
  flux-operator distro sign artifacts \
  -a flux-operator-v0.28.0.jwt \
  -u ghcr.io/controlplaneio-fluxcd/flux-operator:v0.28.0 \
  -u ghcr.io/controlplaneio-fluxcd/flux-operator:v0.28.0-ubi \
  -u ghcr.io/controlplaneio-fluxcd/charts/flux-operator:v0.28.0
`,
	Args: cobra.NoArgs,
	RunE: distroSignArtifactsCmdRun,
}

type distroSignArtifactsFlags struct {
	privateKeySetPath string
	attestationPath   string
	urls              []string
}

var distroSignArtifactsArgs distroSignArtifactsFlags

func init() {
	distroSignArtifactsCmd.Flags().StringVarP(&distroSignArtifactsArgs.privateKeySetPath, "key-set", "k", "",
		"path to the private key set file or /dev/stdin (required)")
	distroSignArtifactsCmd.Flags().StringVarP(&distroSignArtifactsArgs.attestationPath, "attestation", "a", "",
		"path to the output file for the attestation (required)")
	distroSignArtifactsCmd.Flags().StringSliceVarP(&distroSignArtifactsArgs.urls, "url", "u", nil,
		"OCI artifact URLs to sign (required, can be specified multiple times)")
	distroSignCmd.AddCommand(distroSignArtifactsCmd)
}

func distroSignArtifactsCmdRun(cmd *cobra.Command, args []string) error {
	if distroSignArtifactsArgs.attestationPath == "" {
		return fmt.Errorf("--attestation flag is required")
	}
	if len(distroSignArtifactsArgs.urls) == 0 {
		return fmt.Errorf("--url flag is required, specify one or more OCI artifact URLs")
	}

	// Read the JWKS from file or environment variable
	jwksData, err := loadKeySet(distroSignArtifactsArgs.privateKeySetPath, distroPrivateKeySetEnvVar)
	if err != nil {
		return err
	}

	// Parse the JWKS data and extract the private key
	pk, err := lkm.EdPrivateKeyFromSet(jwksData)
	if err != nil {
		return err
	}

	// Process URLs to collect artifact digests
	var digests []string
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	rootCmd.Println("processing artifacts:")
	for _, url := range distroSignArtifactsArgs.urls {
		// First check if URL already contains a digest
		if digest, err := hasArtifactDigest(url); err == nil {
			rootCmd.Printf("  %s -> %s (from URL)\n", strings.Split(url, "@")[0], digest)
			digests = append(digests, digest)
		} else {
			// If no digest in URL, fetch it from the registry
			digest, err := getArtifactDigest(ctx, url)
			if err != nil {
				return fmt.Errorf("failed to get digest for %s: %w", url, err)
			}
			rootCmd.Printf("  %s -> %s (from registry)\n", url, digest)
			digests = append(digests, digest)
		}
	}

	// Create a new artifacts attestation and sign the digests
	att := lkm.NewArtifactsAttestationForAudience(distroDefaultAudience)
	tokenString, err := att.Sign(pk, digests)
	if err != nil {
		return fmt.Errorf("failed to sign artifacts: %w", err)
	}

	// Write the signed JWT token to the output file
	err = os.WriteFile(distroSignArtifactsArgs.attestationPath, []byte(tokenString), 0644)
	if err != nil {
		return fmt.Errorf("failed to write attestation to file: %w", err)
	}

	rootCmd.Println(fmt.Sprintf("✔ attestation written to: %s", distroSignArtifactsArgs.attestationPath))

	return nil
}

// hasArtifactDigest checks if the given OCI URL has a valid artifact digest
// and returns the digest string if it exists, otherwise returns an error.
func hasArtifactDigest(ociURL string) (string, error) {
	ref, err := gcname.NewDigest(strings.TrimPrefix(ociURL, "oci://"), gcname.WeakValidation)
	if err != nil {
		return "", err
	}
	return ref.DigestStr(), nil
}

// getArtifactDigest looks up an artifact from an OCI repository and returns the digest of the artifact.
func getArtifactDigest(ctx context.Context, ociURL string) (string, error) {
	digest, err := crane.Digest(strings.TrimPrefix(ociURL, "oci://"), crane.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("fetching digest for artifact %s failed: %w", ociURL, err)
	}
	return digest, nil
}

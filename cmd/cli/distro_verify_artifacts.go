// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroVerifyArtifactsCmd = &cobra.Command{
	Use:   "artifacts",
	Short: "Verify the attestation of OCI artifacts",
	Example: `  # Verify artifacts attestation
  flux-operator distro verify artifacts \
  --key-set=/path/to/public.jwks \
  --attestation=/path/to/fluxcd-v2.6.4.jwt \
  --url=ghcr.io/fluxcd/source-controller:v1.6.2 \
  --url=ghcr.io/fluxcd/kustomize-controller:v1.6.1 \
  --url=ghcr.io/fluxcd/notification-controller:v1.6.0 \
  --url=ghcr.io/fluxcd/helm-controller:v1.3.0

  # Verify by reading the public key set from env
  export FLUX_DISTRO_SIG_PUBLIC_JWKS="$(cat /path/to/public.jwks)"
  flux-operator distro verify artifacts \
  -a /path/to/flux-operator-v0.28.0.jwt \
  -u ghcr.io/controlplaneio-fluxcd/flux-operator:v0.28.0 \
  -u ghcr.io/controlplaneio-fluxcd/flux-operator:v0.28.0-ubi \
  -u ghcr.io/controlplaneio-fluxcd/charts/flux-operator:v0.28.0
`,
	Args: cobra.NoArgs,
	RunE: distroVerifyArtifactsCmdRun,
}

type distroVerifyArtifactsFlags struct {
	publicKeySetPath string
	attestationPath  string
	urls             []string
}

var distroVerifyArtifactsArgs distroVerifyArtifactsFlags

func init() {
	distroVerifyArtifactsCmd.Flags().StringVarP(&distroVerifyArtifactsArgs.publicKeySetPath, "key-set", "k", "",
		"path to the public key set file or /dev/stdin (required)")
	distroVerifyArtifactsCmd.Flags().StringVarP(&distroVerifyArtifactsArgs.attestationPath, "attestation", "a", "",
		"path to the attestation file (required)")
	distroVerifyArtifactsCmd.Flags().StringSliceVarP(&distroVerifyArtifactsArgs.urls, "url", "u", nil,
		"OCI artifact URLs to verify (required, can be specified multiple times)")
	distroVerifyCmd.AddCommand(distroVerifyArtifactsCmd)
}

func distroVerifyArtifactsCmdRun(cmd *cobra.Command, args []string) error {
	if distroVerifyArtifactsArgs.attestationPath == "" {
		return fmt.Errorf("--attestation flag is required")
	}
	if len(distroVerifyArtifactsArgs.urls) == 0 {
		return fmt.Errorf("--url flag is required, specify one or more OCI artifact URLs")
	}

	// Read the signed JWT from file
	jwtData, err := os.ReadFile(distroVerifyArtifactsArgs.attestationPath)
	if err != nil {
		return fmt.Errorf("failed to read the attestation file: %w", err)
	}

	// Load the JWKS from file or environment variable
	jwksData, err := loadKeySet(distroVerifyArtifactsArgs.publicKeySetPath, distroSigPublicKeySetEnvVar)
	if err != nil {
		return err
	}

	// Verify the JWT signature and extract the verified payload
	verifiedPayload, err := lkm.VerifySignedToken(jwtData, jwksData)
	if err != nil {
		return fmt.Errorf("failed to verify JWT signature: %w", err)
	}

	// Create an ArtifactsAttestation from the verified data
	att, err := lkm.NewArtifactsAttestation(verifiedPayload)
	if err != nil {
		return fmt.Errorf("failed to parse attestation: %w", err)
	}

	// Process URLs to collect artifact digests
	var digests []string
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	rootCmd.Println("processing artifacts:")
	for _, url := range distroVerifyArtifactsArgs.urls {
		// First check if URL already contains a digest
		if digest, err := hasArtifactDigest(url); err == nil {
			rootCmd.Println(fmt.Sprintf("  %s -> %s (from URL)\n", strings.Split(url, "@")[0], digest))
			digests = append(digests, digest)
		} else {
			// If no digest in URL, fetch it from the registry
			digest, err := getArtifactDigest(ctx, url)
			if err != nil {
				return fmt.Errorf("failed to get digest for %s: %w", url, err)
			}
			rootCmd.Println(fmt.Sprintf("  %s -> %s (from registry)", url, digest))
			digests = append(digests, digest)
		}
	}

	// Verify that all digests from URLs are present in the attestation
	rootCmd.Println(fmt.Sprintf("verifying %d digests against attestation:", len(digests)))

	notfound := 0
	for _, digest := range digests {
		if att.HasDigest(digest) {
			rootCmd.Printf("  ✔ %s\n", digest)
		} else {
			rootCmd.Printf("  ✗ %s\n", digest)
			notfound++
		}
	}

	if notfound > 0 {
		return fmt.Errorf("verification failed: %d/%d digest(s) not attested for", notfound, len(digests))
	}

	// Print success message
	rootCmd.Println(fmt.Sprintf("✔ attestation issued by %s at %s is valid", att.GetIssuer(), att.GetIssuedAt()))
	rootCmd.Println(fmt.Sprintf("✔ verified %d artifacts successfully\n", len(digests)))

	return nil
}

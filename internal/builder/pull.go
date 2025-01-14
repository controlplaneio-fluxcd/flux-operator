// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

// PullArtifact downloads an artifact from an OCI repository and extracts the content
// of the first tgz layer to the given destination directory.
// It returns the digest of the artifact.
func PullArtifact(ctx context.Context, ociURL, dstDir string, keyChain authn.Keychain) (string, error) {
	img, err := crane.Pull(strings.TrimPrefix(ociURL, "oci://"), crane.WithContext(ctx), crane.WithAuthFromKeychain(keyChain))
	if err != nil {
		return "", fmt.Errorf("pulling artifact %s failed: %w", ociURL, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing digest for artifact %s failed: %w", ociURL, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return "", fmt.Errorf("listing layers in artifact %s failed: %w", ociURL, err)
	}

	if len(layers) < 1 {
		return "", fmt.Errorf("no layers found in artifact %s", ociURL)
	}

	blob, err := layers[0].Compressed()
	if err != nil {
		return "", fmt.Errorf("extracting layer from artifact %s failed: %w", ociURL, err)
	}

	if err = tar.Untar(blob, dstDir, tar.WithMaxUntarSize(-1)); err != nil {
		return "", fmt.Errorf("extracting layer from artifact %s failed: %w", ociURL, err)
	}

	return digest.String(), nil
}

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
)

// GetArtifactDigest looks up an artifact from an OCI repository and returns the digest of the artifact.
func GetArtifactDigest(ctx context.Context, ociURL string) (string, error) {
	digest, err := crane.Digest(strings.TrimPrefix(ociURL, "oci://"), crane.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("fetching digest for artifact %s failed: %w", ociURL, err)
	}
	return digest, nil
}

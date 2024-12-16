// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
)

// HeadArtifact looks up an artifact from an OCI repository and returns the digest of the artifact.
func HeadArtifact(ctx context.Context, ociURL string) (string, error) {
	desc, err := crane.Head(strings.TrimPrefix(ociURL, "oci://"), crane.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("fetching descriptor for artifact %s failed: %w", ociURL, err)
	}
	return desc.Digest.String(), nil
}

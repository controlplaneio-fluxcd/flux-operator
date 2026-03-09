// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

const (
	// fluxContentMediaType is the media type for Flux OCI artifact content layers.
	fluxContentMediaType = "application/vnd.cncf.flux.content.v1.tar+gzip"

	// AnnotationCreated is the OCI annotation key for the image creation timestamp.
	AnnotationCreated = "org.opencontainers.image.created"

	// AnnotationSource is the OCI annotation key for the source repository URL.
	AnnotationSource = "org.opencontainers.image.source"

	// AnnotationRevision is the OCI annotation key for the source revision.
	AnnotationRevision = "org.opencontainers.image.revision"

	// AnnotationVersion is the OCI annotation key for the image version.
	AnnotationVersion = "org.opencontainers.image.version"
)

// NormalizeRepository strips the oci:// prefix and trailing slashes from a repository URL.
func NormalizeRepository(repo string) string {
	repo = strings.TrimPrefix(repo, "oci://")
	repo = strings.TrimRight(repo, "/")
	return repo
}

// IsGitHubContainerRegistry checks if the repository host is ghcr.io.
func IsGitHubContainerRegistry(repo string) bool {
	return strings.HasPrefix(repo, "ghcr.io/")
}

// DeriveGitHubOwner extracts the GitHub owner from a ghcr.io repository URL.
func DeriveGitHubOwner(repo string) string {
	// repo is like "ghcr.io/OWNER/..."
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// ResolveDigest resolves the digest of an OCI artifact without downloading it.
func ResolveDigest(ctx context.Context, ociURL string) (string, error) {
	ref := strings.TrimPrefix(ociURL, "oci://")

	digest, err := crane.Digest(ref, crane.WithContext(ctx), crane.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", fmt.Errorf("resolving digest for %s: %w", ociURL, err)
	}

	return digest, nil
}

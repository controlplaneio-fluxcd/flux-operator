// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"context"
	"fmt"
	"strings"

	untar "github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

const (
	// fluxContentMediaType is the media type for Flux OCI artifact content layers.
	fluxContentMediaType = "application/vnd.cncf.flux.content.v1.tar+gzip"
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

// ArtifactInfo holds metadata from a pulled OCI artifact.
type ArtifactInfo struct {
	// Digest is the artifact digest string (e.g. "sha256:...").
	Digest string

	// Annotations holds the manifest annotations.
	Annotations map[string]string
}

// PullArtifact pulls a Flux OCI artifact, finds the content layer by media type,
// and extracts it to dstDir. It returns the artifact metadata.
func PullArtifact(ctx context.Context, ociURL, dstDir string) (*ArtifactInfo, error) {
	ref := strings.TrimPrefix(ociURL, "oci://")

	img, err := crane.Pull(ref, crane.WithContext(ctx), crane.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("pulling artifact %s: %w", ociURL, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("getting digest for %s: %w", ociURL, err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("reading manifest for %s: %w", ociURL, err)
	}

	layer, err := findFluxContentLayer(img, ociURL)
	if err != nil {
		return nil, err
	}

	blob, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("extracting layer from %s: %w", ociURL, err)
	}

	if err := untar.Untar(blob, dstDir, untar.WithMaxUntarSize(-1), untar.WithSkipSymlinks()); err != nil {
		return nil, fmt.Errorf("extracting artifact %s: %w", ociURL, err)
	}

	return &ArtifactInfo{
		Digest:      digest.String(),
		Annotations: manifest.Annotations,
	}, nil
}

// findFluxContentLayer finds the Flux content layer in an OCI image.
// It returns an error if no layer matches the Flux content media type.
func findFluxContentLayer(img v1.Image, ociURL string) (v1.Layer, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("listing layers in %s: %w", ociURL, err)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("no layers found in %s", ociURL)
	}

	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			continue
		}
		if string(mt) == fluxContentMediaType {
			return l, nil
		}
	}

	return nil, fmt.Errorf("no layer with media type %s found in %s", fluxContentMediaType, ociURL)
}

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

	layer, err := findFluxContentLayer(img, manifest, ociURL)
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

// findFluxContentLayer finds the Flux content layer in an OCI image using the
// pre-parsed manifest to avoid re-parsing. It looks up the layer by digest
// after matching the media type from the manifest descriptors.
func findFluxContentLayer(img v1.Image, manifest *v1.Manifest, ociURL string) (v1.Layer, error) {
	if len(manifest.Layers) == 0 {
		return nil, fmt.Errorf("no layers found in %s", ociURL)
	}

	for _, desc := range manifest.Layers {
		if string(desc.MediaType) == fluxContentMediaType {
			layer, err := img.LayerByDigest(desc.Digest)
			if err != nil {
				return nil, fmt.Errorf("getting layer %s from %s: %w", desc.Digest, ociURL, err)
			}
			return layer, nil
		}
	}

	return nil, fmt.Errorf("no layer with media type %s found in %s", fluxContentMediaType, ociURL)
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ErrDiffIdentical is returned by DiffArtifact when the local and remote
// content layers are identical.
var ErrDiffIdentical = errors.New("artifact contents are identical")

// DiffArtifact compares the local artifact data against the content layer of
// the remote artifact at the given tag. It fetches only the remote manifest
// and compares the Flux content layer digest.
// It returns ErrDiffIdentical if the content is the same, nil if different.
// Failures are treated as "different" (fail open).
func DiffArtifact(ctx context.Context, repo string, tag string, localData []byte) error {
	ref := fmt.Sprintf("%s:%s", repo, tag)
	craneOpts := []crane.Option{crane.WithContext(ctx), crane.WithAuthFromKeychain(authn.DefaultKeychain)}

	rawManifest, err := crane.Manifest(ref, craneOpts...)
	if err != nil {
		// Tag doesn't exist or registry error — treat as different.
		return nil
	}

	var manifest v1.Manifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil
	}

	// Find the Flux content layer descriptor.
	var layerDigestHex string
	for _, desc := range manifest.Layers {
		if string(desc.MediaType) == fluxContentMediaType {
			layerDigestHex = desc.Digest.Hex
			break
		}
	}
	if layerDigestHex == "" {
		return nil
	}

	h := sha256.Sum256(localData)
	if hex.EncodeToString(h[:]) == layerDigestHex {
		return ErrDiffIdentical
	}

	return nil
}

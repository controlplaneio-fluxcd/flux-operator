// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package cosign

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// SignArtifact signs an OCI artifact using cosign keyless mode.
// The ociRef must be digest-pinned (e.g. repo@sha256:...).
// Stdout and stderr are connected to the terminal so that cosign
// can display the OIDC browser authentication flow.
func SignArtifact(ctx context.Context, ociRef string) error {
	cosignPath, err := exec.LookPath("cosign")
	if err != nil {
		return fmt.Errorf("cosign not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, cosignPath, "sign", "--yes", ociRef)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign sign failed: %w", err)
	}

	return nil
}

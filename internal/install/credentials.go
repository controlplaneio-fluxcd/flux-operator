// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/fluxcd/pkg/ssa"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// ApplyCredentials creates and applies the appropriate Kubernetes Secret
// for the provided address using the credentials in the format "username:token".
// It supports both Git (HTTP/S or SSH) and OCI registry sources.
// The secret is created in the installer's namespace with the given secretName.
func (in *Installer) ApplyCredentials(ctx context.Context, secretName, address string) (*ssa.ChangeSet, error) {
	if address == "" {
		return nil, fmt.Errorf("address is required to create credentials secret")
	}

	// Parse credentials
	parts := strings.SplitN(in.options.credentials, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid credentials format, expected username:token")
	}
	username := parts[0]
	password := parts[1]

	var secret *corev1.Secret
	var err error

	// Determine source type and create appropriate secret
	if strings.HasPrefix(address, "oci://") {
		// Extract server from OCI URL (strip oci:// prefix and take host part)
		server := strings.TrimPrefix(address, "oci://")
		if idx := strings.Index(server, "/"); idx > 0 {
			server = server[:idx]
		}
		secret, err = secrets.MakeRegistrySecret(
			secretName,
			in.options.namespace,
			server,
			username,
			password,
		)
	} else {
		// Git source (HTTP/S or SSH)
		secret, err = secrets.MakeBasicAuthSecret(
			secretName,
			in.options.namespace,
			username,
			password,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to convert secret to unstructured: %w", err)
	}

	return in.kubeClient.Manager.ApplyAllStaged(ctx, []*unstructured.Unstructured{{Object: rawMap}}, ssa.DefaultApplyOptions())
}

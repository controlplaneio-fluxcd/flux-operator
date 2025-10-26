// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

// Installer handles the installation of Flux Operator and Flux instance.
type Installer struct {
	kubeClient *KubeClient
	options    *Options
}

// NewInstaller creates a new Installer with the provided Kubernetes config and options.
// If no ArtifactURL, Owner, or Namespace is provided in the options, it uses the defaults.
func NewInstaller(ctx context.Context, cfg *rest.Config, opts ...Option) (*Installer, error) {
	// Apply default options
	defaultOpts := []Option{
		WithArtifactURL(DefaultArtifactURL),
		WithOwner(DefaultOwner),
		WithNamespace(DefaultNamespace),
		WithTerminationTimeout(DefaultTerminationTimeout),
	}

	// User options override defaults
	allOpts := append(defaultOpts, opts...)

	// Create and validate the final options
	options, err := MakeOptions(allOpts...)
	if err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Create the Kubernetes client
	kubeClient, err := NewKubeClient(ctx, cfg, options.Owner())
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Installer{
		kubeClient: kubeClient,
		options:    options,
	}, nil
}

// IsInstalled checks if the Flux Operator installed in the cluster.
// If the installation is managed by Helm, it returns an error.
func (in *Installer) IsInstalled(ctx context.Context) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := in.kubeClient.Get(ctx, types.NamespacedName{
		Name: "fluxinstances.fluxcd.controlplane.io",
	}, crd)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("unable to check if Flux Operator is installed: %w", err)
	}

	// Check if the CRD is managed by Helm
	if managedBy, exists := crd.Labels["app.kubernetes.io/managed-by"]; exists && managedBy == "Helm" {
		return true, fmt.Errorf("the Flux Operator installation is managed by Helm, cannot proceed with installation")
	}

	return true, nil
}

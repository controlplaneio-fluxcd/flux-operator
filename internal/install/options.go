// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultArtifactURL is the default OCI artifact URL for Flux Operator manifests.
	DefaultArtifactURL = "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest"

	// DefaultOwner is the default field manager name for server-side apply operations.
	DefaultOwner = "kubectl-flux-operator"

	// DefaultNamespace is the default namespace for Flux Operator and Flux instance.
	DefaultNamespace = "flux-system"

	// DefaultTerminationTimeout is the default timeout for waiting for resource termination.
	DefaultTerminationTimeout = 30 * time.Second
)

// Options holds the configuration for installing the Flux Operator and FluxInstance.
// This is shared between the CLI and MCP server.
type Options struct {
	// artifactURL is the OCI artifact URL containing the Flux Operator manifests.
	// Example: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest"
	artifactURL string

	// credentials are the credentials for the sync source in the format "username:token".
	// These are used to create a Kubernetes secret for authenticating to Git or OCI repositories.
	credentials string

	// owner is the field manager name used for server-side apply operations.
	owner string

	// namespace is the namespace where Flux Operator and Flux instance will be installed.
	namespace string

	// terminationTimeout is the timeout for waiting for resource termination during uninstall.
	terminationTimeout time.Duration
}

// Option is a functional option for configuring the installer.
type Option func(*Options)

// WithArtifactURL sets the OCI artifact URL containing the Flux Operator manifests.
func WithArtifactURL(url string) Option {
	return func(o *Options) {
		o.artifactURL = url
	}
}

// WithCredentials sets the credentials for the sync source in the format "username:token".
func WithCredentials(credentials string) Option {
	return func(o *Options) {
		o.credentials = credentials
	}
}

// WithOwner sets the field manager name for server-side apply operations.
func WithOwner(owner string) Option {
	return func(o *Options) {
		o.owner = owner
	}
}

// WithNamespace sets the namespace where Flux Operator and Flux instance will be installed.
func WithNamespace(namespace string) Option {
	return func(o *Options) {
		o.namespace = namespace
	}
}

// WithTerminationTimeout sets the timeout for waiting for resource termination during uninstall.
func WithTerminationTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.terminationTimeout = timeout
	}
}

// MakeOptions creates a new Options instance with the provided functional options.
func MakeOptions(opts ...Option) (*Options, error) {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	if err := o.validate(); err != nil {
		return nil, err
	}

	return o, nil
}

// validate checks if the options are valid and returns an error if not.
func (o *Options) validate() error {
	if !strings.HasPrefix(o.artifactURL, "oci://") {
		return fmt.Errorf("artifact URL must start with 'oci://', got: %s", o.artifactURL)
	}

	if o.credentials != "" {
		parts := strings.SplitN(o.credentials, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("credentials must be in the format 'username:token'")
		}
	}

	return nil
}

// ArtifactURL returns the OCI artifact URL.
func (o *Options) ArtifactURL() string {
	return o.artifactURL
}

// Credentials returns the sync credentials.
func (o *Options) Credentials() string {
	return o.credentials
}

// Owner returns the field manager name.
func (o *Options) Owner() string {
	return o.owner
}

// Namespace returns the installation namespace.
func (o *Options) Namespace() string {
	return o.namespace
}

// TerminationTimeout returns the timeout for waiting for resource termination during uninstall.
func (o *Options) TerminationTimeout() time.Duration {
	return o.terminationTimeout
}

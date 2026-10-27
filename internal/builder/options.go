// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
)

// Options defines the builder configuration.
type Options struct {
	Version                                         string
	VersionInfo                                     *VersionInfo
	Namespace                                       string
	Components                                      []string
	ComponentImages                                 []ComponentImage
	EventsAddr                                      string
	Registry                                        string
	Variant                                         string
	ImagePullSecret                                 string
	WatchAllNamespaces                              bool
	NetworkPolicy                                   bool
	LogLevel                                        string
	ClusterDomain                                   string
	TolerationKeys                                  []string
	Patches                                         string
	ArtifactStorage                                 *ArtifactStorage
	Sync                                            *Sync
	ShardingKey                                     string
	ShardingStorage                                 bool
	Shards                                          []string
	ShardName                                       string
	SourceAPIVersion                                string
	RemovePermissionForCreatingServiceAccountTokens bool
	AddPermissionForCreatingServiceAccountTokens    bool
	// OperatorMTLSEnabled indicates that the operator has a SPIFFE Workload
	// API source and participates in mTLS. When true, the flux-operator SPIFFE
	// ID derived from ComponentIDTemplate with 'flux-operator' as the
	// component name is authorized as a notifier of notification-controller,
	// and as an artifact client of source-controller and source-watcher only
	// when WebUIEnabled is true.
	OperatorMTLSEnabled bool
	// WebUIEnabled indicates that the operator's embedded Web UI is enabled.
	// When true, the operator SPIFFE ID is authorized as an artifact client
	// of source-controller and source-watcher for Web UI artifact downloads.
	// The operator ID is authorized as a notifier of notification-controller
	// when OperatorMTLSEnabled is true, regardless of this setting.
	// Enabling or disabling the embedded Web UI changes these controller
	// arguments and rolls out the affected controllers.
	WebUIEnabled bool
	Logger       logr.Logger
}

// MakeDefaultOptions returns the default builder configuration.
func MakeDefaultOptions() Options {
	return Options{
		Version:   "*",
		Namespace: "flux-system",
		Components: []string{
			"source-controller",
			"kustomize-controller",
			"helm-controller",
			"notification-controller",
			"image-reflector-controller",
			"image-automation-controller",
		},
		EventsAddr:         "",
		Registry:           "ghcr.io/fluxcd",
		Variant:            "",
		ImagePullSecret:    "",
		WatchAllNamespaces: true,
		NetworkPolicy:      true,
		LogLevel:           "info",
		ClusterDomain:      "cluster.local",
		ShardingKey:        "sharding.fluxcd.io/key",
		// Assume the operator has a SPIFFE Workload API source when SPIFFE
		// mTLS is configured. The embedded Web UI setting must be provided
		// explicitly. The controller overrides both values with its runtime
		// configuration.
		OperatorMTLSEnabled: true,
		WebUIEnabled:        false,
		Logger:              logr.Discard(),
	}
}

// ComponentImage represents a container image used by a component.
type ComponentImage struct {
	Name       string
	Repository string
	Tag        string
	Digest     string
}

// ArtifactStorage represents the source-controller PVC.
type ArtifactStorage struct {
	Class string
	Size  string
}

type Sync struct {
	Name       string
	Kind       string
	URL        string
	Ref        string
	Path       string
	Interval   string
	PullSecret string
	Provider   string
}

type VersionInfo struct {
	Major int
	Minor int
	Patch int
}

func (o *Options) buildVersionInfo() error {
	ver, err := semver.NewVersion(o.Version)
	if err != nil {
		return fmt.Errorf("failed to parse Flux version '%s': %w", o.Version, err)
	}
	o.VersionInfo = &VersionInfo{
		Major: int(ver.Major()),
		Minor: int(ver.Minor()),
		Patch: int(ver.Patch()),
	}
	return nil
}

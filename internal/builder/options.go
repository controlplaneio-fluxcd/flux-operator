// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
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

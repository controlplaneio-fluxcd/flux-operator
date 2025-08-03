// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"slices"

	notification "github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
)

// Options defines the builder configuration.
type Options struct {
	Version                                string
	Namespace                              string
	Components                             []string
	ComponentImages                        []ComponentImage
	EventsAddr                             string
	Registry                               string
	ImagePullSecret                        string
	WatchAllNamespaces                     bool
	NetworkPolicy                          bool
	LogLevel                               string
	ClusterDomain                          string
	TolerationKeys                         []string
	Patches                                string
	ArtifactStorage                        *ArtifactStorage
	Sync                                   *Sync
	ShardingKey                            string
	ShardingStorage                        bool
	Shards                                 []string
	ShardName                              string
	SourceAPIVersion                       string
	EnableObjectLevelWorkloadIdentity      bool
	SupportsObjectLevelWorkloadIdentity    bool
	ObjectLevelWorkloadIdentityControllers string
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
			notification.Controller,
			"image-reflector-controller",
			"image-automation-controller",
		},
		EventsAddr:         "",
		Registry:           "ghcr.io/fluxcd",
		ImagePullSecret:    "",
		WatchAllNamespaces: true,
		NetworkPolicy:      true,
		LogLevel:           "info",
		ClusterDomain:      "cluster.local",
		ShardingKey:        "sharding.fluxcd.io/key",
	}
}

func (o *Options) HasNotificationController() bool {
	return slices.Contains(o.Components, notification.Controller)
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

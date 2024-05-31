// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

// Options defines the builder configuration.
type Options struct {
	Version                string
	Namespace              string
	Components             []string
	ComponentImages        []ComponentImage
	EventsAddr             string
	Registry               string
	ImagePullSecret        string
	WatchAllNamespaces     bool
	NetworkPolicy          bool
	LogLevel               string
	NotificationController string
	ClusterDomain          string
	TolerationKeys         []string
	Patches                string
	ArtifactStorage        *ArtifactStorage
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
		EventsAddr:             "",
		Registry:               "ghcr.io/fluxcd",
		ImagePullSecret:        "",
		WatchAllNamespaces:     true,
		NetworkPolicy:          true,
		LogLevel:               "info",
		NotificationController: "notification-controller",
		ClusterDomain:          "cluster.local",
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

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"time"
)

type Options struct {
	Version                string
	Namespace              string
	Components             []string
	EventsAddr             string
	Registry               string
	ImagePullSecret        string
	WatchAllNamespaces     bool
	NetworkPolicy          bool
	LogLevel               string
	NotificationController string
	Timeout                time.Duration
	ClusterDomain          string
	TolerationKeys         []string
	Patches                string
}

func MakeDefaultOptions() Options {
	return Options{
		Version:   "latest",
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
		Timeout:                time.Minute,
		ClusterDomain:          "cluster.local",
	}
}

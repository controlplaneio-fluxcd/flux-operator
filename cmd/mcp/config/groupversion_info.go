// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group and version of the Flux MCP Config API.
	GroupVersion = schema.GroupVersion{Group: "mcp.fluxcd.controlplane.io", Version: "v1"}
)

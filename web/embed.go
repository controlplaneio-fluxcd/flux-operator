// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import "embed"

//go:embed dist/*
var web embed.FS

// GetFS returns the embedded SPA filesystem.
func GetFS() embed.FS {
	return web
}

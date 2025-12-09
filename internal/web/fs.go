// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// FileSystem wraps an embedded filesystem to handle SPA routing.
// It serves index.html for all non-existent file requests (404s).
type FileSystem struct {
	fs fs.FS
}

// NewFileSystem creates a new filesystem handler for SPA routing.
func NewFileSystem(efs fs.FS) *FileSystem {
	return &FileSystem{fs: efs}
}

// Open implements fs.FS interface for SPA routing.
func (w *FileSystem) Open(name string) (fs.File, error) {
	// Normalize path
	name = strings.TrimPrefix(name, "/")

	// Try to open the requested file
	f, err := w.fs.Open(filepath.Join("dist", name))
	if err == nil {
		// File exists, return it
		return f, nil
	}

	// If it's not a directory request and the file doesn't exist,
	// serve index.html for SPA routing
	if !strings.HasSuffix(name, "/") {
		indexPath := filepath.Join("dist", "index.html")
		f, err := w.fs.Open(indexPath)
		if err == nil {
			return f, nil
		}
	}

	// Return original error if we can't fall back to index.html
	return nil, err
}

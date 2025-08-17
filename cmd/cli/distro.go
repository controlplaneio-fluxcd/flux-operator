// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"
	"golang.org/x/mod/sumdb/dirhash"
)

var distroCmd = &cobra.Command{
	Use:   "distro",
	Short: "Provides utilities for managing the Flux distribution",
}

func init() {
	rootCmd.AddCommand(distroCmd)
}

const (
	distroPrivateKeySetEnvVar = "FLUX_DISTRO_PRIVATE_KEY_SET"
	distroPublicKeySetEnvVar  = "FLUX_DISTRO_PUBLIC_KEY_SET"
)

// PrivateKeySet represents a JWK Set object for holding private keys.
type PrivateKeySet struct {
	// Issuer is the identifier of the entity that issued the keys.
	Issuer string `json:"issuer"`
	// Keys is a list of JSON Web Keys (JWKs) that make up the set.
	Keys []jose.JSONWebKey `json:"keys"`
}

// isDir validates that the given path exists and is a directory
func isDir(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", path)
	}
	if err != nil {
		return fmt.Errorf("failed to check path %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", path)
	}
	return nil
}

// hashDir returns the hash of the local file system directory dir,
// replacing the directory name itself with prefix in the file names
// used in the hash function.
func hashDir(dir, prefix, exclude string, hash dirhash.Hash) (string, error) {
	files, err := dirFiles(dir, prefix, exclude)
	if err != nil {
		return "", err
	}
	osOpen := func(name string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(dir, strings.TrimPrefix(name, prefix)))
	}
	return hash(files, osOpen)
}

// dirFiles returns the list of files in the tree rooted at dir,
// replacing the directory name dir with prefix in each name.
// The resulting names always use forward slashes.
func dirFiles(dir, prefix, exclude string) ([]string, error) {
	var files []string
	dir = filepath.Clean(dir)
	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		} else if file == dir {
			return fmt.Errorf("%s is not a directory", dir)
		}

		rel := file
		if dir != "." {
			rel = file[len(dir)+1:]
		}
		f := filepath.Join(prefix, rel)

		if exclude != "" && strings.HasSuffix(f, exclude) {
			// Skip files that match the exclude pattern
			return nil
		}

		files = append(files, filepath.ToSlash(f))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

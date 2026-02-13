// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ssadiff "github.com/fluxcd/pkg/ssa/jsondiff"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/spf13/cobra"
	"github.com/wI2L/jsondiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/install"
)

var diffYAMLCmd = &cobra.Command{
	Use:   "yaml <source> <target>",
	Short: "Diff two YAML files and generate a JSON patch",
	Long: `The diff yaml command compares two YAML files and produces a JSON patch (RFC 6902)
that can be applied to the source file to obtain the target file.

The source and target can be local file paths or remote URLs (including GitHub, GitLab,
GitHub Gist and OCI URLs).
The comparison ignores metadata and status fields, focusing on the semantic content.`,
	Example: `  # Diff two remote GitHub files (default YAML output)
  flux-operator diff yaml \
    https://github.com/fluxcd/source-controller/blob/main/config/crd/bases/source.toolkit.fluxcd.io_buckets.yaml \
    https://github.com/fluxcd/source-controller/blob/feat/config/crd/bases/source.toolkit.fluxcd.io_buckets.yaml

  # Diff with JSON patch output
  flux-operator diff yaml \
    https://github.com/fluxcd/source-controller/blob/main/config/crd/bases/source.toolkit.fluxcd.io_buckets.yaml \
    https://github.com/fluxcd/source-controller/blob/feat/config/crd/bases/source.toolkit.fluxcd.io_buckets.yaml \
    --output=json-patch

  # Diff a local file against a remote file
  flux-operator diff yaml local.yaml https://example.com/remote.yaml`,
	Args: cobra.ExactArgs(2),
	RunE: diffYAMLCmdRun,
}

type diffYAMLFlags struct {
	output string
}

var diffYAMLArgs diffYAMLFlags

func init() {
	diffYAMLCmd.Flags().StringVarP(&diffYAMLArgs.output, "output", "o", "json-patch-yaml",
		"Output format for the diff result. Supported formats: json-patch-yaml, json-patch.")

	diffCmd.AddCommand(diffYAMLCmd)
}

func diffYAMLCmdRun(cmd *cobra.Command, args []string) error {
	if diffYAMLArgs.output != "json-patch-yaml" && diffYAMLArgs.output != "json-patch" {
		return fmt.Errorf("unsupported output format %q, supported formats: json-patch-yaml, json-patch", diffYAMLArgs.output)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), rootArgs.timeout)
	defer cancel()

	sourceData, err := fetchYAML(ctx, args[0])
	if err != nil {
		return fmt.Errorf("fetching source: %w", err)
	}

	targetData, err := fetchYAML(ctx, args[1])
	if err != nil {
		return fmt.Errorf("fetching target: %w", err)
	}

	source, err := parseUnstructured(sourceData)
	if err != nil {
		return fmt.Errorf("parsing source YAML: %w", err)
	}

	target, err := parseUnstructured(targetData)
	if err != nil {
		return fmt.Errorf("parsing target YAML: %w", err)
	}

	patch, err := ssadiff.DiffUnstructured(source, target, jsondiff.Rationalize())
	if err != nil {
		return fmt.Errorf("computing diff: %w", err)
	}

	switch diffYAMLArgs.output {
	case "json-patch":
		patchJSON, err := json.MarshalIndent(patch, "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling patch: %w", err)
		}
		rootCmd.Println(string(patchJSON))
	case "json-patch-yaml":
		patchYAML, err := yaml.Marshal(patch)
		if err != nil {
			return fmt.Errorf("marshalling patch: %w", err)
		}
		rootCmd.Print(string(patchYAML))
	}

	return nil
}

// parseUnstructured parses YAML data into an unstructured Kubernetes object.
func parseUnstructured(data []byte) (*unstructured.Unstructured, error) {
	var obj map[string]any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

// fetchYAML retrieves YAML content from a local file path or remote URL.
// Remote URLs are handled by install.DownloadManifestFromURL which supports
// GitHub, GitLab, GitHub Gist and OCI URLs.
func fetchYAML(ctx context.Context, path string) ([]byte, error) {
	if strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "oci://") {
		return install.DownloadManifestFromURL(ctx, path, authn.DefaultKeychain)
	}
	return os.ReadFile(path)
}

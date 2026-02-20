// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// TestPatchInstanceCmd patches a FluxInstance with all 7 controllers
// from Flux v2.7 to v2.8, fetching real CRD data from GitHub.
func TestPatchInstanceCmd(t *testing.T) {
	g := NewWithT(t)

	inputData, err := os.ReadFile("testdata/patch_instance/input.yaml")
	g.Expect(err).ToNot(HaveOccurred())

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "instance.yaml")
	g.Expect(os.WriteFile(tmpFile, inputData, 0644)).To(Succeed())

	output, err := executeCommand([]string{
		"patch", "instance",
		"-f", tmpFile,
		"-v", "8",
	})
	g.Expect(err).ToNot(HaveOccurred())

	goldenOutput, err := os.ReadFile("testdata/patch_instance/golden-v2.8-output.txt")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(Equal(fmt.Sprintf(string(goldenOutput), tmpFile)))

	result, err := os.ReadFile(tmpFile)
	g.Expect(err).ToNot(HaveOccurred())

	golden, err := os.ReadFile("testdata/patch_instance/golden-v2.8.yaml")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(result)).To(Equal(string(golden)))
}

func TestPatchInstanceCmd_Mock(t *testing.T) {
	g := NewWithT(t)

	sourceData, err := os.ReadFile("testdata/patch_instance/crd-source.yaml")
	g.Expect(err).ToNot(HaveOccurred())
	targetData, err := os.ReadFile("testdata/patch_instance/crd-target.yaml")
	g.Expect(err).ToNot(HaveOccurred())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/v1.7.0/"):
			_, _ = w.Write(sourceData)
		case strings.Contains(r.URL.Path, "/v1.8.0/"):
			_, _ = w.Write(targetData)
		case strings.Contains(r.URL.Path, "/main/"):
			_, _ = w.Write(targetData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	tests := []struct {
		name         string
		input        string
		goldenOutput string
		goldenYAML   string
		extraArgs    []string
		stdin        bool
	}{
		{
			name:         "no kustomize section",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output.txt",
			goldenYAML:   "golden-no-kustomize.yaml",
		},
		{
			name:         "existing user patches",
			input:        "input-with-existing-patches.yaml",
			goldenOutput: "golden-mock-output.txt",
			goldenYAML:   "golden-with-existing-patches.yaml",
		},
		{
			name:         "empty patches array",
			input:        "input-with-empty-patches.yaml",
			goldenOutput: "golden-mock-output.txt",
			goldenYAML:   "golden-with-empty-patches.yaml",
		},
		{
			name:         "deep indent patches",
			input:        "input-with-deep-indent-patches.yaml",
			goldenOutput: "golden-mock-output.txt",
			goldenYAML:   "golden-with-deep-indent-patches.yaml",
		},
		{
			name:         "stale generated patches replaced",
			input:        "input-with-stale-patches.yaml",
			goldenOutput: "golden-mock-output-with-removal.txt",
			goldenYAML:   "golden-with-existing-patches.yaml",
		},
		{
			name:         "idempotent re-run",
			input:        "golden-with-existing-patches.yaml",
			goldenOutput: "golden-mock-output-with-removal.txt",
			goldenYAML:   "golden-with-existing-patches.yaml",
		},
		{
			name:         "registry override",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output.txt",
			goldenYAML:   "golden-registry-override.yaml",
			extraArgs:    []string{"-r", "registry.example.com/fluxcd"},
		},
		{
			name:         "components override",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output-components.txt",
			goldenYAML:   "golden-components-override.yaml",
			extraArgs:    []string{"-c", "notification-controller"},
		},
		{
			name:         "version main",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output-main.txt",
			goldenYAML:   "golden-main.yaml",
			extraArgs:    []string{"-v", "main"},
		},
		{
			name:         "stdin input",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output-stdin.txt",
			stdin:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			inputData, err := os.ReadFile(filepath.Join("testdata/patch_instance", tt.input))
			g.Expect(err).ToNot(HaveOccurred())

			fluxControllerBaseURL = ts.URL

			if tt.stdin {
				// Replace os.Stdin with a file containing the input data.
				stdinFile, err := os.CreateTemp(t.TempDir(), "stdin-*.yaml")
				g.Expect(err).ToNot(HaveOccurred())
				_, err = stdinFile.Write(inputData)
				g.Expect(err).ToNot(HaveOccurred())
				_, err = stdinFile.Seek(0, 0)
				g.Expect(err).ToNot(HaveOccurred())

				origStdin := os.Stdin
				os.Stdin = stdinFile
				defer func() { os.Stdin = origStdin }()

				args := append([]string{"patch", "instance", "-f", "-", "-v", "8"}, tt.extraArgs...)
				output, err := executeCommand(args)
				g.Expect(err).ToNot(HaveOccurred())

				goldenOutput, err := os.ReadFile(filepath.Join("testdata/patch_instance", tt.goldenOutput))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(output).To(Equal(string(goldenOutput)))
				return
			}

			tmpFile := filepath.Join(t.TempDir(), "instance.yaml")
			g.Expect(os.WriteFile(tmpFile, inputData, 0644)).To(Succeed())

			args := append([]string{"patch", "instance", "-f", tmpFile, "-v", "8"}, tt.extraArgs...)

			output, err := executeCommand(args)
			g.Expect(err).ToNot(HaveOccurred())

			goldenOutput, err := os.ReadFile(filepath.Join("testdata/patch_instance", tt.goldenOutput))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(Equal(fmt.Sprintf(string(goldenOutput), tmpFile)))

			result, err := os.ReadFile(tmpFile)
			g.Expect(err).ToNot(HaveOccurred())
			golden, err := os.ReadFile(filepath.Join("testdata/patch_instance", tt.goldenYAML))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(result)).To(Equal(string(golden)))
		})
	}
}

func TestPatchInstanceCmd_Errors(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError string
	}{
		{
			name:        "missing filename",
			args:        []string{"patch", "instance"},
			expectError: "--filename is required",
		},
		{
			name:        "nonexistent file",
			args:        []string{"patch", "instance", "-f", "nonexistent.yaml"},
			expectError: "invalid filename",
		},
		{
			name:        "invalid version flag",
			args:        []string{"patch", "instance", "-f", "testdata/patch_instance/input.yaml", "-v", "invalid"},
			expectError: "resolving target version",
		},
		{
			name:        "downgrade rejected",
			args:        []string{"patch", "instance", "-f", "testdata/patch_instance/input.yaml", "-v", "6"},
			expectError: "target minor version 6 must be greater than current minor version 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			_, err := executeCommand(tt.args)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
		})
	}
}

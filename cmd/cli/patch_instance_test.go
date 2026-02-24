// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// requireEqualStrings compares two strings and fails the test with a
// line-by-line diff showing exactly where they diverge. This avoids
// the truncation that gomega's Equal matcher applies to long strings.
func requireEqualStrings(t *testing.T, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	var b strings.Builder
	maxLines := len(gotLines)
	if len(wantLines) > maxLines {
		maxLines = len(wantLines)
	}
	for i := range maxLines {
		g, w := "", ""
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			fmt.Fprintf(&b, "line %d:\n  got:  %q\n  want: %q\n", i+1, g, w)
		}
	}
	if len(gotLines) != len(wantLines) {
		fmt.Fprintf(&b, "line count: got %d, want %d\n", len(gotLines), len(wantLines))
	}
	t.Fatalf("string mismatch:\n%s", b.String())
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
			name:         "scoped cleanup preserves other components",
			input:        "input-with-mixed-patches.yaml",
			goldenOutput: "golden-mock-output-mixed.txt",
			goldenYAML:   "golden-mixed-patches.yaml",
			extraArgs:    []string{"-c", "kustomize-controller"},
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
			resolveMainBranchSHA = func(_ context.Context, _ string) (string, error) {
				return "abc1234567890def", nil
			}
			validatePatchedInstance = func(_ string) error { return nil }

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
				requireEqualStrings(t, output, string(goldenOutput))
				return
			}

			tmpFile := filepath.Join(t.TempDir(), "instance.yaml")
			g.Expect(os.WriteFile(tmpFile, inputData, 0644)).To(Succeed())

			args := append([]string{"patch", "instance", "-f", tmpFile, "-v", "8"}, tt.extraArgs...)

			output, err := executeCommand(args)
			g.Expect(err).ToNot(HaveOccurred())

			goldenOutput, err := os.ReadFile(filepath.Join("testdata/patch_instance", tt.goldenOutput))
			g.Expect(err).ToNot(HaveOccurred())
			requireEqualStrings(t, output, fmt.Sprintf(string(goldenOutput), tmpFile))

			result, err := os.ReadFile(tmpFile)
			g.Expect(err).ToNot(HaveOccurred())
			golden, err := os.ReadFile(filepath.Join("testdata/patch_instance", tt.goldenYAML))
			g.Expect(err).ToNot(HaveOccurred())
			requireEqualStrings(t, string(result), string(golden))
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

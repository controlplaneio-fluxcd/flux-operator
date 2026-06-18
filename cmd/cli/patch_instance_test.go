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

	"github.com/fluxcd/pkg/apis/kustomize"
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
		case strings.Contains(r.URL.Path, "/release/v2.8.x/"):
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
			name:         "remove scoped to one component preserves others",
			input:        "input-with-mixed-patches.yaml",
			goldenOutput: "golden-mock-output-rm.txt",
			goldenYAML:   "golden-rm-mixed.yaml",
			extraArgs:    []string{"-c", "kustomize-controller", "--rm"},
		},
		{
			name:         "version main",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output-main.txt",
			goldenYAML:   "golden-main.yaml",
			extraArgs:    []string{"-v", "main"},
		},
		{
			name:         "version branch",
			input:        "input-no-kustomize.yaml",
			goldenOutput: "golden-mock-output-branch.txt",
			goldenYAML:   "golden-main.yaml",
			extraArgs:    []string{"-v", "release/v2.8.x"},
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
			resolveBranchSHA = func(_ context.Context, _, _ string) (string, error) {
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

func TestIsGeneratedPatchForComponents(t *testing.T) {
	const (
		imageOp = "- op: replace\n  path: /spec/template/spec/containers/0/image\n  value: ghcr.io/fluxcd/kustomize-controller:v1.8.0\n"
		argsOp  = "- op: add\n  path: /spec/template/spec/containers/0/args/-\n  value: --log-level=debug\n"
		crdOp   = "- op: remove\n  path: /spec/versions/1\n"
		crdName = "kustomizations.kustomize.toolkit.fluxcd.io"
	)
	components := []string{"kustomize-controller"}

	tests := []struct {
		name  string
		patch kustomize.Patch
		want  bool
	}{
		{
			name:  "generated image patch",
			patch: kustomize.Patch{Patch: imageOp, Target: &kustomize.Selector{Kind: "Deployment", Name: "kustomize-controller"}},
			want:  true,
		},
		{
			name:  "user args patch on same target is preserved",
			patch: kustomize.Patch{Patch: argsOp, Target: &kustomize.Selector{Kind: "Deployment", Name: "kustomize-controller"}},
			want:  false,
		},
		{
			name:  "image patch for non-selected component",
			patch: kustomize.Patch{Patch: imageOp, Target: &kustomize.Selector{Kind: "Deployment", Name: "source-controller"}},
			want:  false,
		},
		{
			name:  "image patch with extra namespace selector is user-authored",
			patch: kustomize.Patch{Patch: imageOp, Target: &kustomize.Selector{Kind: "Deployment", Name: "kustomize-controller", Namespace: "flux-system"}},
			want:  false,
		},
		{
			name:  "image patch with extra label selector is user-authored",
			patch: kustomize.Patch{Patch: imageOp, Target: &kustomize.Selector{Kind: "Deployment", Name: "kustomize-controller", LabelSelector: "app=foo"}},
			want:  false,
		},
		{
			name:  "image patch bundled with another op is user-authored",
			patch: kustomize.Patch{Patch: imageOp + argsOp, Target: &kustomize.Selector{Kind: "Deployment", Name: "kustomize-controller"}},
			want:  false,
		},
		{
			name:  "generated CRD patch",
			patch: kustomize.Patch{Patch: crdOp, Target: &kustomize.Selector{Kind: "CustomResourceDefinition", Name: crdName}},
			want:  true,
		},
		{
			name:  "user strategic-merge CRD patch is preserved",
			patch: kustomize.Patch{Patch: "spec:\n  conversion:\n    strategy: None\n", Target: &kustomize.Selector{Kind: "CustomResourceDefinition", Name: crdName}},
			want:  false,
		},
		{
			name:  "CRD patch for CRD not owned by selected component",
			patch: kustomize.Patch{Patch: crdOp, Target: &kustomize.Selector{Kind: "CustomResourceDefinition", Name: "alerts.notification.toolkit.fluxcd.io"}},
			want:  false,
		},
		{
			name:  "unrelated kind",
			patch: kustomize.Patch{Patch: imageOp, Target: &kustomize.Selector{Kind: "ConfigMap", Name: "kustomize-controller"}},
			want:  false,
		},
		{
			name:  "nil target",
			patch: kustomize.Patch{Patch: imageOp},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isGeneratedPatchForComponents(tt.patch, components)).To(Equal(tt.want))
		})
	}
}

func TestPatchInstanceCmd_Remove(t *testing.T) {
	const header = `apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.0"
    registry: "ghcr.io/fluxcd"
  components:
    - kustomize-controller
  kustomize:
    patches:
`
	userArgsPatch := `    - patch: |
        - op: add
          path: /spec/template/spec/containers/0/args/-
          value: --log-level=debug
      target:
        kind: Deployment
        name: kustomize-controller
`
	generatedImagePatch := `    - patch: |
        - op: replace
          path: /spec/template/spec/containers/0/image
          value: ghcr.io/fluxcd/kustomize-controller:v1.8.0
      target:
        kind: Deployment
        name: kustomize-controller
`
	generatedCRDPatch := `    - patch: |
        - op: remove
          path: /spec/versions/1
      target:
        kind: CustomResourceDefinition
        name: kustomizations.kustomize.toolkit.fluxcd.io
`
	userMergeCRDPatch := `    - patch: |
        spec:
          conversion:
            strategy: None
      target:
        kind: CustomResourceDefinition
        name: kustomizations.kustomize.toolkit.fluxcd.io
`
	noKustomize := `apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.0"
    registry: "ghcr.io/fluxcd"
  components:
    - kustomize-controller
`

	tests := []struct {
		name       string
		input      string
		want       string // expected file content; empty means unchanged
		wantOutput string
	}{
		{
			name:       "removes generated patches but keeps user patch on same target",
			input:      header + userArgsPatch + generatedImagePatch + generatedCRDPatch,
			want:       header + userArgsPatch,
			wantOutput: "Removed 2 patches",
		},
		{
			name:       "keeps user strategic-merge CRD patch, removes generated image",
			input:      header + userMergeCRDPatch + generatedImagePatch,
			want:       header + userMergeCRDPatch,
			wantOutput: "Removed 1 patches",
		},
		{
			name:       "no generated patches found leaves file unchanged",
			input:      header + userArgsPatch,
			wantOutput: "No generated patches found",
		},
		{
			name:       "no kustomize section",
			input:      noKustomize,
			wantOutput: "No patches to remove",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			validatePatchedInstance = func(_ string) error { return nil }

			tmpFile := filepath.Join(t.TempDir(), "instance.yaml")
			g.Expect(os.WriteFile(tmpFile, []byte(tt.input), 0644)).To(Succeed())

			output, err := executeCommand([]string{"patch", "instance", "-f", tmpFile, "-c", "kustomize-controller", "--rm"})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(ContainSubstring(tt.wantOutput))

			result, err := os.ReadFile(tmpFile)
			g.Expect(err).ToNot(HaveOccurred())
			want := tt.want
			if want == "" {
				want = tt.input
			}
			requireEqualStrings(t, string(result), want)
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
			name:        "unsupported major version",
			args:        []string{"patch", "instance", "-f", "testdata/patch_instance/input.yaml", "-v", "v3.8"},
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

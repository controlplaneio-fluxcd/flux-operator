// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

/*
To regenerate the golden file:
make cli-build
./bin/flux-operator-cli build resourceset \
-f cmd/cli/testdata/build_resourceset/rset-with-rsip.yaml \
--inputs-from-provider cmd/cli/testdata/build_resourceset/rsip.yaml \
> cmd/cli/testdata/build_resourceset/golden.yaml
*/

func TestBuildResourceSetCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "standalone resourceset",
			args: []string{"build", "resourceset", "-f", "testdata/build_resourceset/rset-standalone.yaml"},
		},
		{
			name: "resourceset with input provider",
			args: []string{"build", "resourceset", "-f", "testdata/build_resourceset/rset-with-rsip.yaml", "--inputs-from-provider", "testdata/build_resourceset/rsip.yaml"},
		},
		{
			name:        "no filename flag",
			args:        []string{"build", "resourceset"},
			expectError: true,
			errorMsg:    "--filename is required",
		},
		{
			name:        "invalid filename",
			args:        []string{"build", "resourceset", "-f", "nonexistent.yaml"},
			expectError: true,
			errorMsg:    "must point to an existing file",
		},
		{
			name:        "resourceset with inputsFrom but no provider",
			args:        []string{"build", "resourceset", "-f", "testdata/build_resourceset/rset-with-rsip.yaml"},
			expectError: true,
			errorMsg:    "please provide the inputs with --inputs-from",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.errorMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errorMsg))
				}
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).ToNot(BeEmpty())

			// Read expected golden output
			expectedBytes, err := os.ReadFile("testdata/build_resourceset/golden.yaml")
			g.Expect(err).ToNot(HaveOccurred())
			expected := string(expectedBytes)

			g.Expect(output).To(Equal(expected))
		})
	}
}

func TestBuildResourceSetCmdWithInputsFile(t *testing.T) {
	g := NewWithT(t)
	rsetFile := "testdata/build_resourceset/rset-with-rsip.yaml"
	inputsFile := "testdata/build_resourceset/inputs.yaml"

	// Execute command with inputs file
	output, err := executeCommand([]string{"build", "rset", "-f", rsetFile, "--inputs-from", inputsFile})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).ToNot(BeEmpty())

	// Read expected golden output
	expectedBytes, err := os.ReadFile("testdata/build_resourceset/golden.yaml")
	g.Expect(err).ToNot(HaveOccurred())
	expected := string(expectedBytes)

	g.Expect(output).To(Equal(expected))
}

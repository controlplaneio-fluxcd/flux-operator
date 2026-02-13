// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDiffYAMLCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		goldenFile  string
		expectError string
	}{
		{
			name:       "json-patch-yaml output (default)",
			args:       []string{"diff", "yaml", "testdata/diff_yaml/source.yaml", "testdata/diff_yaml/target.yaml"},
			goldenFile: "testdata/diff_yaml/golden-json-patch-yaml.yaml",
		},
		{
			name:       "json-patch output",
			args:       []string{"diff", "yaml", "testdata/diff_yaml/source.yaml", "testdata/diff_yaml/target.yaml", "--output=json-patch"},
			goldenFile: "testdata/diff_yaml/golden-json-patch.json",
		},
		{
			name: "identical files",
			args: []string{"diff", "yaml", "testdata/diff_yaml/source.yaml", "testdata/diff_yaml/identical.yaml"},
		},
		{
			name:        "missing args",
			args:        []string{"diff", "yaml"},
			expectError: "accepts 2 arg(s), received 0",
		},
		{
			name:        "single arg",
			args:        []string{"diff", "yaml", "testdata/diff_yaml/source.yaml"},
			expectError: "accepts 2 arg(s), received 1",
		},
		{
			name:        "nonexistent source file",
			args:        []string{"diff", "yaml", "nonexistent.yaml", "testdata/diff_yaml/target.yaml"},
			expectError: "fetching source",
		},
		{
			name:        "nonexistent target file",
			args:        []string{"diff", "yaml", "testdata/diff_yaml/source.yaml", "nonexistent.yaml"},
			expectError: "fetching target",
		},
		{
			name:        "unsupported output format",
			args:        []string{"diff", "yaml", "testdata/diff_yaml/source.yaml", "testdata/diff_yaml/target.yaml", "--output=text"},
			expectError: "unsupported output format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			output, err := executeCommand(tt.args)

			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			if tt.goldenFile != "" {
				expected, err := os.ReadFile(tt.goldenFile)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(output).To(Equal(string(expected)))
			} else {
				g.Expect(output).To(ContainSubstring("null"))
			}
		})
	}
}

func TestDiffYAMLCmd_RemoteURL(t *testing.T) {
	g := NewWithT(t)

	sourceData, err := os.ReadFile("testdata/diff_yaml/source.yaml")
	g.Expect(err).ToNot(HaveOccurred())
	targetData, err := os.ReadFile("testdata/diff_yaml/target.yaml")
	g.Expect(err).ToNot(HaveOccurred())

	mux := http.NewServeMux()
	mux.HandleFunc("/source.yaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sourceData)
	})
	mux.HandleFunc("/target.yaml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(targetData)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	output, err := executeCommand([]string{
		"diff", "yaml",
		fmt.Sprintf("%s/source.yaml", ts.URL),
		fmt.Sprintf("%s/target.yaml", ts.URL),
		"--output=json-patch",
	})

	g.Expect(err).ToNot(HaveOccurred())

	expected, err := os.ReadFile("testdata/diff_yaml/golden-json-patch.json")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(Equal(string(expected)))
}

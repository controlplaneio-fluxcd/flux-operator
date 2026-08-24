// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseFieldPaths(t *testing.T) {
	tests := []struct {
		testName string
		fields   []string
		exprs    []string
		prefixes [][]string
		matchErr string
	}{
		{
			testName: "no fields",
			exprs:    []string{},
			prefixes: [][]string{},
		},
		{
			testName: "optional braces and leading dot",
			fields:   []string{"spec.chart.spec.version", " .status.conditions ", "{.metadata.name}"},
			exprs:    []string{"spec.chart.spec.version", "status.conditions", "metadata.name"},
			prefixes: [][]string{
				{"spec", "chart", "spec", "version"},
				{"status", "conditions"},
				{"metadata", "name"},
			},
		},
		{
			testName: "prefix stops at the first non-field node",
			fields: []string{
				"status.conditions[?(@.type=='Ready')].message",
				".status.history[0].chartVersion",
				"{.status.conditions[?(@.message==\"{}\")].type}",
				"{..message}",
				"['spec'].interval",
			},
			exprs: []string{
				"status.conditions[?(@.type=='Ready')].message",
				"status.history[0].chartVersion",
				"status.conditions[?(@.message==\"{}\")].type",
				"..message",
				"['spec'].interval",
			},
			prefixes: [][]string{
				{"status", "conditions"},
				{"status", "history"},
				{"status", "conditions"},
				nil,
				{"spec", "interval"},
			},
		},
		{
			testName: "empty field",
			fields:   []string{"spec", " "},
			matchErr: "field path must not be empty",
		},
		{
			testName: "empty field after normalisation",
			fields:   []string{"{ . }"},
			matchErr: "field path must not be empty",
		},
		{
			testName: "multiple expressions",
			fields:   []string{"{.spec}{.status}"},
			matchErr: "invalid field path '{.spec}{.status}': must contain a single expression",
		},
		{
			testName: "range template",
			fields:   []string{"{range .status.conditions[*]}{.type}{end}"},
			matchErr: "must contain a single expression",
		},
		{
			testName: "invalid expression",
			fields:   []string{"{.spec"},
			matchErr: "invalid field path '{.spec'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			paths, err := ParseFieldPaths(tt.fields)
			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(paths).To(HaveLen(len(tt.prefixes)))
			for i, path := range paths {
				g.Expect(path.expr).To(Equal(tt.exprs[i]))
				g.Expect(path.prefix).To(Equal(tt.prefixes[i]))
			}
		})
	}
}

func TestFieldPaths_Covers(t *testing.T) {
	tests := []struct {
		testName string
		fields   []string
		field    []string
		covered  bool
	}{
		{
			testName: "empty covers everything",
			field:    []string{"status", "events"},
			covered:  true,
		},
		{
			testName: "exact match",
			fields:   []string{"spec", "status.events"},
			field:    []string{"status", "events"},
			covered:  true,
		},
		{
			testName: "expression is a prefix of the field",
			fields:   []string{"status"},
			field:    []string{"status", "events"},
			covered:  true,
		},
		{
			testName: "field is a prefix of the expression",
			fields:   []string{"status.events[*].message"},
			field:    []string{"status", "events"},
			covered:  true,
		},
		{
			testName: "sibling does not match",
			fields:   []string{"status.conditions[?(@.type=='Ready')].message", "spec"},
			field:    []string{"status", "events"},
			covered:  false,
		},
		{
			testName: "recursive descent covers everything",
			fields:   []string{"{..message}"},
			field:    []string{"status", "events"},
			covered:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			paths, err := ParseFieldPaths(tt.fields)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(paths.Covers(tt.field...)).To(Equal(tt.covered))
		})
	}
}

func TestFieldPaths_Project(t *testing.T) {
	obj := map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      "podinfo",
			"namespace": "default",
			"labels":    map[string]any{"app": "podinfo"},
		},
		"spec": map[string]any{
			"interval": "10m",
			"chart": map[string]any{
				"spec": map[string]any{
					"chart":   "podinfo",
					"version": "6.x",
				},
			},
		},
		"status": map[string]any{
			"lastAttemptedRevision": "6.9.0",
			"observedGeneration":    nil,
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "message": "install succeeded"},
				map[string]any{"type": "Released", "status": "True", "message": "release succeeded"},
			},
			"history": []any{
				map[string]any{"version": int64(2), "chartVersion": "6.9.0"},
				map[string]any{"version": int64(1), "chartVersion": "6.8.0"},
			},
		},
	}

	identity := map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      "podinfo",
			"namespace": "default",
		},
	}

	withIdentity := func(fields map[string]any) map[string]any {
		result := make(map[string]any, len(identity)+len(fields))
		for k, v := range identity {
			result[k] = v
		}
		for k, v := range fields {
			result[k] = v
		}
		return result
	}

	tests := []struct {
		testName string
		fields   []string
		expected map[string]any
	}{
		{
			testName: "no fields returns object unchanged",
			expected: obj,
		},
		{
			testName: "nested fields keyed by expression",
			fields:   []string{"spec.chart.spec.version", ".status.lastAttemptedRevision", "status.observedGeneration"},
			expected: withIdentity(map[string]any{
				"spec.chart.spec.version":      "6.x",
				"status.lastAttemptedRevision": "6.9.0",
				"status.observedGeneration":    nil,
			}),
		},
		{
			testName: "top level fields keep their shape",
			fields:   []string{"spec", "status.conditions"},
			expected: withIdentity(map[string]any{
				"spec": obj["spec"],
				"status.conditions": []any{
					map[string]any{"type": "Ready", "status": "True", "message": "install succeeded"},
					map[string]any{"type": "Released", "status": "True", "message": "release succeeded"},
				},
			}),
		},
		{
			testName: "filter index and wildcard",
			fields: []string{
				"status.conditions[?(@.type=='Ready')].message",
				"{.status.conditions[?(@.type==\"Released\")].status}",
				"status.history[0].chartVersion",
				"status.history[*].chartVersion",
				"status.history[-1:].version",
				"status.history[0:2].version",
				"status.conditions[0,1].type",
				"..chartVersion",
				"['spec'].interval",
			},
			expected: withIdentity(map[string]any{
				"status.conditions[?(@.type=='Ready')].message":     "install succeeded",
				"status.conditions[?(@.type==\"Released\")].status": "True",
				"status.history[0].chartVersion":                    "6.9.0",
				"status.history[*].chartVersion":                    []any{"6.9.0", "6.8.0"},
				"status.history[-1:].version":                       int64(1),
				"status.history[0:2].version":                       []any{int64(2), int64(1)},
				"status.conditions[0,1].type":                       []any{"Ready", "Released"},
				"..chartVersion":                                    []any{"6.9.0", "6.8.0"},
				"['spec'].interval":                                 "10m",
			}),
		},
		{
			testName: "unknown fields and paths into lists are omitted",
			fields:   []string{"spec.values", "status.inventory", "status.conditions.type", "status.history[5].version"},
			expected: identity,
		},
		{
			testName: "metadata replaces the identity",
			fields:   []string{"metadata"},
			expected: map[string]any{
				"apiVersion": "helm.toolkit.fluxcd.io/v2",
				"kind":       "HelmRelease",
				"metadata":   obj["metadata"],
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			paths, err := ParseFieldPaths(tt.fields)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(paths.Project(obj)).To(Equal(tt.expected))
		})
	}
}

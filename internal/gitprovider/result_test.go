// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

func TestMakeInputs(t *testing.T) {
	tests := []struct {
		name     string
		results  []Result
		defaults map[string]any
		want     string
	}{
		{
			name: "results without defaults",
			results: []Result{
				{
					ID:     "1433470881",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Branch: "patch-1",
				},
				{
					ID:     "1433536418",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Branch: "patch-2",
				},
			},
			defaults: nil,
			want: `
- id: "1433470881"
  sha: "2dd3a8d2088457e5cf991018edf13e25cbd61380"
  branch: "patch-1"
- id: "1433536418"
  sha: "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8"
  branch: "patch-2"
`,
		},
		{
			name: "results with defaults",
			results: []Result{
				{
					ID:     "1433470881",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Branch: "patch-1",
					Title:  "my title",
				},
				{
					ID:     "1433536418",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Branch: "patch-2",
				},
			},
			defaults: map[string]any{
				"title":   "some title",
				"boolean": true,
				"numbers": []int{1, 2},
			},
			want: `
- id: "1433470881"
  sha: "2dd3a8d2088457e5cf991018edf13e25cbd61380"
  branch: "patch-1"
  title: "my title"
  boolean: true
  numbers:
  - 1
  - 2
- id: "1433536418"
  sha: "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8"
  branch: "patch-2"
  title: "some title"
  boolean: true
  numbers:
  - 1
  - 2
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := MakeInputs(tt.results, tt.defaults)
			g.Expect(err).NotTo(HaveOccurred())

			gotData, err := yaml.Marshal(got)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(string(gotData)).To(MatchYAML(tt.want))
		})
	}
}

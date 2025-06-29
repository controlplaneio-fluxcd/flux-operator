// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package filtering_test

import (
	"regexp"
	"testing"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

func TestFilters_MatchLabels(t *testing.T) {
	for _, tt := range []struct {
		name         string
		filterLabels []string
		labels       []string
		want         bool
	}{
		{
			name:         "no filter labels",
			filterLabels: nil,
			labels:       []string{"foo", "bar"},
			want:         true,
		},
		{
			name:         "empty filter labels",
			filterLabels: []string{},
			labels:       []string{"foo", "bar"},
			want:         true,
		},
		{
			name:         "match all labels",
			filterLabels: []string{"foo", "bar"},
			labels:       []string{"foo", "bar"},
			want:         true,
		},
		{
			name:         "match some labels",
			filterLabels: []string{"foo", "baz"},
			labels:       []string{"foo", "bar"},
			want:         false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := &filtering.Filters{Labels: tt.filterLabels}
			got := f.MatchLabels(tt.labels)

			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestFilters_MatchString(t *testing.T) {
	for _, tt := range []struct {
		name    string
		include *regexp.Regexp
		exclude *regexp.Regexp
		input   string
		want    bool
	}{
		{
			name:    "no include or exclude",
			include: nil,
			exclude: nil,
			input:   "main",
			want:    true,
		},
		{
			name:    "include only",
			include: regexp.MustCompile(`^main$`),
			exclude: nil,
			input:   "main",
			want:    true,
		},
		{
			name:    "include only - no match",
			include: regexp.MustCompile(`^main$`),
			exclude: nil,
			input:   "develop",
			want:    false,
		},
		{
			name:    "exclude only",
			include: nil,
			exclude: regexp.MustCompile(`^develop$`),
			input:   "main",
			want:    true,
		},
		{
			name:    "exclude only - match",
			include: nil,
			exclude: regexp.MustCompile(`^develop$`),
			input:   "develop",
			want:    false,
		},
		{
			name:    "include and exclude",
			include: regexp.MustCompile(`^main$`),
			exclude: regexp.MustCompile(`^develop$`),
			input:   "main",
			want:    true,
		},
		{
			name:    "include and exclude - no match",
			include: regexp.MustCompile(`^main$`),
			exclude: regexp.MustCompile(`^develop$`),
			input:   "develop",
			want:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := &filtering.Filters{
				Include: tt.include,
				Exclude: tt.exclude,
			}
			got := f.MatchString(tt.input)

			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestFilters_Tags(t *testing.T) {
	newConstraint := func(s string) *semver.Constraints {
		c, err := semver.NewConstraint(s)
		if err != nil {
			panic(err)
		}
		return c
	}

	for _, tt := range []struct {
		name     string
		filters  *filtering.Filters
		tags     []string
		expected []string
	}{
		{
			name:     "no filters - sorts in reverse alphabetical order",
			filters:  &filtering.Filters{},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0"},
			expected: []string{"v2.0.0", "v1.1.0", "v1.0.0"},
		},
		{
			name: "include filter - sorts in reverse alphabetical order",
			filters: &filtering.Filters{
				Include: regexp.MustCompile(`^v1\.`),
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0"},
			expected: []string{"v1.1.0", "v1.0.0"},
		},
		{
			name: "exclude filter - sorts in reverse alphabetical order",
			filters: &filtering.Filters{
				Exclude: regexp.MustCompile(`^v2\.`),
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0"},
			expected: []string{"v1.1.0", "v1.0.0"},
		},
		{
			name: "include and exclude filters - sorts in reverse alphabetical order",
			filters: &filtering.Filters{
				Include: regexp.MustCompile(`^v1\.`),
				Exclude: regexp.MustCompile(`^v1\.0\.0$`),
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "v1.2.0"},
			expected: []string{"v1.2.0", "v1.1.0"},
		},
		{
			name: "semver filter - sorts in reverse order",
			filters: &filtering.Filters{
				SemVer: newConstraint(">= 1.0.0 < 2.0.0"),
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "v1.2.0"},
			expected: []string{"v1.2.0", "v1.1.0", "v1.0.0"},
		},
		{
			name: "limit filter - limits the number of results",
			filters: &filtering.Filters{
				Limit: 2,
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "v1.2.0"},
			expected: []string{"v2.0.0", "v1.2.0"},
		},
		{
			name: "semver and limit filters - sorts and limits the number of results",
			filters: &filtering.Filters{
				SemVer: newConstraint(">= 1.0.0 < 2.0.0"),
				Limit:  1,
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "v1.2.0"},
			expected: []string{"v1.2.0"},
		},
		{
			name: "include, exclude, semver and limit filters - applies all filters",
			filters: &filtering.Filters{
				Include: regexp.MustCompile(`^v1\.`),
				Exclude: regexp.MustCompile(`^v1\.0\.0$`),
				SemVer:  newConstraint(">= 1.0.0 < 2.0.0"),
				Limit:   2,
			},
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "v1.2.0", "rubbish", "garbage"},
			expected: []string{"v1.2.0", "v1.1.0"},
		},
		{
			name: "calver works with alphabetical sorting",
			filters: &filtering.Filters{
				Limit: 1,
			},
			tags:     []string{"2024.01.01", "2024.01.02", "2024.01.03"},
			expected: []string{"2024.01.03"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := tt.filters.Tags(tt.tags)

			g.Expect(got).To(Equal(tt.expected))
		})
	}
}

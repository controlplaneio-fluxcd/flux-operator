// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package filtering

import (
	"math/big"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/fluxcd/pkg/version"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// DefaultLimit is the default limit for the number of results returned by the filters.
const DefaultLimit = 100

const (
	// OrderByDesc sorts tags in descending order.
	OrderByDesc = "desc"
	// OrderByAsc sorts tags in ascending order.
	OrderByAsc = "asc"
)

// Filters holds the filters for the input provider responses.
type Filters struct {
	// Labels is used for a "match labels" filter.
	Labels []string

	// Include is used for including tags or branches.
	Include *regexp.Regexp

	// Exclude is used for excluding tags or branches.
	Exclude *regexp.Regexp

	// SemVer is used for sorting and filtering tags.
	// Supported only for tags at the moment.
	SemVer *semver.Constraints

	// Pattern is used to match tags before sorting.
	Pattern *regexp.Regexp

	// Extract is the replacement template applied to Pattern matches to derive
	// the sortable value for tags.
	Extract string

	// OrderBy determines whether tags are sorted in ascending or descending order.
	// Supported values are "asc" and "desc". Defaults to "desc".
	OrderBy string

	// Limit is used to limit the number of results.
	Limit int
}

// MatchLabels returns true if the given labels include all the label filters.
func (f *Filters) MatchLabels(labels []string) bool {
	for _, label := range f.Labels {
		if !slices.Contains(labels, label) {
			return false
		}
	}
	return true
}

// MatchString returns true if the string matches the include and exclude regex filters.
func (f *Filters) MatchString(s string) bool {
	if f.Include != nil {
		if !f.Include.MatchString(s) {
			return false
		}
	}
	if f.Exclude != nil {
		if f.Exclude.MatchString(s) {
			return false
		}
	}
	return true
}

// Tags applies all the filters supported for tags to a list of tags.
// nolint:prealloc
func (f *Filters) Tags(tags []string) []string {
	type parsedTag struct {
		name string
		key  string
	}
	filtered := make([]parsedTag, 0, len(tags))

	// Apply include and exclude.
	for _, tag := range tags {
		if !f.MatchString(tag) {
			continue
		}

		key, ok := f.TagSortKey(tag)
		if !ok {
			continue
		}

		filtered = append(filtered, parsedTag{name: tag, key: key})
	}

	// Apply semver or sort in reverse alphabetical order. Keep input order
	// stable for tags with equal semantic-version precedence.
	switch {
	case f.SemVer != nil:
		type semverTag struct {
			name    string
			version *semver.Version
		}
		parsed := make([]semverTag, 0, len(filtered))
		for _, tag := range filtered {
			parsedVersion, err := version.ParseVersion(tag.key)
			if err == nil && f.SemVer.Check(parsedVersion) {
				parsed = append(parsed, semverTag{name: tag.name, version: parsedVersion})
			}
		}
		sort.SliceStable(parsed, func(i, j int) bool {
			if parsed[i].version.Equal(parsed[j].version) {
				return false
			}
			if f.orderByAscending() {
				return parsed[i].version.LessThan(parsed[j].version)
			}
			return parsed[i].version.GreaterThan(parsed[j].version)
		})
		filtered = filtered[:0]
		for _, tag := range parsed {
			filtered = append(filtered, parsedTag{name: tag.name})
		}
	default:
		sort.SliceStable(filtered, func(i, j int) bool {
			cmp := CompareTagSortKeys(filtered[i].key, filtered[j].key)
			if cmp == 0 {
				return false
			}
			if f.orderByAscending() {
				return cmp < 0
			}
			return cmp > 0
		})
	}

	// Apply limit.
	lim := fluxcdv1.DefaultResourceSetInputProviderFilterLimit
	if f.Limit > 0 {
		lim = f.Limit
	}
	lim = min(lim, len(filtered))
	res := make([]string, 0, lim)
	for _, tag := range filtered[:lim] {
		res = append(res, tag.name)
	}
	return res
}

// TagSortKey returns the sortable key for a tag.
// When Pattern is set, only matching tags are included. If Extract is set, the
// sortable key is derived from the replacement template.
func (f *Filters) TagSortKey(tag string) (string, bool) {
	if f.Pattern == nil {
		return tag, true
	}

	match := f.Pattern.FindStringSubmatchIndex(tag)
	if match == nil {
		return "", false
	}

	if f.Extract == "" {
		return tag, true
	}

	key := f.Pattern.ExpandString(nil, f.Extract, tag, match)
	return string(key), true
}

func (f *Filters) orderByAscending() bool {
	return strings.EqualFold(f.OrderBy, OrderByAsc)
}

// CompareTagSortKeys compares two tag sort keys.
// When both keys are base-10 integers, comparison is numeric; otherwise lexical.
func CompareTagSortKeys(a, b string) int {
	av, aok := new(big.Int).SetString(a, 10)
	bv, bok := new(big.Int).SetString(b, 10)
	if aok && bok {
		return av.Cmp(bv)
	}
	return strings.Compare(a, b)
}

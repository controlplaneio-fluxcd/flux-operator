// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package filtering

import (
	"regexp"
	"slices"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/fluxcd/pkg/version"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// DefaultLimit is the default limit for the number of results returned by the filters.
const DefaultLimit = 100

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

		key, ok := f.extractTagSortKey(tag)
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
			return parsed[i].version.GreaterThan(parsed[j].version)
		})
		filtered = filtered[:0]
		for _, tag := range parsed {
			filtered = append(filtered, parsedTag{name: tag.name})
		}
	default:
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].key > filtered[j].key
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

// extractTagSortKey returns the sortable key for a tag.
// When Pattern is set, only matching tags are included. If Extract is set, the
// sortable key is derived from the replacement template.
func (f *Filters) extractTagSortKey(tag string) (string, bool) {
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

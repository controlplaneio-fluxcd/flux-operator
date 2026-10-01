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
	// OrderBySemVer sorts tags by semantic version precedence.
	OrderBySemVer = "SemVer"
	// OrderByAlphabetical sorts tags in ascending lexical order.
	OrderByAlphabetical = "Alphabetical"
	// OrderByReverseAlphabetical sorts tags in descending lexical order.
	OrderByReverseAlphabetical = "ReverseAlphabetical"
	// OrderByNumerical sorts tags in ascending numeric order.
	OrderByNumerical = "Numerical"
	// OrderByReverseNumerical sorts tags in descending numeric order.
	OrderByReverseNumerical = "ReverseNumerical"
)

// Filters holds the filters for the input provider responses.
type Filters struct {
	// Labels is used for a "match labels" filter.
	Labels []string

	// Include is used for including tags or branches.
	Include *regexp.Regexp

	// Exclude is used for excluding tags or branches.
	Exclude *regexp.Regexp

	// SemVer is used for filtering and sorting tags by semantic version
	// precedence.
	// Supported only for tags at the moment.
	SemVer *semver.Constraints

	// ExtractOrder is the replacement template applied to Include matches to
	// derive the sortable value for tags.
	ExtractOrder string

	// ExtractGroup is the replacement template applied to Include matches to
	// derive the group key for tags.
	ExtractGroup string

	// OrderBy determines how tags are sorted.
	// Supported values are SemVer, Alphabetical, ReverseAlphabetical, Numerical,
	// and ReverseNumerical. Defaults to ReverseAlphabetical unless SemVer is set,
	// in which case SemVer is used when OrderBy is empty.
	OrderBy string

	// Limit is used to limit the number of results.
	Limit int
}

type tagSortValue struct {
	name     string
	orderKey string
	groupKey string
	version  *semver.Version
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
	filtered := make([]tagSortValue, 0, len(tags))
	orderBy := f.effectiveOrderBy()

	// Apply include and exclude.
	for _, tag := range tags {
		if !f.MatchString(tag) {
			continue
		}

		orderKey, groupKey, ok := f.TagKeys(tag)
		if !ok {
			continue
		}

		parsed := tagSortValue{name: tag, orderKey: orderKey, groupKey: groupKey}
		if orderBy == OrderBySemVer {
			parsedVersion, err := version.ParseVersion(orderKey)
			if err != nil {
				continue
			}
			if f.SemVer != nil && !f.SemVer.Check(parsedVersion) {
				continue
			}
			parsed.version = parsedVersion
		} else if f.SemVer != nil {
			parsedVersion, err := version.ParseVersion(orderKey)
			if err != nil || !f.SemVer.Check(parsedVersion) {
				continue
			}
			parsed.version = parsedVersion
		}
		filtered = append(filtered, parsed)
	}

	grouped := make(map[string][]tagSortValue)
	for _, tag := range filtered {
		grouped[tag.groupKey] = append(grouped[tag.groupKey], tag)
	}

	groupKeys := make([]string, 0, len(grouped))
	for key := range grouped {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	lim := fluxcdv1.DefaultResourceSetInputProviderFilterLimit
	if f.Limit > 0 {
		lim = f.Limit
	}
	res := make([]string, 0, min(lim, len(filtered)))
	for _, groupKey := range groupKeys {
		tags := grouped[groupKey]
		sort.SliceStable(tags, func(i, j int) bool {
			cmp := compareTagSortValues(orderBy, tags[i], tags[j])
			if cmp == 0 {
				return false
			}
			return cmp > 0
		})
		limit := min(lim, len(tags))
		for _, tag := range tags[:limit] {
			res = append(res, tag.name)
		}
	}
	return res
}

// TagKeys returns the sortable and grouping keys for a tag.
// When Include is set, only matching tags are included. If ExtractOrder or
// ExtractGroup is set, the keys are derived from the replacement templates.
func (f *Filters) TagKeys(tag string) (orderKey, groupKey string, ok bool) {
	if f.Include == nil {
		if f.ExtractOrder != "" || f.ExtractGroup != "" {
			return "", "", false
		}
		return tag, "", true
	}

	match := f.Include.FindStringSubmatchIndex(tag)
	if match == nil {
		return "", "", false
	}

	orderKey = tag
	if f.ExtractOrder != "" {
		orderKey = string(f.Include.ExpandString(nil, f.ExtractOrder, tag, match))
	}
	if f.ExtractGroup != "" {
		groupKey = string(f.Include.ExpandString(nil, f.ExtractGroup, tag, match))
	}
	return orderKey, groupKey, true
}

// TagSortKey returns the sortable key for a tag.
// This is a legacy helper retained for internal callers.
func (f *Filters) TagSortKey(tag string) (string, bool) {
	key, _, ok := f.TagKeys(tag)
	return key, ok
}

func (f *Filters) effectiveOrderBy() string {
	if f.OrderBy != "" {
		return f.OrderBy
	}
	if f.SemVer != nil {
		return OrderBySemVer
	}
	return OrderByReverseAlphabetical
}

func compareTagSortValues(orderBy string, a, b tagSortValue) int {
	switch orderBy {
	case OrderBySemVer:
		switch {
		case a.version.LessThan(b.version):
			return -1
		case a.version.GreaterThan(b.version):
			return 1
		default:
			return 0
		}
	case OrderByAlphabetical:
		return strings.Compare(b.orderKey, a.orderKey)
	case OrderByReverseAlphabetical:
		return strings.Compare(a.orderKey, b.orderKey)
	case OrderByNumerical:
		return CompareNumericKeys(b.orderKey, a.orderKey)
	case OrderByReverseNumerical:
		return CompareNumericKeys(a.orderKey, b.orderKey)
	default:
		return strings.Compare(a.orderKey, b.orderKey)
	}
}

// CompareNumericKeys compares two tag sort keys.
// When both keys are base-10 integers, comparison is numeric; otherwise lexical.
func CompareNumericKeys(a, b string) int {
	av, aok := new(big.Int).SetString(a, 10)
	bv, bok := new(big.Int).SetString(b, 10)
	if aok && bok {
		return av.Cmp(bv)
	}
	return strings.Compare(a, b)
}

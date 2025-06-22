// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package filtering

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

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
	// When set together with Value or GroupKey, can be used
	// to expand a value or group key from the branch or tag
	// name for usage with another filter.
	Include *regexp.Regexp

	// Exclude is used for excluding tags or branches.
	Exclude *regexp.Regexp

	// Value is an optional template that can be expanded using the regex
	// matching results of Include against a branch or tag to expand a value
	// that can be used with another filter.
	// Supported by the SemVer and LatestPolicy filters.
	Value string

	// GroupKey is an optional template that can be expanded using the regex
	// matching results of Include against a branch or tag to expand a value
	// that can be used with another filter.
	// Supported only by the LatestPolicy filter.
	GroupKey string

	// LatestPolicy is the order for sorting each group of tags.
	// After sorting each group, the last tag is selected as
	// the "latest" in the group. When this field is set, only
	// the latest tags from each group are exported as inputs.
	// Supported only for tags at the moment.
	LatestPolicy string

	// SemVer is used for sorting and filtering tags.
	// When both Value and Include are set, the expanded
	// value is checked against the SemVer range instead of
	// the tag name itself.
	// Supported only for tags at the moment.
	SemVer *semver.Constraints

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

// MatchBranch returns true if the branch matches the include and exclude regex filters.
func (f *Filters) MatchBranch(branch string) bool {
	if f.Include != nil {
		if !f.Include.MatchString(branch) {
			return false
		}
	}
	if f.Exclude != nil {
		if f.Exclude.MatchString(branch) {
			return false
		}
	}
	return true
}

// Tags applies all the filters supported for tags to a list of tags.
// nolint:prealloc
func (f *Filters) Tags(tags []string) []string {

	var filtered []string

	// Apply include and exclude, compute expanded values and group keys.
	var expanded []*tagOrBranchExpandedValueAndGroupKey
	expandTemplates := f.Include != nil && (f.Value != "" || f.GroupKey != "")
	for _, tag := range tags {
		if f.Exclude != nil && f.Exclude.MatchString(tag) {
			continue
		}
		if f.Include == nil || (!expandTemplates && f.Include.MatchString(tag)) {
			filtered = append(filtered, tag)
			continue
		}
		if !expandTemplates { // f.Include does not match tag.
			continue
		}
		// Match with Include was not tested above because
		// we need the indexes of the matches to expand
		// the value and/or group key.
		matching := f.Include.FindStringSubmatchIndex(tag)
		if len(matching) == 0 {
			continue
		}
		e := newExpandedTagOrBranch(tag)
		if f.Value != "" {
			e.value = string(f.Include.ExpandString(nil, f.Value, tag, matching))
		}
		if f.GroupKey != "" {
			e.groupKey = string(f.Include.ExpandString(nil, f.GroupKey, tag, matching))
		}
		filtered = append(filtered, tag)
		expanded = append(expanded, e)
	}

	// Apply semver.
	if f.SemVer != nil {
		hasValue := f.Include != nil && f.Value != ""
		semverList := filtered
		tagsByValue := make(map[string][]*tagOrBranchExpandedValueAndGroupKey)

		// Use the semver value from the tag if expansion is configured,
		// and store the original tags for the same expanded value in the
		// original order.
		if hasValue {
			semverList = nil
			for _, e := range expanded {
				value := e.value
				semverList = append(semverList, value)
				tagsByValue[value] = append(tagsByValue[value], e)
			}
		}

		// Filter and sort by semver.
		filtered = version.Sort(f.SemVer, semverList)

		// Recover the original tags for each expanded value
		// in the sorted order of semver, but keeping the original
		// order of the tags with the same expanded value.
		if hasValue {
			semverList = filtered
			filtered = nil
			expanded = nil
			for _, value := range semverList {
				for _, e := range tagsByValue[value] {
					filtered = append(filtered, e.tagOrBranch)
					expanded = append(expanded, e)
				}
			}
		}
	}

	// Apply latest policy.
	if f.LatestPolicy != "" {
		compare := getCompareFunc(f.LatestPolicy)

		if !expandTemplates { // Let's build the expanded slice with the defaults.
			expanded = make([]*tagOrBranchExpandedValueAndGroupKey, 0, len(filtered))
			for _, tag := range filtered {
				expanded = append(expanded, newExpandedTagOrBranch(tag))
			}
		}

		// Separate in groups. In this construction it's guaranteed that
		// each group will have at least one tag or branch.
		tagsByGroup := make(map[string][]*tagOrBranchExpandedValueAndGroupKey)
		for _, e := range expanded {
			tagsByGroup[e.groupKey] = append(tagsByGroup[e.groupKey], e)
		}

		// Sort groups.
		for _, group := range tagsByGroup {
			slices.SortFunc(group, compare)
		}

		// Select the latest tag from each group.
		filtered = make([]string, 0, len(tagsByGroup))
		for _, group := range tagsByGroup {
			filtered = append(filtered, group[0].tagOrBranch) // [0] is guaranteed
		}

		// Make output stable (map order is random)
		slices.Sort(filtered)

		return filtered
	}

	// Apply limit.
	lim := fluxcdv1.DefaultResourceSetInputProviderFilterLimit
	if f.Limit > 0 {
		lim = f.Limit
	}
	lim = min(lim, len(filtered))
	return slices.Clone(filtered[:lim])
}

type tagOrBranchExpandedValueAndGroupKey struct {
	tagOrBranch string
	value       string
	groupKey    string
}

// newExpandedTagOrBranch creates a default tagOrBranchExpandedValueAndGroupKey
// from a tag or branch name.
func newExpandedTagOrBranch(tagOrBranch string) *tagOrBranchExpandedValueAndGroupKey {
	return &tagOrBranchExpandedValueAndGroupKey{
		tagOrBranch: tagOrBranch,
		value:       tagOrBranch, // the default is the tag or branch itself
		groupKey:    "",          // the default is a single group with all tags or branches
	}
}

// compareSemVer compares two tagOrBranchExpandedValueAndGroupKey
// placing the latest version first.
func compareSemVer(a, b *tagOrBranchExpandedValueAndGroupKey) int {
	aVer, aErr := version.ParseVersion(a.value)
	bVer, bErr := version.ParseVersion(b.value)

	// If both versions are invalid, they are considered equal.
	if aErr != nil && bErr != nil {
		return 0
	}

	// We put valid versions before invalid ones.
	if bErr != nil {
		return -1
	}
	if aErr != nil {
		return 1
	}

	// We compare SemVer in reverse order as we want the first
	// element in a slice to be the latest version.
	return -aVer.Compare(bVer)
}

// compareAlphabetical compares two tagOrBranchExpandedValueAndGroupKey
// placing the alphabetically latest tag or branch first.
func compareAlphabetical(a, b *tagOrBranchExpandedValueAndGroupKey) int {
	return -strings.Compare(a.value, b.value)
}

// compareReverseAlphabetical compares two tagOrBranchExpandedValueAndGroupKey
// placing the alphabetically earliest tag or branch first.
func compareReverseAlphabetical(a, b *tagOrBranchExpandedValueAndGroupKey) int {
	return -compareAlphabetical(a, b)
}

// compareNumerical compares two tagOrBranchExpandedValueAndGroupKey
// placing the numerically latest tag or branch first.
func compareNumerical(a, b *tagOrBranchExpandedValueAndGroupKey) int {
	aNum, aErr := strconv.ParseFloat(a.value, 64)
	bNum, bErr := strconv.ParseFloat(b.value, 64)

	// If both values are invalid, they are considered equal.
	if aErr != nil && bErr != nil {
		return 0
	}

	// We put valid numbers before invalid ones.
	if bErr != nil {
		return -1
	}
	if aErr != nil {
		return 1
	}

	// We compare numbers in reverse order as we want the first
	// element in a slice to be the latest version.
	return -cmp.Compare(aNum, bNum)
}

// compareReverseNumerical compares two tagOrBranchExpandedValueAndGroupKey
// placing the numerically earliest tag or branch first.
func compareReverseNumerical(a, b *tagOrBranchExpandedValueAndGroupKey) int {
	return -compareNumerical(a, b)
}

// getCompareFunc assumes the latest policy is one of the
// fluxcdv1.LatestPolicy* constants and returns the appropriate
// comparison function for sorting tags or branches.
func getCompareFunc(latestPolicy string) func(a, b *tagOrBranchExpandedValueAndGroupKey) int {
	switch latestPolicy {
	case fluxcdv1.LatestPolicySemVer:
		return compareSemVer
	case fluxcdv1.LatestPolicyAlphabetical:
		return compareAlphabetical
	case fluxcdv1.LatestPolicyReverseAlphabetical:
		return compareReverseAlphabetical
	case fluxcdv1.LatestPolicyNumerical:
		return compareNumerical
	case fluxcdv1.LatestPolicyReverseNumerical:
		return compareReverseNumerical
	default:
		return nil
	}
}

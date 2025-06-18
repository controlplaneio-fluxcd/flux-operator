// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"crypto/x509"
	"regexp"
	"slices"
	"sort"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

// Options holds the configuration for the Git SaaS provider.
type Options struct {
	URL      string
	CertPool *x509.CertPool
	Token    string
	Filters  Filters
}

// Filters holds the filters for the Git SaaS responses.
type Filters struct {
	IncludeBranchRe   *regexp.Regexp
	ExcludeBranchRe   *regexp.Regexp
	Labels            []string
	Limit             int
	SemverConstraints *semver.Constraints
	Alphabetical      string
	Numerical         string
}

// matchBranch returns true if the branch matches the include and exclude regex filters.
func matchBranch(opt Options, branch string) bool {
	if opt.Filters.IncludeBranchRe != nil {
		if !opt.Filters.IncludeBranchRe.MatchString(branch) {
			return false
		}
	}

	if opt.Filters.ExcludeBranchRe != nil {
		if opt.Filters.ExcludeBranchRe.MatchString(branch) {
			return false
		}
	}
	return true
}

// matchLabels returns true if the given labels include all the label filters.
func matchLabels(opt Options, labels []string) bool {
	for _, label := range opt.Filters.Labels {
		if !slices.Contains(labels, label) {
			return false
		}
	}
	return true
}

// sortSemver filters the tags based the provided semver range
// and sorts them in descending order.
func sortSemver(constraint *semver.Constraints, tags []string) []string {
	var versions []*semver.Version
	for _, tag := range tags {
		if v, err := semver.NewVersion(tag); err == nil {
			if constraint.Check(v) {
				versions = append(versions, v)
			}
		}
	}

	if len(tags) == 0 || len(versions) == 0 {
		return nil
	}

	sort.Sort(sort.Reverse(semver.Collection(versions)))
	sortedTags := make([]string, 0, len(versions))
	for _, v := range versions {
		sortedTags = append(sortedTags, v.Original())
	}

	return sortedTags
}

// sortAlphabetically sorts the tags in ascending or descending order
// based on the specified alphabetical order.
func sortAlphabetically(alphabetical string, tags []string) []string {
	if alphabetical == "desc" {
		sort.Sort(sort.Reverse(sort.StringSlice(tags)))
	} else {
		sort.Strings(tags)
	}
	return tags
}

// sortNumerically sorts the tags in ascending or descending order
// based on the specified numerical order.
func sortNumerically(numerical string, tags []string) []string {
	var nums []float64
	m := make(map[float64]string)
	for _, tag := range tags {
		if n, err := strconv.ParseFloat(tag, 64); err == nil {
			nums = append(nums, n)
			m[n] = tag
		}
	}
	if numerical == "desc" {
		sort.Sort(sort.Reverse(sort.Float64Slice(nums)))
	} else {
		sort.Float64s(nums)
	}
	tags = make([]string, 0, len(nums))
	for _, n := range nums {
		tags = append(tags, m[n])
	}
	return tags
}

// sortTags sorts the tags based on the provided options.
func sortTags(opt Options, tags []string) []string {
	switch {
	case opt.Filters.SemverConstraints != nil:
		return sortSemver(opt.Filters.SemverConstraints, tags)
	case opt.Filters.Alphabetical != "":
		return sortAlphabetically(opt.Filters.Alphabetical, tags)
	case opt.Filters.Numerical != "":
		return sortNumerically(opt.Filters.Numerical, tags)
	default:
		return tags
	}
}

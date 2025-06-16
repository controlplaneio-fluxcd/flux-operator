// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"crypto/x509"
	"regexp"
	"slices"
	"sort"

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
func sortSemver(opt Options, tags []string) []string {
	constraint := opt.Filters.SemverConstraints
	if constraint == nil {
		return tags
	}

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

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"crypto/x509"
	"regexp"
	"slices"
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
	IncludeBranchRx *regexp.Regexp
	ExcludeBranchRx *regexp.Regexp
	Labels          []string
	Limit           int
}

// matchBranch returns true if the branch matches the include and exclude regex filters.
func matchBranch(opt Options, branch string) bool {
	if opt.Filters.IncludeBranchRx != nil {
		if !opt.Filters.IncludeBranchRx.MatchString(branch) {
			return false
		}
	}

	if opt.Filters.ExcludeBranchRx != nil {
		if opt.Filters.ExcludeBranchRx.MatchString(branch) {
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

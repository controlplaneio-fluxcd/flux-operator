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

// Filters holds the filters for the pull/merge requests.
type Filters struct {
	SourceBranchRx *regexp.Regexp
	TargetBranchRx *regexp.Regexp
	Labels         []string
	Limit          int
}

func matchBranches(opt Options, sourceBranch, targetBranch string) bool {
	if opt.Filters.SourceBranchRx != nil {
		if !opt.Filters.SourceBranchRx.MatchString(sourceBranch) {
			return false
		}
	}
	if opt.Filters.TargetBranchRx != nil {
		if !opt.Filters.TargetBranchRx.MatchString(targetBranch) {
			return false
		}
	}
	return true
}

func includesLabels(opt Options, labels []string) bool {
	for _, label := range opt.Filters.Labels {
		if !slices.Contains(labels, label) {
			return false
		}
	}
	return true
}

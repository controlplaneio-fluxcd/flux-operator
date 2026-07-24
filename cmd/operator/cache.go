// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// resolveWatchNamespaces converts the raw --watch-namespaces flag values into a
// sorted, de-duplicated namespace set used to scope ResourceSet /
// ResourceSetInputProvider reconciliation. Each value may be repeated and/or
// comma-separated. The operator's runtime namespace is always included.
//
// It returns (nil, nil) when no scoping is requested (empty input). It returns
// an error if any token is not a valid DNS-1123 label, or if values were given
// but none resolved to a non-empty namespace (so a typo like
// "--watch-namespaces=," fails loudly instead of silently scoping to just the
// runtime namespace).
func resolveWatchNamespaces(runtimeNamespace string, watchNamespaces []string) ([]string, error) {
	if len(watchNamespaces) == 0 {
		return nil, nil
	}
	set := map[string]struct{}{runtimeNamespace: {}}
	valid := 0
	for _, entry := range watchNamespaces {
		// Accept both repeated flags and comma-separated values.
		for _, ns := range strings.Split(entry, ",") {
			ns = strings.TrimSpace(ns)
			if ns == "" {
				continue
			}
			if errs := validation.IsDNS1123Label(ns); len(errs) > 0 {
				return nil, fmt.Errorf("invalid namespace %q: %s", ns, strings.Join(errs, "; "))
			}
			set[ns] = struct{}{}
			valid++
		}
	}
	if valid == 0 {
		return nil, fmt.Errorf("--watch-namespaces was set but contained no valid namespaces")
	}
	out := make([]string, 0, len(set))
	for ns := range set {
		out = append(out, ns)
	}
	sort.Strings(out)
	return out, nil
}

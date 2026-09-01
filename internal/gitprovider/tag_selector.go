// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"container/heap"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	fluxversion "github.com/fluxcd/pkg/version"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

// tagCandidate associates a provider result with its sortable tag metadata.
type tagCandidate struct {
	name     string
	key      string
	groupKey string
	result   Result
	version  *semver.Version
	sequence uint64
}

// tagCandidateHeap keeps the lowest-ranked retained tag at its root.
type tagCandidateHeap struct {
	items   []*tagCandidate
	orderBy string
}

// Len returns the number of retained tag candidates.
func (h tagCandidateHeap) Len() int { return len(h.items) }

// Less reports whether the first candidate ranks below the second.
func (h tagCandidateHeap) Less(i, j int) bool {
	return compareTagCandidates(h.items[i], h.items[j], h.orderBy) < 0
}

// Swap exchanges two retained tag candidates.
func (h tagCandidateHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

// Push adds a tag candidate to the heap.
func (h *tagCandidateHeap) Push(value any) {
	h.items = append(h.items, value.(*tagCandidate))
}

// Pop removes and returns the final tag candidate from the heap.
func (h *tagCandidateHeap) Pop() any {
	last := len(h.items) - 1
	value := h.items[last]
	h.items[last] = nil
	h.items = h.items[:last]
	return value
}

// tagSelector retains only the highest-ranked matching tags.
type tagSelector struct {
	filters      filtering.Filters
	limit        int
	nextSequence uint64
	groups       map[string]*tagGroupSelector
	orderBy      string
	useSemver    bool
}

// tagGroupSelector retains the best tags for one group.
type tagGroupSelector struct {
	selected map[string][]*tagCandidate
	heap     tagCandidateHeap
}

// newTagSelector creates a bounded selector for the given filters.
func newTagSelector(filters filtering.Filters) (*tagSelector, error) {
	limit, err := gitProviderResultLimit(filters)
	if err != nil {
		return nil, err
	}
	orderBy := filters.OrderBy
	if orderBy == "" {
		if filters.SemVer != nil {
			orderBy = filtering.OrderBySemVer
		} else {
			orderBy = filtering.OrderByReverseAlphabetical
		}
	}
	selector := &tagSelector{
		filters:   filters,
		limit:     limit,
		groups:    make(map[string]*tagGroupSelector, 1),
		orderBy:   orderBy,
		useSemver: orderBy == filtering.OrderBySemVer,
	}
	return selector, nil
}

// Add considers one tag for the bounded result set.
func (s *tagSelector) Add(name string, result Result) {
	if !s.filters.MatchString(name) {
		return
	}

	key, groupKey, ok := s.filters.TagKeys(name)
	if !ok {
		return
	}

	candidate := &tagCandidate{
		name:     name,
		key:      key,
		groupKey: groupKey,
		result:   result,
		sequence: s.nextSequence,
	}
	s.nextSequence++
	if s.useSemver {
		parsed, err := fluxversion.ParseVersion(key)
		if err != nil {
			return
		}
		if s.filters.SemVer != nil && !s.filters.SemVer.Check(parsed) {
			return
		}
		candidate.version = parsed
	}

	group := s.groupSelector(groupKey)
	for _, selected := range group.selected[name] {
		// The previous implementation resolved duplicate names through a map,
		// so every occurrence used the last response object's metadata.
		selected.result = result
	}

	if group.heap.Len() < s.limit {
		s.push(group, candidate)
		return
	}
	if compareTagCandidates(candidate, group.heap.items[0], s.orderBy) <= 0 {
		return
	}

	discarded := heap.Pop(&group.heap).(*tagCandidate)
	s.removeSelected(group, discarded)
	s.push(group, candidate)
}

// push adds a candidate to the heap and duplicate-name index.
func (s *tagSelector) push(group *tagGroupSelector, candidate *tagCandidate) {
	heap.Push(&group.heap, candidate)
	group.selected[candidate.name] = append(group.selected[candidate.name], candidate)
}

// removeSelected removes a candidate from the duplicate-name index.
func (s *tagSelector) removeSelected(group *tagGroupSelector, candidate *tagCandidate) {
	selected := group.selected[candidate.name]
	for i, item := range selected {
		if item == candidate {
			selected = append(selected[:i], selected[i+1:]...)
			break
		}
	}
	if len(selected) == 0 {
		delete(group.selected, candidate.name)
	} else {
		group.selected[candidate.name] = selected
	}
}

// Results returns selected tags in semver or lexical order.
func (s *tagSelector) Results() []Result {
	groupKeys := make([]string, 0, len(s.groups))
	for groupKey := range s.groups {
		groupKeys = append(groupKeys, groupKey)
	}
	sort.Strings(groupKeys)

	results := make([]Result, 0, len(groupKeys)*s.limit)
	for _, groupKey := range groupKeys {
		group := s.groups[groupKey]
		selected := append([]*tagCandidate(nil), group.heap.items...)
		sort.Slice(selected, func(i, j int) bool {
			return compareTagCandidates(selected[i], selected[j], s.orderBy) > 0
		})
		for _, candidate := range selected {
			results = append(results, candidate.result)
		}
	}
	return results
}

func (s *tagSelector) groupSelector(groupKey string) *tagGroupSelector {
	group, ok := s.groups[groupKey]
	if ok {
		return group
	}

	group = &tagGroupSelector{
		selected: make(map[string][]*tagCandidate, s.limit),
		heap: tagCandidateHeap{
			orderBy: s.orderBy,
		},
	}
	heap.Init(&group.heap)
	s.groups[groupKey] = group
	return group
}

// compareTagCandidates returns a negative value when a ranks below b, a
// positive value when a ranks above b, and zero when they are equivalent.
func compareTagCandidates(a, b *tagCandidate, orderBy string) int {
	switch orderBy {
	case filtering.OrderBySemVer:
		switch {
		case a.version.LessThan(b.version):
			return -1
		case a.version.GreaterThan(b.version):
			return 1
		}
	case filtering.OrderByAlphabetical:
		if keyOrder := strings.Compare(b.key, a.key); keyOrder != 0 {
			return keyOrder
		}
	case filtering.OrderByReverseAlphabetical:
		if keyOrder := strings.Compare(a.key, b.key); keyOrder != 0 {
			return keyOrder
		}
	case filtering.OrderByNumerical:
		if keyOrder := filtering.CompareNumericKeys(b.key, a.key); keyOrder != 0 {
			return keyOrder
		}
	case filtering.OrderByReverseNumerical:
		if keyOrder := filtering.CompareNumericKeys(a.key, b.key); keyOrder != 0 {
			return keyOrder
		}
	default:
		if keyOrder := strings.Compare(a.key, b.key); keyOrder != 0 {
			return keyOrder
		}
	}
	switch {
	case a.sequence < b.sequence:
		return 1
	case a.sequence > b.sequence:
		return -1
	default:
		return 0
	}
}

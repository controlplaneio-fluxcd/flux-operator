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
	result   Result
	version  *semver.Version
	sequence uint64
}

// tagCandidateHeap keeps the lowest-ranked retained tag at its root.
type tagCandidateHeap struct {
	items  []*tagCandidate
	semver bool
}

// Len returns the number of retained tag candidates.
func (h tagCandidateHeap) Len() int { return len(h.items) }

// Less reports whether the first candidate ranks below the second.
func (h tagCandidateHeap) Less(i, j int) bool {
	return compareTagCandidates(h.items[i], h.items[j], h.semver) < 0
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
	selected     map[string][]*tagCandidate
	heap         tagCandidateHeap
}

// newTagSelector creates a bounded selector for the given filters.
func newTagSelector(filters filtering.Filters) (*tagSelector, error) {
	limit, err := gitProviderResultLimit(filters)
	if err != nil {
		return nil, err
	}
	selector := &tagSelector{
		filters:  filters,
		limit:    limit,
		selected: make(map[string][]*tagCandidate, limit),
		heap:     tagCandidateHeap{semver: filters.SemVer != nil},
	}
	heap.Init(&selector.heap)
	return selector, nil
}

// Add considers one tag for the bounded result set.
func (s *tagSelector) Add(name string, result Result) {
	if !s.filters.MatchString(name) {
		return
	}

	for _, selected := range s.selected[name] {
		// The previous implementation resolved duplicate names through a map,
		// so every occurrence used the last response object's metadata.
		selected.result = result
	}

	candidate := &tagCandidate{
		name:     name,
		result:   result,
		sequence: s.nextSequence,
	}
	s.nextSequence++
	if s.filters.SemVer != nil {
		parsed, err := fluxversion.ParseVersion(name)
		if err != nil || !s.filters.SemVer.Check(parsed) {
			return
		}
		candidate.version = parsed
	}

	if s.heap.Len() < s.limit {
		s.push(candidate)
		return
	}
	if compareTagCandidates(candidate, s.heap.items[0], s.heap.semver) <= 0 {
		return
	}

	discarded := heap.Pop(&s.heap).(*tagCandidate)
	s.removeSelected(discarded)
	s.push(candidate)
}

// push adds a candidate to the heap and duplicate-name index.
func (s *tagSelector) push(candidate *tagCandidate) {
	heap.Push(&s.heap, candidate)
	s.selected[candidate.name] = append(s.selected[candidate.name], candidate)
}

// removeSelected removes a candidate from the duplicate-name index.
func (s *tagSelector) removeSelected(candidate *tagCandidate) {
	selected := s.selected[candidate.name]
	for i, item := range selected {
		if item == candidate {
			selected = append(selected[:i], selected[i+1:]...)
			break
		}
	}
	if len(selected) == 0 {
		delete(s.selected, candidate.name)
	} else {
		s.selected[candidate.name] = selected
	}
}

// Results returns selected tags in descending semver or lexical order.
func (s *tagSelector) Results() []Result {
	selected := append([]*tagCandidate(nil), s.heap.items...)
	sort.Slice(selected, func(i, j int) bool {
		return compareTagCandidates(selected[i], selected[j], s.heap.semver) > 0
	})
	results := make([]Result, len(selected))
	for i, candidate := range selected {
		results[i] = candidate.result
	}
	return results
}

// compareTagCandidates returns a negative value when a ranks below b, a
// positive value when a ranks above b, and zero when they are equivalent.
func compareTagCandidates(a, b *tagCandidate, useSemver bool) int {
	if useSemver {
		switch {
		case a.version.LessThan(b.version):
			return -1
		case a.version.GreaterThan(b.version):
			return 1
		}
	} else if nameOrder := strings.Compare(a.name, b.name); nameOrder != 0 {
		return nameOrder
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

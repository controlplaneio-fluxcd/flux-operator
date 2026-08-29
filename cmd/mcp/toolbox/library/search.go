// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	prefixWeight = 0.375
	fuzzyWeight  = 0.45
	maxFuzzy     = 6
)

// SearchOptions configures result filtering and limiting.
type SearchOptions struct {
	Limit  int
	Filter func(*Chunk) bool
}

// Hit is a scored documentation chunk and its owning page.
type Hit struct {
	Chunk *Chunk
	Doc   *Doc
	Score float64
}

type weightedTerm struct {
	term   string
	weight float64
}

type scoredChunk struct {
	score   float64
	matched map[string]struct{}
}

// Search returns documentation chunks ranked with MiniSearch-compatible field scoring.
func (l *Library) Search(query string, opts SearchOptions) []Hit {
	if l == nil || l.index == nil {
		return nil
	}
	queryGroups := TokenizeGroups(query)
	if len(queryGroups) == 0 {
		return nil
	}

	allowed := make(map[int]bool, len(l.chunks))
	for _, chunk := range l.chunks {
		allowed[chunk.ID] = opts.Filter == nil || opts.Filter(chunk)
	}

	termResults := make([]map[int]*scoredChunk, 0, len(queryGroups))
	for _, group := range queryGroups {
		results := make(map[int]*scoredChunk)
		for _, queryTerm := range group {
			variantScores := make(map[int]float64)
			for _, expansion := range l.index.expand(queryTerm) {
				for fieldID := range l.index.fields {
					field := &l.index.fields[fieldID]
					postings := field.postings[expansion.term]
					matchingCount := len(postings)
					for _, posting := range postings {
						if !allowed[posting.chunkID] {
							continue
						}
						variantScores[posting.chunkID] += expansion.weight * field.boost * calcBM25Score(
							posting.frequency,
							matchingCount,
							l.index.totalChunks,
							field.fieldLengths[posting.chunkID],
							field.averageFieldLength,
						)
					}
				}
			}
			for chunkID, score := range variantScores {
				result := results[chunkID]
				if result == nil {
					result = &scoredChunk{matched: map[string]struct{}{group[0]: {}}}
					results[chunkID] = result
				}
				result.score = max(result.score, score)
			}
		}
		termResults = append(termResults, results)
	}

	andResults := intersectResults(termResults)
	andHits := l.rank(andResults)
	if len(andHits) >= 3 {
		return limitHits(andHits, opts.Limit)
	}

	orResults := unionResults(termResults)
	orHits := l.rank(orResults)
	seen := make(map[int]struct{}, len(andHits))
	for _, hit := range andHits {
		seen[hit.Chunk.ID] = struct{}{}
	}
	combined := append([]Hit(nil), andHits...)
	for _, hit := range orHits {
		if _, found := seen[hit.Chunk.ID]; found {
			continue
		}
		combined = append(combined, hit)
	}
	return limitHits(combined, opts.Limit)
}

func (index *invertedIndex) expand(queryTerm string) []weightedTerm {
	expansions := make([]weightedTerm, 0, 16)
	matched := make(map[string]struct{})
	position := sort.SearchStrings(index.vocabulary, queryTerm)
	if position < len(index.vocabulary) && index.vocabulary[position] == queryTerm {
		expansions = append(expansions, weightedTerm{term: queryTerm, weight: 1})
		matched[queryTerm] = struct{}{}
	}

	queryLength := utf8.RuneCountInString(queryTerm)
	for ; position < len(index.vocabulary); position++ {
		term := index.vocabulary[position]
		if !strings.HasPrefix(term, queryTerm) {
			break
		}
		if term == queryTerm {
			continue
		}
		termLength := utf8.RuneCountInString(term)
		distance := termLength - queryLength
		weight := prefixWeight * float64(termLength) / (float64(termLength) + 0.3*float64(distance))
		expansions = append(expansions, weightedTerm{term: term, weight: weight})
		matched[term] = struct{}{}
	}

	fuzzyDistance := min(maxFuzzy, int(math.Round(0.15*float64(queryLength))))
	if fuzzyDistance == 0 {
		return expansions
	}
	for _, term := range index.vocabulary {
		if _, found := matched[term]; found {
			continue
		}
		distance, within := bandedLevenshtein(queryTerm, term, fuzzyDistance)
		if !within || distance == 0 {
			continue
		}
		termLength := utf8.RuneCountInString(term)
		weight := fuzzyWeight * float64(termLength) / float64(termLength+distance)
		expansions = append(expansions, weightedTerm{term: term, weight: weight})
	}
	return expansions
}

func intersectResults(results []map[int]*scoredChunk) map[int]*scoredChunk {
	if len(results) == 0 {
		return nil
	}
	combined := cloneResults(results[0])
	for _, next := range results[1:] {
		for chunkID, current := range combined {
			match := next[chunkID]
			if match == nil {
				delete(combined, chunkID)
				continue
			}
			current.score += match.score
			mergeMatches(current.matched, match.matched)
		}
	}
	return combined
}

func unionResults(results []map[int]*scoredChunk) map[int]*scoredChunk {
	combined := make(map[int]*scoredChunk)
	for _, result := range results {
		for chunkID, match := range result {
			current := combined[chunkID]
			if current == nil {
				combined[chunkID] = &scoredChunk{score: match.score, matched: cloneMatches(match.matched)}
				continue
			}
			current.score += match.score
			mergeMatches(current.matched, match.matched)
		}
	}
	return combined
}

func cloneResults(source map[int]*scoredChunk) map[int]*scoredChunk {
	clone := make(map[int]*scoredChunk, len(source))
	for chunkID, result := range source {
		clone[chunkID] = &scoredChunk{score: result.score, matched: cloneMatches(result.matched)}
	}
	return clone
}

func cloneMatches(source map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(source))
	mergeMatches(clone, source)
	return clone
}

func mergeMatches(target, source map[string]struct{}) {
	for term := range source {
		target[term] = struct{}{}
	}
}

func (l *Library) rank(results map[int]*scoredChunk) []Hit {
	hits := make([]Hit, 0, len(results))
	for chunkID, result := range results {
		chunk := l.chunksByID[chunkID]
		if chunk == nil {
			continue
		}
		hits = append(hits, Hit{
			Chunk: chunk,
			Doc:   l.docsByPath[chunk.DocPath],
			Score: result.score * float64(len(result.matched)),
		})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Chunk.ID < hits[j].Chunk.ID
	})
	return hits
}

func limitHits(hits []Hit, limit int) []Hit {
	if limit > 0 && len(hits) > limit {
		return hits[:limit]
	}
	return hits
}

func bandedLevenshtein(left, right string, maxDistance int) (int, bool) {
	a, b := []rune(left), []rune(right)
	if abs(len(a)-len(b)) > maxDistance {
		return maxDistance + 1, false
	}
	infinity := maxDistance + 1
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		if j <= maxDistance {
			previous[j] = j
		} else {
			previous[j] = infinity
		}
	}
	for i := 1; i <= len(a); i++ {
		for j := range current {
			current[j] = infinity
		}
		if i <= maxDistance {
			current[0] = i
		}
		start := max(1, i-maxDistance)
		end := min(len(b), i+maxDistance)
		for j := start; j <= end; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(b)], previous[len(b)] <= maxDistance
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

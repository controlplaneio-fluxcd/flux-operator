// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import "sort"

const (
	titleFieldBoost        = 2.5
	headingTrailFieldBoost = 3.0
	textFieldBoost         = 1.0
	descriptionFieldBoost  = 0.5
)

type posting struct {
	chunkID   int
	frequency int
}

type fieldIndex struct {
	name               string
	boost              float64
	postings           map[string][]posting
	fieldLengths       map[int]int
	averageFieldLength float64
}

type invertedIndex struct {
	fields      []fieldIndex
	vocabulary  []string
	totalChunks int
}

func buildInvertedIndex(library *Library) *invertedIndex {
	index := &invertedIndex{
		fields: []fieldIndex{
			{name: "title", boost: titleFieldBoost, postings: make(map[string][]posting), fieldLengths: make(map[int]int)},
			{name: "headingTrail", boost: headingTrailFieldBoost, postings: make(map[string][]posting), fieldLengths: make(map[int]int)},
			{name: "text", boost: textFieldBoost, postings: make(map[string][]posting), fieldLengths: make(map[int]int)},
			{name: "description", boost: descriptionFieldBoost, postings: make(map[string][]posting), fieldLengths: make(map[int]int)},
		},
		totalChunks: len(library.chunks),
	}
	vocabulary := make(map[string]struct{})

	for _, chunk := range library.chunks {
		doc := library.docsByPath[chunk.DocPath]
		values := []string{doc.Title, chunk.HeadingTrail, chunk.Text, doc.Description}
		for fieldID, value := range values {
			field := &index.fields[fieldID]
			tokens := Tokenize(value)
			frequencies := make(map[string]int, len(tokens))
			for _, token := range tokens {
				frequencies[token]++
				vocabulary[token] = struct{}{}
			}
			// MiniSearch computes field length from unique raw tokenizer output
			// before processTerm normalization and expansion.
			rawTokens := splitTokens(value)
			uniqueRawTokens := make(map[string]struct{}, len(rawTokens))
			for _, token := range rawTokens {
				uniqueRawTokens[token] = struct{}{}
			}
			field.fieldLengths[chunk.ID] = len(uniqueRawTokens)
			field.averageFieldLength += float64(len(uniqueRawTokens))
			for term, frequency := range frequencies {
				field.postings[term] = append(field.postings[term], posting{chunkID: chunk.ID, frequency: frequency})
			}
		}
	}

	if index.totalChunks > 0 {
		for fieldID := range index.fields {
			index.fields[fieldID].averageFieldLength /= float64(index.totalChunks)
		}
	}
	index.vocabulary = make([]string, 0, len(vocabulary))
	for term := range vocabulary {
		index.vocabulary = append(index.vocabulary, term)
	}
	sort.Strings(index.vocabulary)
	return index
}

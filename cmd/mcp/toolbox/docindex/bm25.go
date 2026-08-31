// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import "math"

const (
	bm25K = 1.2
	bm25B = 0.7
	bm25D = 0.5
)

func calcBM25Score(termFrequency, matchingCount, totalCount, fieldLength int, averageFieldLength float64) float64 {
	if termFrequency <= 0 || matchingCount <= 0 || totalCount <= 0 || averageFieldLength <= 0 {
		return 0
	}
	idf := math.Log(1 + (float64(totalCount-matchingCount)+0.5)/(float64(matchingCount)+0.5))
	tf := float64(termFrequency)
	normalization := tf + bm25K*(1-bm25B+bm25B*float64(fieldLength)/averageFieldLength)
	return idf * (bm25D + tf*(bm25K+1)/normalization)
}

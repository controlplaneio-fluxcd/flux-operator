// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"fmt"
	"hash/adler32"
)

// Result holds the information about a pull/merge request.
type Result struct {
	ID     string `json:"id"`
	SHA    string `json:"sha"`
	Author string `json:"author"`
	Title  string `json:"title"`
	Branch string `json:"sourceBranch"`
}

func (r *Result) ToMap() map[string]any {
	return map[string]any{
		"id":     r.ID,
		"sha":    r.SHA,
		"author": r.Author,
		"title":  r.Title,
		"branch": r.Branch,
	}
}

func (r *Result) ToMapWithDefault(defaults map[string]any) map[string]any {
	m := r.ToMap()
	for k, v := range defaults {
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return m
}

func checksum(txt string) string {
	return fmt.Sprintf("%v", adler32.Checksum([]byte(txt)))
}

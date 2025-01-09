// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

// Result holds the information about a pull/merge request.
type Result struct {
	ID           string `json:"id"`
	SHA          string `json:"sha"`
	Author       string `json:"author"`
	Title        string `json:"title"`
	SourceBranch string `json:"sourceBranch"`
	TargetBranch string `json:"targetBranch"`
}

func (r *Result) ToMap() map[string]any {
	return map[string]any{
		"id":           r.ID,
		"sha":          r.SHA,
		"author":       r.Author,
		"title":        r.Title,
		"sourceBranch": r.SourceBranch,
		"targetBranch": r.TargetBranch,
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

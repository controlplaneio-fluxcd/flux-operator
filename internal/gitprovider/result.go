// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"fmt"
	"hash/adler32"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

// Result holds the information extracted from the Git SaaS provider response.
type Result struct {
	ID     string `json:"id"`
	SHA    string `json:"sha"`
	Branch string `json:"branch"`
	Author string `json:"author,omitempty"`
	Title  string `json:"title,omitempty"`
}

// ToMap converts the result into a map.
func (r *Result) ToMap() map[string]any {
	m := map[string]any{
		"id":     r.ID,
		"sha":    r.SHA,
		"branch": r.Branch,
	}

	if r.Author != "" {
		m["author"] = r.Author
	}

	if r.Title != "" {
		m["title"] = r.Title
	}

	return m
}

// ToMapWithDefaults converts the result into a map with default values.
func (r *Result) ToMapWithDefaults(defaults map[string]any) map[string]any {
	m := r.ToMap()
	for k, v := range defaults {
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return m
}

// MakeInputs converts a list of results into a list of ResourceSet inputs with defaults.
func MakeInputs(results []Result, defaults map[string]any) ([]map[string]*apiextensionsv1.JSON, error) {
	var inputs []map[string]*apiextensionsv1.JSON

	list := make([]map[string]any, 0, len(results))
	for _, r := range results {
		list = append(list, r.ToMapWithDefaults(defaults))
	}

	for _, item := range list {
		input := make(map[string]*apiextensionsv1.JSON)
		for k, v := range item {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal value %v: %w", v, err)
			}
			input[k] = &apiextensionsv1.JSON{Raw: b}
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}

func checksum(txt string) string {
	return fmt.Sprintf("%v", adler32.Checksum([]byte(txt)))
}

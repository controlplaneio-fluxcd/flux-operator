// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"k8s.io/apimachinery/pkg/util/json"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Result holds the information extracted from the Git SaaS provider response.
type Result struct {
	ID     string   `json:"id"`
	SHA    string   `json:"sha"`
	Branch string   `json:"branch,omitempty"`
	Tag    string   `json:"tag,omitempty"`
	Author string   `json:"author,omitempty"`
	Title  string   `json:"title,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// ToMap converts the result into a map.
func (r *Result) ToMap() map[string]any {
	m := map[string]any{
		"id":  r.ID,
		"sha": r.SHA,
	}

	if r.Branch != "" {
		m["branch"] = r.Branch
	}

	if r.Tag != "" {
		m["tag"] = r.Tag
	}

	if r.Author != "" {
		m["author"] = r.Author
	}

	if r.Title != "" {
		m["title"] = r.Title
	}

	if len(r.Labels) > 0 {
		m["labels"] = r.Labels
	}

	return m
}

// OverrideFromExportedInputs override result fields from exportedInput.
func (r *Result) OverrideFromExportedInputs(input map[string]any) error {
	var err error

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, r)
	if err != nil {
		return err
	}

	return nil
}

// MakeInputs converts a list of results into a list of ResourceSet inputs with defaults.
func MakeInputs(results []Result, defaults map[string]any) ([]fluxcdv1.ResourceSetInput, error) {
	inputs := make([]fluxcdv1.ResourceSetInput, 0, len(results))
	for _, item := range results {
		input, err := fluxcdv1.NewResourceSetInput(defaults, item.ToMap())
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

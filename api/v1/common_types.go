// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

// CommonMetadata defines the common labels and annotations.
type CommonMetadata struct {
	// Annotations to be added to the object's metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels to be added to the object's metadata.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

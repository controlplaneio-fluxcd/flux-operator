// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// Result holds the build result.
type Result struct {
	Version         string
	Digest          string
	Revision        string
	Objects         []*unstructured.Unstructured
	ComponentImages []ComponentImage
}

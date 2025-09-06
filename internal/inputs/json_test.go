// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func mustJSON(t *testing.T, s string) *apiextensionsv1.JSON {
	id, err := inputs.JSON(s)
	if err != nil {
		t.Fatalf("failed to compute JSON: %v", err)
	}
	return id
}

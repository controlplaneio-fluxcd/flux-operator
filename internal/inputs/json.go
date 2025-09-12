// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

// JSON returns the Kubernetes API JSON representation of any value.
func JSON(v any) (*apiextensionsv1.JSON, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &apiextensionsv1.JSON{Raw: b}, nil
}

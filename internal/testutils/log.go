// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package testutils

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// LogObjectStatus logs the status fields of a FluxObject in YAML format.
func LogObjectStatus(t *testing.T, obj fluxcdv1.FluxObject) {
	u, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	status, _, _ := unstructured.NestedFieldCopy(u, "status")
	sts, _ := yaml.Marshal(status)
	t.Log(obj.GetName(), "status:\n", string(sts))
}

// LogObject logs the entire FluxObject in YAML format.
func LogObject(t *testing.T, obj fluxcdv1.FluxObject) {
	sts, _ := yaml.Marshal(obj)
	t.Log("object:\n", string(sts))
}

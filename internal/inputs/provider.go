// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ProviderKey is the key used to identify input providers.
type ProviderKey struct {
	GVK       schema.GroupVersionKind
	Name      string
	Namespace string
}

// NewProviderKey returns the key for the input provider.
func NewProviderKey(provider fluxcdv1.InputProvider) ProviderKey {
	return ProviderKey{
		GVK:       provider.GroupVersionKind(),
		Name:      provider.GetName(),
		Namespace: provider.GetNamespace(),
	}
}

// compareProviderKeys compares two ProviderKey objects.
func compareProviderKeys(a, b ProviderKey) int {
	if gvkA, gvkB := a.GVK.String(), b.GVK.String(); gvkA != gvkB {
		return strings.Compare(gvkA, gvkB)
	}
	if a.Namespace != b.Namespace {
		return strings.Compare(a.Namespace, b.Namespace)
	}
	return strings.Compare(a.Name, b.Name)
}

// getFromProvider returns the inputs from the given input provider
// and annotates each input with the provider reference.
func getFromProvider(provider fluxcdv1.InputProvider) ([]map[string]any, error) {
	providerInputs, err := provider.GetInputs()
	if err != nil {
		return nil, err
	}

	for _, input := range providerInputs {
		providerType := provider.GroupVersionKind()
		input["provider"] = map[string]any{
			"apiVersion": providerType.GroupVersion().String(),
			"kind":       providerType.Kind,
			"name":       provider.GetName(),
			"namespace":  provider.GetNamespace(),
		}
	}
	return providerInputs, nil
}

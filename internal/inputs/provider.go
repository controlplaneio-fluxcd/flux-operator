// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
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

// AddProviderReference adds the provider reference to the input.
func AddProviderReference(input map[string]any, provider fluxcdv1.InputProvider) {
	providerType := provider.GroupVersionKind()
	input["provider"] = map[string]any{
		"apiVersion": providerType.GroupVersion().String(),
		"kind":       providerType.Kind,
		"name":       provider.GetName(),
		"namespace":  provider.GetNamespace(),
	}
}

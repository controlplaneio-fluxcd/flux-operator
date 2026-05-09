// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package v1 contains API Schema definitions for the fluxcd v1 API group
// +kubebuilder:object:generate=true
// +groupName=fluxcd.controlplane.io
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "fluxcd.controlplane.io", Version: "v1"}

	// schemeBuilder accumulates the type registration functions for this API group.
	schemeBuilder runtime.SchemeBuilder

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &groupVersionBuilder{}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = schemeBuilder.AddToScheme
)

// groupVersionBuilder registers Go types with the package's GroupVersion.
// It replaces the deprecated sigs.k8s.io/controller-runtime/pkg/scheme.Builder
// so that this API package depends only on k8s.io/apimachinery.
type groupVersionBuilder struct{}

// Register schedules the given runtime.Object types to be added to the scheme
// under the package's GroupVersion when AddToScheme is called.
func (b *groupVersionBuilder) Register(objects ...runtime.Object) {
	schemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, objects...)
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})
}

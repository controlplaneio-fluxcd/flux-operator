// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
)

const (
	FluxInstanceKind = "FluxInstance"
	EnabledValue     = "enabled"
	DisabledValue    = "disabled"
)

var (
	Finalizer                = fmt.Sprintf("%s/finalizer", GroupVersion.Group)
	ReconcileAnnotation      = fmt.Sprintf("%s/reconcile", GroupVersion.Group)
	ReconcileEveryAnnotation = fmt.Sprintf("%s/reconcileEvery", GroupVersion.Group)
)

// FluxInstanceSpec defines the desired state of FluxInstance
type FluxInstanceSpec struct {
	// Kustomize holds a set of patches that can be applied to the
	// Flux installation, to customize the way Flux operates.
	// +optional
	Kustomize *Kustomize `json:"kustomize,omitempty"`
}

// Kustomize specification.
type Kustomize struct {
	// Strategic merge and JSON patches, defined as inline YAML objects,
	// capable of targeting objects based on kind, label and annotation selectors.
	// +optional
	Patches []kustomize.Patch `json:"patches,omitempty"`
}

// FluxInstanceStatus defines the observed state of FluxInstance
type FluxInstanceStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in *FluxInstance) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *FluxInstance) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetInterval returns the interval at which the object should be reconciled.
// If no interval is set, the default is 60 minutes.
func (in *FluxInstance) GetInterval() time.Duration {
	val, ok := in.GetAnnotations()[ReconcileAnnotation]
	if ok && strings.ToLower(val) == DisabledValue {
		return 0
	}
	val, ok = in.GetAnnotations()[ReconcileEveryAnnotation]
	if !ok {
		return 60 * time.Minute
	}
	interval, err := time.ParseDuration(val)
	if err != nil {
		return 60 * time.Minute
	}
	return interval
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FluxInstance is the Schema for the fluxinstances API
type FluxInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluxInstanceSpec   `json:"spec,omitempty"`
	Status FluxInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FluxInstanceList contains a list of FluxInstance
type FluxInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluxInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluxInstance{}, &FluxInstanceList{})
}

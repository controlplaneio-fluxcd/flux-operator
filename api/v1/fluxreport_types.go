// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	FluxReportKind = "FluxReport"
)

// FluxReportSpec defines the observed state of a Flux installation.
type FluxReportSpec struct {
	// Distribution is the version information of the Flux installation.
	// +required
	Distribution FluxDistributionStatus `json:"distribution"`

	// Cluster is the version information of the Kubernetes cluster.
	// +optional
	Cluster *ClusterInfo `json:"cluster,omitempty"`

	// ComponentsStatus is the status of the Flux controller deployments.
	// +optional
	ComponentsStatus []FluxComponentStatus `json:"components,omitempty"`

	// ReconcilersStatus is the list of Flux reconcilers and
	// their statistics grouped by API kind.
	// +optional
	ReconcilersStatus []FluxReconcilerStatus `json:"reconcilers,omitempty"`

	// SyncStatus is the status of the cluster sync
	// Source and Kustomization resources.
	// +optional
	SyncStatus *FluxSyncStatus `json:"sync,omitempty"`
}

// FluxDistributionStatus defines the version information of the Flux instance.
type FluxDistributionStatus struct {
	// Entitlement is the entitlement verification status.
	// +required
	Entitlement string `json:"entitlement"`

	// Status is a human-readable message indicating details
	// about the distribution observed state.
	// +required
	Status string `json:"status"`

	// Version is the version of the Flux instance.
	// +optional
	Version string `json:"version,omitempty"`

	// ManagedBy is the name of the operator managing the Flux instance.
	// +optional
	ManagedBy string `json:"managedBy,omitempty"`
}

// ClusterInfo defines the version information of the Kubernetes cluster.
type ClusterInfo struct {
	// ServerVersion is the version of the Kubernetes API server.
	ServerVersion string `json:"serverVersion,omitempty"`

	// Platform is the os/arch of the Kubernetes control plane.
	Platform string `json:"platform,omitempty"`

	// Nodes is the number of nodes in the Kubernetes cluster.
	Nodes int `json:"nodes,omitempty"`
}

// FluxComponentStatus defines the observed state of a Flux component.
type FluxComponentStatus struct {
	// Name is the name of the Flux component.
	// +required
	Name string `json:"name"`

	// Ready is the readiness status of the Flux component.
	// +required
	Ready bool `json:"ready"`

	// Status is a human-readable message indicating details
	// about the Flux component observed state.
	// +required
	Status string `json:"status"`

	// Image is the container image of the Flux component.
	// +required
	Image string `json:"image"`
}

// FluxReconcilerStatus defines the observed state of a Flux reconciler.
type FluxReconcilerStatus struct {
	// APIVersion is the API version of the Flux resource.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the Flux resource.
	// +required
	Kind string `json:"kind"`

	// Stats is the reconcile statics of the Flux resource kind.
	// +optional
	Stats FluxReconcilerStats `json:"stats,omitempty"`
}

// FluxReconcilerStats defines the reconcile statistics.
type FluxReconcilerStats struct {
	// Running is the number of reconciled
	// resources in the Running state.
	// +required
	Running int `json:"running"`

	// Failing is the number of reconciled
	// resources in the Failing state.
	// +required
	Failing int `json:"failing"`

	// Suspended is the number of reconciled
	// resources in the Suspended state.
	// +required
	Suspended int `json:"suspended"`

	// TotalSize is the total size of the artifacts in storage.
	// +optional
	TotalSize string `json:"totalSize,omitempty"`
}

// FluxSyncStatus defines the observed state of the cluster sync.
type FluxSyncStatus struct {
	// ID is the identifier of the sync.
	// +required
	ID string `json:"id"`

	// Path is the kustomize path of the sync.
	// +optional
	Path string `json:"path,omitempty"`

	// Ready is the readiness status of the sync.
	// +required
	Ready bool `json:"ready"`

	// Status is a human-readable message indicating details
	// about the sync observed state.
	// +required
	Status string `json:"status"`

	// Source is the URL of the source repository.
	// +optional
	Source string `json:"source,omitempty"`
}

// FluxReportStatus defines the readiness of a FluxReport.
type FluxReportStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// Conditions contains the readiness conditions of the object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Entitlement",type="string",JSONPath=".spec.distribution.entitlement",description="",priority=10
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="LastUpdated",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].lastTransitionTime",description=""

// FluxReport is the Schema for the fluxreports API.
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'flux'", message="the only accepted name for a FluxReport is 'flux'"
type FluxReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluxReportSpec   `json:"spec,omitempty"`
	Status FluxReportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FluxReportList contains a list of FluxReport.
type FluxReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluxReport `json:"items"`
}

// GetConditions returns the status conditions of the object.
func (in *FluxReport) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *FluxReport) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// IsDisabled returns true if the object has the reconcile annotation set to 'disabled'.
func (in *FluxReport) IsDisabled() bool {
	val, ok := in.GetAnnotations()[ReconcileAnnotation]
	return ok && strings.ToLower(val) == DisabledValue
}

// GetInterval returns the interval at which the object should be reconciled.
// If the annotation is not set, it returns 0.
func (in *FluxReport) GetInterval() time.Duration {
	val, ok := in.GetAnnotations()[ReconcileEveryAnnotation]
	if !ok {
		return 0
	}
	interval, err := time.ParseDuration(val)
	if err == nil {
		return interval
	}
	return 0
}

func init() {
	SchemeBuilder.Register(&FluxReport{}, &FluxReportList{})
}

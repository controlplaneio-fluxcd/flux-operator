// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

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
	Finalizer                  = fmt.Sprintf("%s/finalizer", GroupVersion.Group)
	ReconcileAnnotation        = fmt.Sprintf("%s/reconcile", GroupVersion.Group)
	ReconcileEveryAnnotation   = fmt.Sprintf("%s/reconcileEvery", GroupVersion.Group)
	ReconcileTimeoutAnnotation = fmt.Sprintf("%s/reconcileTimeout", GroupVersion.Group)
	PruneAnnotation            = fmt.Sprintf("%s/prune", GroupVersion.Group)
	RevisionAnnotation         = fmt.Sprintf("%s/revision", GroupVersion.Group)
)

// FluxInstanceSpec defines the desired state of FluxInstance
type FluxInstanceSpec struct {
	// Distribution specifies the version and container registry to pull images from.
	// +required
	Distribution Distribution `json:"distribution"`

	// Components is the list of controllers to install.
	// Defaults to all controllers.
	// +optional
	Components []Component `json:"components,omitempty"`

	// Cluster holds the specification of the Kubernetes cluster.
	// +optional
	Cluster *Cluster `json:"cluster,omitempty"`

	// Kustomize holds a set of patches that can be applied to the
	// Flux installation, to customize the way Flux operates.
	// +optional
	Kustomize *Kustomize `json:"kustomize,omitempty"`

	// Wait instructs the controller to check the health of all the reconciled
	// resources. Defaults to true.
	// +kubebuilder:default:=true
	// +required
	Wait bool `json:"wait"`
}

// Distribution specifies the version and container registry to pull images from.
type Distribution struct {
	// Version semver expression e.g. '2.x', '2.3.x'.
	// +required
	Version string `json:"version"`

	// Registry address to pull the distribution images from
	// e.g. 'ghcr.io/fluxcd'.
	// +required
	Registry string `json:"registry"`

	// ImagePullSecret is the name of the Kubernetes secret
	// to use for pulling images.
	// +optional
	ImagePullSecret string `json:"imagePullSecret,omitempty"`
}

// Component is the name of a controller to install.
// +kubebuilder:validation:Enum:=source-controller;kustomize-controller;helm-controller;notification-controller;image-reflector-controller;image-automation-controller
type Component string

// Cluster is the specification for the Kubernetes cluster.
type Cluster struct {
	// Domain is the cluster domain used for generating the FQDN of services.
	// Defaults to 'cluster.local'.
	// +kubebuilder:default:=cluster.local
	// +required
	Domain string `json:"domain"`

	// Multitenant enables the multitenancy lockdown.
	// +optional
	Multitenant bool `json:"multitenant,omitempty"`

	// NetworkPolicy restricts network access to the current namespace.
	// Defaults to true.
	// +kubebuilder:default:=true
	// +required
	NetworkPolicy bool `json:"networkPolicy"`

	// Type specifies the distro of the Kubernetes cluster.
	// Defaults to 'kubernetes'.
	// +kubebuilder:validation:Enum:=kubernetes;openshift
	// +kubebuilder:default:=kubernetes
	// +optional
	Type string `json:"type,omitempty"`
}

// Kustomize holds a set of patches that can be applied to the
// Flux installation, to customize the way Flux operates.
type Kustomize struct {
	// Strategic merge and JSON patches, defined as inline YAML objects,
	// capable of targeting objects based on kind, label and annotation selectors.
	// +optional
	Patches []kustomize.Patch `json:"patches,omitempty"`
}

// ResourceInventory contains a list of Kubernetes resource object references
// that have been applied.
type ResourceInventory struct {
	// Entries of Kubernetes resource object references.
	Entries []ResourceRef `json:"entries"`
}

// ResourceRef contains the information necessary to locate a resource within a cluster.
type ResourceRef struct {
	// ID is the string representation of the Kubernetes resource object's metadata,
	// in the format '<namespace>_<name>_<group>_<kind>'.
	ID string `json:"id"`

	// Version is the API version of the Kubernetes resource object's kind.
	Version string `json:"v"`
}

// FluxInstanceStatus defines the observed state of FluxInstance
type FluxInstanceStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// Conditions contains the readiness conditions of the object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastAttemptedRevision is the version and digest of the
	// distribution config that was last attempted to reconcile.
	// +optional
	LastAttemptedRevision string `json:"lastAttemptedRevision,omitempty"`

	// LastAppliedRevision is the version and digest of the
	// distribution config that was last reconcile.
	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty"`

	// Inventory contains a list of Kubernetes resource object references
	// last applied on the cluster.
	// +optional
	Inventory *ResourceInventory `json:"inventory,omitempty"`
}

// GetDistribution returns the distribution specification with defaults.
func (in *FluxInstance) GetDistribution() Distribution {
	return in.Spec.Distribution
}

// GetComponents returns the components to install with defaults.
func (in *FluxInstance) GetComponents() []string {
	components := make([]string, len(in.Spec.Components))
	for i, c := range in.Spec.Components {
		components[i] = string(c)
	}
	if len(components) == 0 {
		components = []string{
			"source-controller",
			"kustomize-controller",
			"helm-controller",
			"notification-controller",
		}
	}

	return components
}

// GetCluster returns the cluster specification with defaults.
func (in *FluxInstance) GetCluster() Cluster {
	cluster := in.Spec.Cluster
	if cluster == nil {
		cluster = &Cluster{}
	}
	if cluster.Domain == "" {
		cluster.Domain = "cluster.local"
	}
	if cluster.NetworkPolicy {
		cluster.NetworkPolicy = true
	}
	if cluster.Type == "" {
		cluster.Type = "kubernetes"
	}
	return *cluster
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
	defaultInterval := 60 * time.Minute
	val, ok = in.GetAnnotations()[ReconcileEveryAnnotation]
	if !ok {
		return defaultInterval
	}
	interval, err := time.ParseDuration(val)
	if err != nil {
		return defaultInterval
	}
	return interval
}

// GetTimeout returns the timeout for the reconciliation process.
// If no timeout is set, the default is 5 minutes.
func (in *FluxInstance) GetTimeout() time.Duration {
	defaultTimeout := 5 * time.Minute
	val, ok := in.GetAnnotations()[ReconcileTimeoutAnnotation]
	if !ok {
		return defaultTimeout
	}
	timeout, err := time.ParseDuration(val)
	if err != nil {
		return defaultTimeout
	}
	return timeout
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

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

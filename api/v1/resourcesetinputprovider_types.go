// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/fluxcd/pkg/apis/meta"
)

const (
	ResourceSetInputProviderKind    = "ResourceSetInputProvider"
	InputProviderGitHubBranch       = "GitHubBranch"
	InputProviderGitHubPullRequest  = "GitHubPullRequest"
	InputProviderGitLabBranch       = "GitLabBranch"
	InputProviderGitLabMergeRequest = "GitLabMergeRequest"
)

// ResourceSetInputProviderSpec defines the desired state of ResourceSetInputProvider
type ResourceSetInputProviderSpec struct {
	// Type specifies the type of the input provider.
	// +kubebuilder:validation:Enum=GitHubBranch;GitHubPullRequest;GitLabBranch;GitLabMergeRequest
	// +required
	Type string `json:"type"`

	// URL specifies the HTTP/S address of the input provider API.
	// When connecting to a Git provider, the URL should point to the repository address.
	// +kubebuilder:validation:Pattern="^(http|https)://.*$"
	// +required
	URL string `json:"url"`

	// SecretRef specifies the Kubernetes Secret containing the basic-auth credentials
	// to access the input provider. The secret must contain the keys
	// 'username' and 'password'.
	// When connecting to a Git provider, the password should be a personal access token
	// that grants read-only access to the repository.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// CertSecretRef specifies the Kubernetes Secret containing either or both of
	//
	// - a PEM-encoded CA certificate (`ca.crt`)
	// - a PEM-encoded client certificate (`tls.crt`) and private key (`tls.key`)
	//
	// When connecting to a Git provider that uses self-signed certificates, the CA certificate
	// must be set in the Secret under the 'ca.crt' key to establish the trust relationship.
	// +optional
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`

	// DefaultValues contains the default values for the inputs.
	// These values are used to populate the inputs when the provider
	// response does not contain them.
	// +optional
	DefaultValues ResourceSetInput `json:"defaultValues,omitempty"`

	// Filter defines the filter to apply to the input provider response.
	// +optional
	Filter *ResourceSetInputFilter `json:"filter,omitempty"`
}

// ResourceSetInputFilter defines the filter to apply to the input provider response.
type ResourceSetInputFilter struct {
	// IncludeBranch specifies the regular expression to filter the branches
	// that the input provider should include.
	// +optional
	IncludeBranch string `json:"includeBranch,omitempty"`

	// ExcludeBranch specifies the regular expression to filter the branches
	// that the input provider should exclude.
	// +optional
	ExcludeBranch string `json:"excludeBranch,omitempty"`

	// Labels specifies the list of labels to filter the input provider response.
	// +optional
	Labels []string `json:"labels,omitempty"`

	// Limit specifies the maximum number of input sets to return.
	// When not set, the default limit is 100.
	// +optional
	Limit int `json:"limit,omitempty"`
}

// ResourceSetInputProviderStatus defines the observed state of ResourceSetInputProvider.
type ResourceSetInputProviderStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`

	// Conditions contains the readiness conditions of the object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ExportedInputs contains the list of inputs exported by the provider.
	// +optional
	ExportedInputs []ResourceSetInput `json:"exportedInputs,omitempty"`

	// LastExportedRevision is the digest of the
	// inputs that were last reconcile.
	// +optional
	LastExportedRevision string `json:"lastExportedRevision,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in *ResourceSetInputProvider) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *ResourceSetInputProvider) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// IsDisabled returns true if the object has the reconcile annotation set to 'disabled'.
func (in *ResourceSetInputProvider) IsDisabled() bool {
	val, ok := in.GetAnnotations()[ReconcileAnnotation]
	return ok && strings.ToLower(val) == DisabledValue
}

// GetInterval returns the interval at which the object should be reconciled.
// If no interval is set, the default is 60 minutes.
func (in *ResourceSetInputProvider) GetInterval() time.Duration {
	val, ok := in.GetAnnotations()[ReconcileAnnotation]
	if ok && strings.ToLower(val) == DisabledValue {
		return 0
	}
	defaultInterval := 10 * time.Minute
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
func (in *ResourceSetInputProvider) GetTimeout() time.Duration {
	defaultTimeout := 1 * time.Minute
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

// GetDefaultInputs returns the ResourceSetInputProvider default inputs.
func (in *ResourceSetInputProvider) GetDefaultInputs() (map[string]any, error) {
	defaults := make(map[string]any)
	for k, v := range in.Spec.DefaultValues {
		var data any
		if err := json.Unmarshal(v.Raw, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal default values[%s]: %w", k, err)
		}
		defaults[k] = data
	}
	return defaults, nil
}

// GetInputs returns the exported inputs from ResourceSetInputProvider status.
func (in *ResourceSetInputProvider) GetInputs() ([]map[string]any, error) {
	inputs := make([]map[string]any, 0, len(in.Status.ExportedInputs))
	for i, ji := range in.Status.ExportedInputs {
		inp := make(map[string]any, len(ji))
		for k, v := range ji {
			var data any
			if err := json.Unmarshal(v.Raw, &data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal inputs[%d]: %w", i, err)
			}
			inp[k] = data
		}
		inputs = append(inputs, inp)
	}
	return inputs, nil
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rsip
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// ResourceSetInputProvider is the Schema for the ResourceSetInputProviders API.
type ResourceSetInputProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSetInputProviderSpec   `json:"spec,omitempty"`
	Status ResourceSetInputProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceSetInputProviderList contains a list of ResourceSetInputProvider.
type ResourceSetInputProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceSetInputProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceSetInputProvider{}, &ResourceSetInputProviderList{})
}

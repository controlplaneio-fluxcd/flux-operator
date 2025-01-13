// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import "fmt"

const (
	EnabledValue  = "enabled"
	DisabledValue = "disabled"

	ReconciliationDisabledReason  = "ReconciliationDisabled"
	ReconciliationDisabledMessage = "Reconciliation is disabled"
)

var (
	Finalizer                        = fmt.Sprintf("%s/finalizer", GroupVersion.Group)
	ReconcileAnnotation              = fmt.Sprintf("%s/reconcile", GroupVersion.Group)
	ReconcileEveryAnnotation         = fmt.Sprintf("%s/reconcileEvery", GroupVersion.Group)
	ReconcileArtifactEveryAnnotation = fmt.Sprintf("%s/reconcileArtifactEvery", GroupVersion.Group)
	ReconcileTimeoutAnnotation       = fmt.Sprintf("%s/reconcileTimeout", GroupVersion.Group)
	PruneAnnotation                  = fmt.Sprintf("%s/prune", GroupVersion.Group)
	RevisionAnnotation               = fmt.Sprintf("%s/revision", GroupVersion.Group)
)

// CommonMetadata defines the common labels and annotations.
type CommonMetadata struct {
	// Annotations to be added to the object's metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels to be added to the object's metadata.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

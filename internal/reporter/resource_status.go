// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Resource status constants.
const (
	StatusReady       = "Ready"
	StatusFailed      = "Failed"
	StatusProgressing = "Progressing"
	StatusSuspended   = "Suspended"
	StatusUnknown     = "Unknown"
)

// ResourceStatus represents the reconciliation status of a Flux resource.
type ResourceStatus struct {
	// Name of the resource.
	Name string `json:"name"`

	// Kind of the resource.
	Kind string `json:"kind"`

	// Namespace of the resource.
	Namespace string `json:"namespace"`

	// Status can be "Ready", "Failed", "Progressing", "Suspended", "Unknown"
	Status string `json:"status"`

	// Message is a brief reason for the current status.
	Message string `json:"message"`

	// LastReconciled is the timestamp of the last reconciliation.
	LastReconciled metav1.Time `json:"lastReconciled"`
}

// NewResourceStatus extracts the ResourceStatus from an unstructured Kubernetes object.
// Maps Kubernetes condition status to one of: "Ready", "Failed", "Progressing", "Suspended", "Unknown"
// nolint: gocyclo
func NewResourceStatus(obj unstructured.Unstructured) ResourceStatus {
	status := StatusUnknown
	message := "No status information available"
	lastReconciled := metav1.Time{Time: obj.GetCreationTimestamp().Time}

	// Check for status conditions (Ready condition)
	if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && err == nil {
		for _, cond := range conditions {
			if condition, ok := cond.(map[string]any); ok && condition["type"] == meta.ReadyCondition {
				// Get condition status (True/False/Unknown)
				if condStatus, ok := condition["status"].(string); ok {
					switch condStatus {
					case "True":
						status = StatusReady
					case "False":
						if reason, exists := condition["reason"]; exists {
							if reasonStr, _ := reason.(string); reasonStr == meta.DependencyNotReadyReason {
								status = StatusProgressing
								break
							}
						}
						status = StatusFailed
					case "Unknown":
						// Check reason to determine if it's progressing or truly unknown
						if reason, exists := condition["reason"]; exists {
							reasonStr, _ := reason.(string)
							if reasonStr == StatusProgressing || reasonStr == "Reconciling" {
								status = StatusProgressing
							} else {
								status = StatusUnknown
							}
						} else {
							status = StatusProgressing
						}
					default:
						// Any other status value defaults to Unknown
						status = StatusUnknown
					}
				}

				// Extract message
				if msg, exists := condition["message"]; exists {
					if msgStr, ok := msg.(string); ok && msgStr != "" {
						message = msgStr
					}
				}

				// Extract last transition time
				if lastTransitionTime, exists := condition["lastTransitionTime"]; exists {
					if timeStr, ok := lastTransitionTime.(string); ok {
						if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
							lastReconciled = metav1.Time{Time: parsedTime}
						}
					}
				}

				break // Found Ready condition, no need to check others
			}
		}
	}

	// If kind is Alert or Provider set status to Ready as they don't have conditions
	if (obj.GetKind() == fluxcdv1.FluxAlertKind ||
		obj.GetKind() == fluxcdv1.FluxAlertProviderKind) &&
		status == StatusUnknown {
		status = StatusReady
		message = "Valid configuration"
	}

	// if kind is HelmRepository and has .spec.type of 'oci', set status to Ready
	if obj.GetKind() == fluxcdv1.FluxHelmRepositoryKind {
		if specType, found, err := unstructured.NestedString(obj.Object, "spec", "type"); found && err == nil {
			if specType == "oci" && status == StatusUnknown {
				status = StatusReady
				message = "Valid configuration"
			}
		}
	}

	// Check for suspended state (takes precedence over condition status)
	// Check reconcile annotation
	if ssautil.AnyInMetadata(&obj,
		map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
		status = StatusSuspended
		message = "Reconciliation suspended"
	}

	// Check spec.suspend field
	if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend && found && err == nil {
		status = StatusSuspended
		message = "Reconciliation suspended"
	}

	return ResourceStatus{
		Kind:           obj.GetKind(),
		Name:           obj.GetName(),
		Namespace:      obj.GetNamespace(),
		LastReconciled: lastReconciled,
		Status:         status,
		Message:        message,
	}
}

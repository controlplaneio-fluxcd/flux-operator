// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Schedule defines a schedule for the input provider to run.
type Schedule struct {
	// Cron specifies the cron expression for the schedule.
	// +required
	Cron string `json:"cron"`

	// TimeZone specifies the time zone for the cron schedule. Defaults to UTC.
	// +kubebuilder:default:="UTC"
	// +optional
	TimeZone string `json:"timeZone"`

	// Window defines the time window during which the input provider is allowed to run.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +kubebuilder:default:="0s"
	// +optional
	Window metav1.Duration `json:"window"`
}

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	ReasonInvalidSchedule      = "InvalidSchedule"
	ReasonSkippedDueToSchedule = "SkippedDueToSchedule"
)

// Schedule defines a schedule for something to run.
type Schedule struct {
	// Cron specifies the cron expression for the schedule.
	// +required
	Cron string `json:"cron"`

	// TimeZone specifies the time zone for the cron schedule. Defaults to UTC.
	// +kubebuilder:default:="UTC"
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// Window defines the time window during which the execution is allowed.
	// Defaults to 0s, meaning no window is applied.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +kubebuilder:default:="0s"
	// +optional
	Window metav1.Duration `json:"window"`
}

// NextSchedule defines the next time a schedule will run.
type NextSchedule struct {
	// Schedule has the configuration of the next schedule.
	Schedule `json:",inline"`

	// When is the next time the schedule will run.
	// +required
	When metav1.Time `json:"when"`
}

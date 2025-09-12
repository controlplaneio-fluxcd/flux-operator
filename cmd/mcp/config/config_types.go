// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConfigKind is the kind of the Flux MCP configuration API.
	ConfigKind = "Config"

	// TransportHTTP is the http transport mode.
	TransportHTTP = "http"
	// TransportSTDIO is the stdio transport mode.
	TransportSTDIO = "stdio"
	// TransportSSE is the legacy sse transport mode. Not supported in the Config API.
	TransportSSE = "sse"
)

// Config is the Flux MCP configuration.
type Config struct {
	metav1.TypeMeta `json:",inline"`

	// Spec holds the Flux MCP configuration.
	Spec ConfigSpec `json:"spec"`
}

// ConfigSpec holds the Flux MCP configuration.
type ConfigSpec struct {
	// Transport is the MCP transport. One of: http, stdio.
	// +kubebuilder:validation:Enum=http;stdio
	// +required
	Transport string `json:"transport"`

	// ReadOnly indicates if the MCP server should operate in read-only mode.
	// +optional
	ReadOnly bool `json:"readonly,omitempty"`

	// Authentication holds the authentication configuration.
	// +optional
	Authentication *AuthenticationSpec `json:"authentication,omitempty"`
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// WebConfigGroupVersion is group and version of the Flux Status Page configuration API.
	WebConfigGroupVersion = schema.GroupVersion{Group: "web.fluxcd.controlplane.io", Version: "v1"}
)

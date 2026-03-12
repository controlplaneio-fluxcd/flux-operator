// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// AgentGroupVersion is the group and version of the Agent Catalog API.
	AgentGroupVersion = schema.GroupVersion{Group: "agent.fluxcd.controlplane.io", Version: "v1"}
)

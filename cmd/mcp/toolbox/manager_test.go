// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestManager_RegisterToolsDoesNotPanic(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "flux-operator-mcp",
		Version: "test-version",
	}, &mcp.ServerOptions{
		HasTools: true,
	})

	manager := NewManager(nil, 0, false, false)
	manager.RegisterTools(server, false)
}

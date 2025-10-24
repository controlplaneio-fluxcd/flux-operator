// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SystemPrompt struct {
	Prompt  *mcp.Prompt
	Handler mcp.PromptHandler
}

// Manager represents an entity responsible for managing and
// registering prompts and their handlers in a server.
type Manager struct{}

// NewManager creates and returns a new instance of Manager
// for managing and registering prompts and their handlers.
func NewManager() *Manager {
	return &Manager{}
}

// RegisterPrompts registers all prompts in the Manager's PromptSet with the provided server.
func (m *Manager) RegisterPrompts(server *mcp.Server) {
	for _, p := range m.PromptSet() {
		server.AddPrompt(p.Prompt, p.Handler)
	}
}

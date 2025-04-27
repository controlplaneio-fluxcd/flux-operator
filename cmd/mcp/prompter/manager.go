// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"fmt"
	"io"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type SystemPrompt struct {
	Prompt  mcp.Prompt
	Handler mcpserver.PromptHandlerFunc
}

type DocResource struct {
	Resource mcp.Resource
	Handler  mcpserver.ResourceHandlerFunc
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
func (m *Manager) RegisterPrompts(server *mcpserver.MCPServer) {
	for _, p := range m.PromptSet() {
		server.AddPrompt(p.Prompt, p.Handler)
	}
}

func (m *Manager) RegisterResources(server *mcpserver.MCPServer) {
	for _, doc := range m.DocResourceSet() {
		server.AddResource(doc.Resource, doc.Handler)
	}
}

// fetchMarkdown retrieves the content of the document
// from the specified URL and returns it as a string.
func (m *Manager) fetchMarkdown(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}

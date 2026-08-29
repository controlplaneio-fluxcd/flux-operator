// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewToolResultText creates a new CallToolResult with a text content.
func NewToolResultText(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// NewToolResultTexts creates a new CallToolResult with one text content per string.
func NewToolResultTexts(texts []string) (*mcp.CallToolResult, any, error) {
	content := make([]mcp.Content, len(texts))
	for i, text := range texts {
		content[i] = &mcp.TextContent{Text: text}
	}
	return &mcp.CallToolResult{Content: content}, nil, nil
}

// NewToolResultError creates a new CallToolResult with an error message.
// Any errors that originate from the tool SHOULD be reported inside the result object.
func NewToolResultError(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}, nil, nil
}

// NewToolResultErrorFromErr creates a new CallToolResult with an error message.
// If an error is provided, its details will be appended to the text message.
// Any errors that originate from the tool SHOULD be reported inside the result object.
func NewToolResultErrorFromErr(text string, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		text = fmt.Sprintf("%s: %v", text, err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
		IsError: true,
	}, nil, nil
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewMiddleware creates a new authentication middleware for the MCP server.
func NewMiddleware(transport Transport, authenticator Authenticator, validateScopes bool) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			creds, ok := transport.Extract(ctx, request.Header)
			if !ok {
				return nil, errors.New("failed to extract credentials from request")
			}
			sess, err := authenticator.Authenticate(ctx, *creds)
			if err != nil {
				return nil, fmt.Errorf("failed to authenticate request: %w", err)
			}
			if !validateScopes {
				sess.Scopes = nil
			} else if sess.Scopes == nil {
				sess.Scopes = []string{}
			}
			ctx = context.WithValue(ctx, sessionContextKey{}, sess)
			return next(ctx, request)
		}
	}
}

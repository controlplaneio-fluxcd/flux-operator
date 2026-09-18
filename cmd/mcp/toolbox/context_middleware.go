// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// contextInputName is the tool input that selects the kubeconfig context of a call.
	contextInputName = "context"
)

// ContextMiddleware returns middleware that runs a tool call against the
// kubeconfig context named by its context input, by placing a Kubernetes
// client for that context in the request context where GetClient finds it.
// Calls without the input, and calls to tools that do not talk to a cluster,
// pass through unchanged.
func (m *Manager) ContextMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			callReq, ok := req.(*mcp.CallToolRequest)
			if !ok || method != "tools/call" || systemTools[callReq.Params.Name].local {
				return next(ctx, method, req)
			}

			var input struct {
				Context string `json:"context"`
			}
			if len(callReq.Params.Arguments) > 0 {
				// A malformed input is left to the tool's own validation.
				if err := json.Unmarshal(callReq.Params.Arguments, &input); err != nil {
					return next(ctx, method, req)
				}
			}
			if input.Context == "" {
				return next(ctx, method, req)
			}

			kubeClient, err := m.kubeClient.GetClientForContext(input.Context)
			if err != nil {
				result, _, _ := NewToolResultErrorFromErr("Failed to get Kubernetes client for context "+input.Context, err)
				return result, nil
			}
			return next(kubeClient.IntoContext(ctx), method, req)
		}
	}
}

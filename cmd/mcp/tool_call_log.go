// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolCallLogEntry is the JSON document appended for each MCP tool call.
type toolCallLogEntry struct {
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionID,omitempty"`
	Tool      string          `json:"tool"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    mcp.Result      `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// toolCallLogMiddleware returns middleware that logs MCP tool calls after
// the tool handler returns.
func toolCallLogMiddleware(path string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, req)

			callReq, ok := req.(*mcp.CallToolRequest)
			if !ok || method != "tools/call" {
				return result, err
			}

			entry := toolCallLogEntry{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Tool:      callReq.Params.Name,
				Input:     callReq.Params.Arguments,
				Output:    result,
			}
			if session := req.GetSession(); session != nil {
				entry.SessionID = session.ID()
			}
			if err != nil {
				entry.Error = err.Error()
			}
			if logErr := appendToolCallLog(path, entry); logErr != nil {
				fmt.Fprintf(os.Stderr, "failed to append MCP tool call log: %v\n", logErr)
			}

			return result, err
		}
	}
}

// appendToolCallLog appends one JSON document to path using a fresh file
// descriptor for each call.
func appendToolCallLog(path string, entry toolCallLogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}
	data = append(data, '\n')

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write log file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

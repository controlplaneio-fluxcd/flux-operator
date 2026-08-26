// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox"
)

func newTestServer() *mcp.Server {
	tm := toolbox.NewManager(k8s.NewClientFactory(kubeconfigArgs), time.Minute, true, true, nil, false)
	mcpServer := mcp.NewServer(mcpImpl, &mcp.ServerOptions{
		Instructions: tm.Instructions(true),
		Capabilities: &mcp.ServerCapabilities{
			Tools: &mcp.ToolCapabilities{},
		},
	})
	tm.RegisterTools(mcpServer, true)
	return mcpServer
}

func TestHTTPHandler_Stateless(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpServer := httptest.NewServer(newHTTPHandler(newTestServer()))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
	}, nil)
	g.Expect(err).ToNot(HaveOccurred())
	defer session.Close()

	// The server must negotiate the latest protocol version.
	g.Expect(session.InitializeResult().ProtocolVersion).To(Equal("2026-07-28"))
	g.Expect(session.InitializeResult().ServerInfo.Name).To(Equal(mcpImpl.Name))

	// The server must advertise instructions naming the available tools.
	g.Expect(session.InitializeResult().Instructions).To(ContainSubstring(toolbox.ToolGetFluxInstance))
	g.Expect(session.InitializeResult().Instructions).ToNot(ContainSubstring(toolbox.ToolSetKubeConfigContext))

	// Stateless servers don't issue session IDs.
	g.Expect(session.ID()).To(BeEmpty())

	// Tools must be discoverable and callable.
	tools, err := session.ListTools(ctx, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(tools.Tools).ToNot(BeEmpty())

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolbox.ToolSearchFluxDocs,
		Arguments: map[string]any{"query": "ResourceSet"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.IsError).To(BeFalse())
	g.Expect(result.Content).ToNot(BeEmpty())

	// Session deletion is not supported in stateless mode.
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, httpServer.URL+"/mcp", nil)
	g.Expect(err).ToNot(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	g.Expect(err).ToNot(HaveOccurred())
	_ = resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
}

func TestHTTPHandler_LegacyClient(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpServer := httptest.NewServer(newHTTPHandler(newTestServer()))
	defer httpServer.Close()

	// Clients on older protocol versions use the initialize handshake
	// and must be negotiated down to the version they requested.
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0.0.0"}}}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+"/mcp", strings.NewReader(body))
	g.Expect(err).ToNot(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	g.Expect(err).ToNot(HaveOccurred())
	defer resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
	g.Expect(resp.Header.Get("Mcp-Session-Id")).To(BeEmpty())

	data, err := io.ReadAll(resp.Body)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(data)).To(ContainSubstring(`"protocolVersion":"2025-11-25"`))
	g.Expect(string(data)).To(ContainSubstring(`"name":"flux-operator-mcp"`))
}

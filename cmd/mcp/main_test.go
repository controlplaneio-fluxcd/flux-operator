// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/skills"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/docindex"
)

func newTestServer() (*mcp.Server, []string) {
	if err := docindex.Load(); err != nil {
		panic(err)
	}
	catalog, err := skills.LoadEmbedded()
	if err != nil {
		panic(err)
	}
	tm := toolbox.NewManager(k8s.NewClientFactory(kubeconfigArgs), time.Minute, true, true, false)
	mcpServer, registered, err := newMCPServer(tm, catalog, true)
	if err != nil {
		panic(err)
	}
	return mcpServer, registered
}

func TestHTTPHandler_Stateless(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mcpServer, registeredTools := newTestServer()
	searchIndex := slices.Index(registeredTools, toolbox.ToolSearchFluxDocs)
	g.Expect(searchIndex).To(BeNumerically(">=", 0))
	g.Expect(registeredTools[searchIndex+1]).To(Equal(toolbox.ToolReadFluxDoc))

	httpServer := httptest.NewServer(newHTTPHandler(mcpServer))
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
	toolNames := make([]string, len(tools.Tools))
	for i, tool := range tools.Tools {
		toolNames[i] = tool.Name
	}
	g.Expect(toolNames).To(ContainElements(toolbox.ToolSearchFluxDocs, toolbox.ToolReadFluxDoc))

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

	mcpServer, _ := newTestServer()
	httpServer := httptest.NewServer(newHTTPHandler(mcpServer))
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

func TestHTTPHandler_Skills(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mcpServer, _ := newTestServer()
	httpServer := httptest.NewServer(newHTTPHandler(mcpServer))
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.0"}, nil)
	g.Expect(mcp.AddSendingCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](client, skills.MethodList)).To(Succeed())
	g.Expect(mcp.AddSendingCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](client, skills.MethodGet)).To(Succeed())
	g.Expect(mcp.AddSendingCustomMethod[*skills.ReadDirectoryParams, *skills.ReadDirectoryResult](client, skills.MethodReadDirectory)).To(Succeed())
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
	}, nil)
	g.Expect(err).ToNot(HaveOccurred())
	defer session.Close()

	// The server must declare the Skills extension and the resources capability.
	caps := session.InitializeResult().Capabilities
	g.Expect(caps.Resources).ToNot(BeNil())
	g.Expect(caps.Extensions).To(HaveKeyWithValue(skills.ExtensionID, map[string]any{"directoryRead": true}))

	list, err := mcp.CallCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](ctx, session,
		skills.MethodList, &skills.ListSkillsParams{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(list.ResultType).To(Equal("complete"))
	g.Expect(list.Skills).ToNot(BeEmpty())

	// The instructions point to every served skill.
	for _, skill := range list.Skills {
		g.Expect(session.InitializeResult().Instructions).To(ContainSubstring(skill.URI))
	}

	skill := list.Skills[0]
	got, err := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](ctx, session,
		skills.MethodGet, &skills.GetSkillParams{URI: skill.URI})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got.Skill).To(Equal(skill))

	dir, err := mcp.CallCustomMethod[*skills.ReadDirectoryParams, *skills.ReadDirectoryResult](ctx, session,
		skills.MethodReadDirectory, &skills.ReadDirectoryParams{URI: strings.TrimSuffix(skill.URI, "/SKILL.md")})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(dir.Resources).ToNot(BeEmpty())

	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: skill.URI})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Contents).To(HaveLen(1))
	g.Expect(int64(len(res.Contents[0].Text))).To(Equal(skill.Resources[0].Size))

	_, err = mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](ctx, session,
		skills.MethodGet, &skills.GetSkillParams{URI: "skill://unknown/SKILL.md"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no skill found at skill://unknown/SKILL.md"))
}

func TestHTTPHandler_SkillsLegacyClient(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mcpServer, _ := newTestServer()
	httpServer := httptest.NewServer(newHTTPHandler(mcpServer))
	defer httpServer.Close()

	post := func(body string) string {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+"/mcp", strings.NewReader(body))
		g.Expect(err).ToNot(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
		resp, err := http.DefaultClient.Do(req)
		g.Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
		data, err := io.ReadAll(resp.Body)
		g.Expect(err).ToNot(HaveOccurred())
		return string(data)
	}

	// Clients on older protocol versions see the extension in the initialize result.
	data := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"0.0.0"}}}`)
	g.Expect(data).To(ContainSubstring(`"io.modelcontextprotocol/skills":{"directoryRead":true}`))

	// The skills methods are served without request metadata.
	data = post(`{"jsonrpc":"2.0","id":2,"method":"skills/list","params":{}}`)
	g.Expect(data).To(ContainSubstring(`"resultType":"complete"`))
	g.Expect(data).To(ContainSubstring(`"uri":"skill://`))

	data = post(`{"jsonrpc":"2.0","id":3,"method":"skills/get","params":{"uri":"skill://unknown/SKILL.md"}}`)
	g.Expect(data).To(ContainSubstring(`"code":-32602`))
}

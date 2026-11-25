// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package skills

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
)

// connect serves the catalog on an in-memory transport
// and returns a client session able to call the skills methods.
func connect(t *testing.T, catalog *Catalog) *mcp.ClientSession {
	t.Helper()
	g := NewWithT(t)
	ctx := context.Background()

	caps := &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}}
	AddCapabilities(caps)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, &mcp.ServerOptions{
		Capabilities: caps,
	})
	g.Expect(catalog.Register(server)).To(Succeed())

	client := newTestClient()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() { _ = serverSession.Close() })

	session, err := client.Connect(ctx, clientTransport, nil)
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// newTestClient returns an MCP client that can send the skills methods.
func newTestClient() *mcp.Client {
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	if err := mcp.AddSendingCustomMethod[*ListSkillsParams, *ListSkillsResult](client, MethodList); err != nil {
		panic(err)
	}
	if err := mcp.AddSendingCustomMethod[*GetSkillParams, *GetSkillResult](client, MethodGet); err != nil {
		panic(err)
	}
	if err := mcp.AddSendingCustomMethod[*ReadDirectoryParams, *ReadDirectoryResult](client, MethodReadDirectory); err != nil {
		panic(err)
	}
	return client
}

func listAll(ctx context.Context, g Gomega, session *mcp.ClientSession) ([]*Skill, int) {
	var all []*Skill
	var cursor string
	pages := 0
	for {
		res, err := mcp.CallCustomMethod[*ListSkillsParams, *ListSkillsResult](ctx, session, MethodList,
			&ListSkillsParams{Cursor: cursor})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res.ResultType).To(Equal("complete"))
		g.Expect(res.TTLMs).To(Equal(ttlMs))
		g.Expect(res.CacheScope).To(Equal("public"))
		all = append(all, res.Skills...)
		pages++
		if res.NextCursor == "" {
			return all, pages
		}
		cursor = res.NextCursor
	}
}

func expectInvalidParams(g Gomega, err error) {
	g.Expect(err).To(HaveOccurred())
	var rpcErr *jsonrpc.Error
	g.Expect(errors.As(err, &rpcErr)).To(BeTrue(), "expected a JSON-RPC error, got %v", err)
	g.Expect(rpcErr.Code).To(Equal(int64(jsonrpc.CodeInvalidParams)))
}

func TestServer_Capabilities(t *testing.T) {
	g := NewWithT(t)

	catalog, err := LoadEmbedded()
	g.Expect(err).ToNot(HaveOccurred())
	session := connect(t, catalog)

	caps := session.InitializeResult().Capabilities
	g.Expect(caps.Resources).ToNot(BeNil())
	g.Expect(caps.Tools).ToNot(BeNil())
	g.Expect(caps.Extensions).To(HaveKeyWithValue(ExtensionID, map[string]any{"directoryRead": true}))
}

func TestServer_EmbeddedSkills(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	catalog, err := LoadEmbedded()
	g.Expect(err).ToNot(HaveOccurred())
	session := connect(t, catalog)

	skills, _ := listAll(ctx, g, session)
	g.Expect(skills).ToNot(BeEmpty())
	g.Expect(skills).To(HaveLen(len(catalog.Skills())))

	// Every skill file is listed as a resource.
	var listed []string
	for res, err := range session.Resources(ctx, nil) {
		g.Expect(err).ToNot(HaveOccurred())
		listed = append(listed, res.URI)
	}

	// Every regular file of the embedded data is listed as a resource
	// and appears in the manifest of its skill.
	manifests := make(map[string][]string)
	for _, skill := range skills {
		for _, entry := range skill.Resources {
			manifests[skill.URI] = append(manifests[skill.URI], entry.URI)
		}
	}
	data, err := fs.Sub(embedded, "data")
	g.Expect(err).ToNot(HaveOccurred())
	files := 0
	g.Expect(fs.WalkDir(data, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files++
		uri := URIScheme + p
		skillName, _, _ := strings.Cut(p, "/")
		g.Expect(listed).To(ContainElement(uri))
		g.Expect(manifests[URIScheme+skillName+"/SKILL.md"]).To(ContainElement(uri))
		return nil
	})).To(Succeed())
	total := 0
	for _, m := range manifests {
		total += len(m)
	}
	g.Expect(total).To(Equal(files))

	for _, skill := range skills {
		got, err := mcp.CallCustomMethod[*GetSkillParams, *GetSkillResult](ctx, session, MethodGet,
			&GetSkillParams{URI: skill.URI})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(got.ResultType).To(Equal("complete"))
		g.Expect(got.TTLMs).To(Equal(ttlMs))
		g.Expect(got.CacheScope).To(Equal("public"))
		g.Expect(got.Skill).To(Equal(skill))

		// The manifest matches the bytes served by resources/read.
		for _, entry := range skill.Resources {
			g.Expect(listed).To(ContainElement(entry.URI))
			res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: entry.URI})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Contents).To(HaveLen(1))
			content := res.Contents[0]
			g.Expect(content.URI).To(Equal(entry.URI))
			data := []byte(content.Text)
			if content.Blob != nil {
				data = content.Blob
			}
			g.Expect(int64(len(data))).To(Equal(entry.Size), "size of %s", entry.URI)
			g.Expect(sha256Digest(data)).To(Equal(entry.Digest), "digest of %s", entry.URI)
		}

		// The skill root directory can be read.
		root := skill.URI[:len(skill.URI)-len("/SKILL.md")]
		dir, err := mcp.CallCustomMethod[*ReadDirectoryParams, *ReadDirectoryResult](ctx, session, MethodReadDirectory,
			&ReadDirectoryParams{URI: root})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(dir.ResultType).To(Equal("complete"))
		g.Expect(dir.Resources).To(ContainElement(HaveField("URI", skill.URI)))
	}
}

func TestServer_Fixture(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	catalog, err := Load(fixtureFS())
	g.Expect(err).ToNot(HaveOccurred())
	catalog.pageSize = 1
	session := connect(t, catalog)

	// Pagination returns every skill exactly once, one per page.
	skills, pages := listAll(ctx, g, session)
	g.Expect(pages).To(Equal(2))
	g.Expect(skills).To(HaveLen(2))
	g.Expect(skills[0].URI).To(Equal("skill://demo/SKILL.md"))
	g.Expect(skills[1].URI).To(Equal("skill://other/SKILL.md"))
	g.Expect(skills[0].Frontmatter).To(HaveKeyWithValue("license", "Apache-2.0"))
	g.Expect(skills[0].Frontmatter).To(HaveKey("metadata"))
	g.Expect(skills[0].Resources).To(HaveLen(7))

	// Directory reads list the direct children with pagination.
	var children []*mcp.Resource
	var cursor string
	for {
		res, err := mcp.CallCustomMethod[*ReadDirectoryParams, *ReadDirectoryResult](ctx, session, MethodReadDirectory,
			&ReadDirectoryParams{URI: "skill://demo/references", Cursor: cursor})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res.Resources).To(HaveLen(1))
		children = append(children, res.Resources...)
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	g.Expect(children).To(HaveLen(2))
	g.Expect(children[0].URI).To(Equal("skill://demo/references/guide.md"))
	g.Expect(children[0].Name).To(Equal("guide.md"))
	g.Expect(children[0].MIMEType).To(Equal("text/markdown"))
	g.Expect(children[1].URI).To(Equal("skill://demo/references/nested"))
	g.Expect(children[1].Name).To(Equal("nested"))
	g.Expect(children[1].MIMEType).To(Equal(DirectoryMIMEType))

	// Binary files are served as blobs matching the manifest.
	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "skill://demo/assets/logo.bin"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Contents[0].Blob).To(Equal([]byte{0xff, 0xfe, 0x00, 0x01}))
	g.Expect(res.Contents[0].MIMEType).To(Equal("application/octet-stream"))
	g.Expect(res.TTLMs).To(Equal(ttlMs))

	// Text files are served verbatim.
	res, err = session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "skill://demo/SKILL.md"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Contents[0].Text).To(Equal(fixtureSkillMD))
	g.Expect(res.Contents[0].MIMEType).To(Equal("text/markdown"))
}

func TestServer_Errors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	catalog, err := Load(fixtureFS())
	NewWithT(t).Expect(err).ToNot(HaveOccurred())
	session := connect(t, catalog)

	getSkill := func(uri string) error {
		_, err := mcp.CallCustomMethod[*GetSkillParams, *GetSkillResult](ctx, session, MethodGet,
			&GetSkillParams{URI: uri})
		return err
	}
	readDir := func(uri, cursor string) error {
		_, err := mcp.CallCustomMethod[*ReadDirectoryParams, *ReadDirectoryResult](ctx, session, MethodReadDirectory,
			&ReadDirectoryParams{URI: uri, Cursor: cursor})
		return err
	}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "get unknown skill", call: func() error { return getSkill("skill://unknown/SKILL.md") }},
		{name: "get skill root", call: func() error { return getSkill("skill://demo") }},
		{name: "get supporting file", call: func() error { return getSkill("skill://demo/references/guide.md") }},
		{name: "get empty uri", call: func() error { return getSkill("") }},
		{name: "read unknown directory", call: func() error { return readDir("skill://unknown", "") }},
		{name: "read directory with trailing slash", call: func() error { return readDir("skill://demo/", "") }},
		{name: "read file as directory", call: func() error { return readDir("skill://demo/SKILL.md", "") }},
		{name: "read directory with invalid cursor", call: func() error { return readDir("skill://demo", "!") }},
		{name: "read directory with cursor of another directory", call: func() error {
			return readDir("skill://demo", encodeCursor(MethodReadDirectory+" skill://demo/references", "skill://demo/references/guide.md"))
		}},
		{name: "read directory with cursor of skills/list", call: func() error {
			return readDir("skill://demo", encodeCursor(MethodList, "skill://demo/SKILL.md"))
		}},
		{name: "read directory with unscoped cursor", call: func() error {
			return readDir("skill://demo", base64.RawURLEncoding.EncodeToString([]byte("skill://demo/SKILL.md")))
		}},
		{name: "read directory with unknown cursor key", call: func() error {
			return readDir("skill://demo", encodeCursor(MethodReadDirectory+" skill://demo", "skill://demo/unknown.md"))
		}},
		{name: "list with unknown cursor key", call: func() error {
			_, err := mcp.CallCustomMethod[*ListSkillsParams, *ListSkillsResult](ctx, session, MethodList,
				&ListSkillsParams{Cursor: encodeCursor(MethodList, "skill://a/SKILL.md")})
			return err
		}},
		{name: "list with directory cursor", call: func() error {
			_, err := mcp.CallCustomMethod[*ListSkillsParams, *ListSkillsResult](ctx, session, MethodList,
				&ListSkillsParams{Cursor: encodeCursor(MethodReadDirectory+" skill://demo", "skill://demo/SKILL.md")})
			return err
		}},
		{name: "list with invalid cursor", call: func() error {
			_, err := mcp.CallCustomMethod[*ListSkillsParams, *ListSkillsResult](ctx, session, MethodList,
				&ListSkillsParams{Cursor: "%%%"})
			return err
		}},
		{name: "read unknown file", call: func() error {
			_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "skill://demo/unknown.md"})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectInvalidParams(NewWithT(t), tt.call())
		})
	}
}

func TestServer_WireFormat(t *testing.T) {
	g := NewWithT(t)
	otherSize := strconv.Itoa(len(fixtureFS()["other/SKILL.md"].Data))

	catalog, err := Load(fixtureFS())
	g.Expect(err).ToNot(HaveOccurred())

	res, err := catalog.getSkill(context.Background(), nil, &GetSkillParams{URI: "skill://other/SKILL.md"})
	g.Expect(err).ToNot(HaveOccurred())
	data, err := json.Marshal(res)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(data)).To(MatchJSON(`{
		"resultType": "complete",
		"ttlMs": 3600000,
		"cacheScope": "public",
		"skill": {
			"uri": "skill://other/SKILL.md",
			"frontmatter": {"name": "other", "description": "Other skill"},
			"resources": [{
				"uri": "skill://other/SKILL.md",
				"digest": "` + sha256Digest(fixtureFS()["other/SKILL.md"].Data) + `",
				"size": ` + otherSize + `
			}]
		}
	}`))

	list, err := catalog.listSkills(context.Background(), nil, nil)
	g.Expect(err).ToNot(HaveOccurred())
	data, err = json.Marshal(list)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(data)).ToNot(ContainSubstring("nextCursor"))

	dir, err := catalog.readDirectory(context.Background(), nil, &ReadDirectoryParams{URI: "skill://other"})
	g.Expect(err).ToNot(HaveOccurred())
	data, err = json.Marshal(dir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(data)).To(MatchJSON(`{
		"resultType": "complete",
		"ttlMs": 3600000,
		"cacheScope": "public",
		"resources": [{"uri": "skill://other/SKILL.md", "name": "other", "description": "Other skill", "mimeType": "text/markdown", "size": ` + otherSize + `}]
	}`))
}

func TestServer_EmptyFile(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fsys := fixtureFS()
	fsys["demo/empty.md"] = &fstest.MapFile{Data: []byte{}}
	catalog, err := Load(fsys)
	g.Expect(err).ToNot(HaveOccurred())
	session := connect(t, catalog)

	skill := catalog.skillsByURI["skill://demo/SKILL.md"]
	g.Expect(skill.Resources).To(ContainElement(&Resource{
		URI:    "skill://demo/empty.md",
		Digest: sha256Digest(nil),
		Size:   0,
	}))

	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "skill://demo/empty.md"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Contents).To(HaveLen(1))
	g.Expect(res.Contents[0].Text).To(BeEmpty())
	g.Expect(res.Contents[0].Blob).ToNot(BeNil())
	g.Expect(res.Contents[0].Blob).To(BeEmpty())

	result, err := catalog.readResource(ctx, &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: "skill://demo/empty.md"}})
	g.Expect(err).ToNot(HaveOccurred())
	wire, err := json.Marshal(result.Contents[0])
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(wire)).To(MatchJSON(`{"uri":"skill://demo/empty.md","mimeType":"text/markdown","blob":""}`))
}

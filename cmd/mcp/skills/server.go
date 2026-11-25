// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package skills

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddCapabilities declares the resources capability and the Skills extension
// in the capabilities passed to [mcp.NewServer].
func AddCapabilities(caps *mcp.ServerCapabilities) {
	if caps.Resources == nil {
		caps.Resources = &mcp.ResourceCapabilities{}
	}
	caps.AddExtension(ExtensionID, map[string]any{"directoryRead": true})
}

// Register adds the skill files as resources and the skills/list, skills/get
// and resources/directory/read handlers to the server.
//
// TODO: replace AddCapabilities, Register and the protocol types with the
// go-sdk skills package once it ships upstream, serving the embedded data with
// skills.AddFS(server, fs.Sub(embedded, "data"), opts). Tracked in
// https://github.com/modelcontextprotocol/go-sdk/pull/1238 and
// https://github.com/modelcontextprotocol/go-sdk/pull/1240.
func (c *Catalog) Register(server *mcp.Server) error {
	for _, f := range c.files {
		server.AddResource(f.resource, c.readResource)
	}
	if err := mcp.AddReceivingCustomMethod(server, MethodList, c.listSkills); err != nil {
		return err
	}
	if err := mcp.AddReceivingCustomMethod(server, MethodGet, c.getSkill); err != nil {
		return err
	}
	return mcp.AddReceivingCustomMethod(server, MethodReadDirectory, c.readDirectory)
}

// readResource serves a skill file as text if it is valid UTF-8, or as a blob.
func (c *Catalog) readResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	f, ok := c.filesByURI[uri]
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	contents := &mcp.ResourceContents{
		URI:      uri,
		MIMEType: f.resource.MIMEType,
	}
	switch {
	case len(f.content) == 0:
		// The SDK omits an empty text, while an empty blob is sent as "".
		contents.Blob = []byte{}
	case utf8.Valid(f.content):
		contents.Text = string(f.content)
	default:
		contents.Blob = f.content
	}
	return &mcp.ReadResourceResult{
		Cacheable: cacheable(),
		Contents:  []*mcp.ResourceContents{contents},
	}, nil
}

// listSkills handles skills/list returning a page of skill entries.
func (c *Catalog) listSkills(_ context.Context, _ *mcp.ServerSession, params *ListSkillsParams) (*ListSkillsResult, error) {
	var cursor string
	if params != nil {
		cursor = params.Cursor
	}
	page, next, err := paginate(c.skills, MethodList, cursor, c.pageSize, func(s *Skill) string { return s.URI })
	if err != nil {
		return nil, err
	}
	return &ListSkillsResult{
		Cacheable:  cacheable(),
		ResultType: resultTypeComplete,
		NextCursor: next,
		Skills:     page,
	}, nil
}

// getSkill handles skills/get returning the entry of a single skill.
func (c *Catalog) getSkill(_ context.Context, _ *mcp.ServerSession, params *GetSkillParams) (*GetSkillResult, error) {
	if params == nil || params.URI == "" {
		return nil, invalidParams("missing required uri")
	}
	skill, ok := c.skillsByURI[params.URI]
	if !ok {
		return nil, invalidParams("no skill found at " + params.URI)
	}
	return &GetSkillResult{
		Cacheable:  cacheable(),
		ResultType: resultTypeComplete,
		Skill:      skill,
	}, nil
}

// readDirectory handles resources/directory/read returning
// a page of the direct children of a skill directory.
func (c *Catalog) readDirectory(_ context.Context, _ *mcp.ServerSession, params *ReadDirectoryParams) (*ReadDirectoryResult, error) {
	if params == nil || params.URI == "" {
		return nil, invalidParams("missing required uri")
	}
	children, ok := c.directories[params.URI]
	if !ok {
		if _, isFile := c.filesByURI[params.URI]; isFile {
			return nil, invalidParams(params.URI + " is not a directory resource")
		}
		return nil, invalidParams("no directory found at " + params.URI)
	}
	page, next, err := paginate(children, MethodReadDirectory+" "+params.URI, params.Cursor, c.pageSize, func(r *mcp.Resource) string { return r.URI })
	if err != nil {
		return nil, err
	}
	return &ReadDirectoryResult{
		Cacheable:  cacheable(),
		ResultType: resultTypeComplete,
		NextCursor: next,
		Resources:  page,
	}, nil
}

// paginate returns the page of items, sorted by key, that follows the cursor,
// and the cursor of the next page. A cursor is valid only for its scope.
func paginate[T any](items []T, scope, cursor string, pageSize int, key func(T) string) ([]T, string, error) {
	start := 0
	if cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, "", invalidParams("invalid cursor")
		}
		cursorScope, last, ok := strings.Cut(string(decoded), "\x00")
		if !ok || cursorScope != scope || last == "" {
			return nil, "", invalidParams("invalid cursor")
		}
		i, found := slices.BinarySearchFunc(items, last, func(item T, target string) int {
			return strings.Compare(key(item), target)
		})
		if !found {
			return nil, "", invalidParams("invalid cursor")
		}
		start = i + 1
	}
	end := min(start+pageSize, len(items))
	page := slices.Clone(items[start:end])
	if page == nil {
		page = []T{}
	}
	if end == len(items) {
		return page, "", nil
	}
	return page, encodeCursor(scope, key(items[end-1])), nil
}

// encodeCursor returns the base64url encoding of the scope and the key
// of the last returned item.
func encodeCursor(scope, key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(scope + "\x00" + key))
}

// cacheable returns the cache hints of the skills results.
func cacheable() mcp.Cacheable {
	return mcp.Cacheable{TTLMs: ttlMs, CacheScope: cacheScopePublic}
}

// invalidParams returns a JSON-RPC Invalid Params (-32602) error.
func invalidParams(message string) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: message}
}

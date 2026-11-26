// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package skills

import (
	"bytes"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ExtensionID is the capability identifier of the MCP Skills extension.
	ExtensionID = "io.modelcontextprotocol/skills"
	// MethodList is the skills/list method name.
	MethodList = "skills/list"
	// MethodGet is the skills/get method name.
	MethodGet = "skills/get"
	// MethodReadDirectory is the resources/directory/read method name.
	MethodReadDirectory = "resources/directory/read"

	// URIScheme is the URI scheme of the skill resources.
	URIScheme = "skill://"
	// DirectoryMIMEType is the MIME type of directory resources.
	DirectoryMIMEType = "inode/directory"

	resultTypeComplete = "complete"
	cacheScopePublic   = "public"
)

// Frontmatter is the YAML frontmatter of a SKILL.md represented as
// JSON-compatible values. JSON numbers are decoded as [json.Number]
// to preserve their precision.
type Frontmatter map[string]any

// Name returns the skill name from the frontmatter.
func (f Frontmatter) Name() string { name, _ := f["name"].(string); return name }

// Description returns the skill description from the frontmatter.
func (f Frontmatter) Description() string { desc, _ := f["description"].(string); return desc }

// UnmarshalJSON decodes the frontmatter preserving the precision of numbers.
func (f *Frontmatter) UnmarshalJSON(data []byte) error {
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return err
	}
	*f = fields
	return nil
}

// Resource identifies and fingerprints one file of a skill.
type Resource struct {
	// URI is the resource URI of the file.
	URI string `json:"uri"`
	// Digest is the SHA-256 hash of the raw file bytes, formatted as
	// "sha256:" followed by 64 lowercase hexadecimal digits.
	Digest string `json:"digest"`
	// Size is the length of the raw file content in bytes.
	Size int64 `json:"size"`
}

// Skill is an entry returned by skills/list and skills/get.
type Skill struct {
	// URI is the resource URI of the skill's SKILL.md.
	URI string `json:"uri"`
	// Frontmatter holds the fields of the SKILL.md YAML frontmatter.
	Frontmatter Frontmatter `json:"frontmatter"`
	// Resources is the complete manifest of the skill's files,
	// including SKILL.md and every supporting file.
	Resources []*Resource `json:"resources"`
}

// ListSkillsParams contains the parameters of skills/list.
type ListSkillsParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

// ListSkillsResult is one page of skills/list. Each Skill is a complete
// entry, its manifest is never split across pages.
type ListSkillsResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string   `json:"resultType"`
	NextCursor string   `json:"nextCursor,omitempty"`
	Skills     []*Skill `json:"skills"`
}

// GetSkillParams contains the parameters of skills/get.
type GetSkillParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

// GetSkillResult is the result of skills/get.
type GetSkillResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string `json:"resultType"`
	Skill      *Skill `json:"skill"`
}

// ReadDirectoryParams contains the parameters of resources/directory/read.
type ReadDirectoryParams struct {
	mcp.ParamsBase
	URI    string `json:"uri"`
	Cursor string `json:"cursor,omitempty"`
}

// ReadDirectoryResult is one page of resources/directory/read
// listing the direct children of a directory resource.
type ReadDirectoryResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string          `json:"resultType"`
	NextCursor string          `json:"nextCursor,omitempty"`
	Resources  []*mcp.Resource `json:"resources"`
}

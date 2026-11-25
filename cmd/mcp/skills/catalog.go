// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package skills embeds the Flux agent skills published at
// https://github.com/fluxcd/agent-skills and serves them from the MCP server
// as resources under the skill:// URI scheme, implementing the MCP Skills
// extension (io.modelcontextprotocol/skills) methods skills/list, skills/get
// and resources/directory/read.
//
// The MCP Go SDK does not support the Skills extension yet, so the protocol
// types and handlers are implemented here with the SDK primitives, following
// the API proposed upstream in:
//   - https://github.com/modelcontextprotocol/go-sdk/pull/1238 (protocol support)
//   - https://github.com/modelcontextprotocol/go-sdk/pull/1240 (filesystem provider)
package skills

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.yaml.in/yaml/v3"
)

const (
	// MaxFilesPerSkill is the maximum number of files in a skill,
	// SKILL.md included, as defined by the MCP Skills extension.
	MaxFilesPerSkill = 512
	// MaxSkillSize is the maximum total size in bytes of the files
	// of a skill, as defined by the MCP Skills extension.
	MaxSkillSize = 16 * 1024 * 1024

	skillFile            = "SKILL.md"
	maxNameLength        = 64
	maxDescriptionLength = 1024
	defaultPageSize      = 100
	// ttlMs is the cache hint of the skills results.
	ttlMs = 60 * 60 * 1000
)

//go:embed all:data
var embedded embed.FS

var nameRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// file is a skill file served as a resource.
type file struct {
	resource *mcp.Resource
	content  []byte
}

// Catalog holds the validated skills and the resources derived from them.
type Catalog struct {
	skills      []*Skill
	skillsByURI map[string]*Skill
	files       []*file
	filesByURI  map[string]*file
	directories map[string][]*mcp.Resource
	pageSize    int
}

// LoadEmbedded loads and validates the agent skills embedded in the binary.
func LoadEmbedded() (*Catalog, error) {
	fsys, err := fs.Sub(embedded, "data")
	if err != nil {
		return nil, err
	}
	return Load(fsys)
}

// Load reads and validates the skills found in the top-level directories of fsys.
func Load(fsys fs.FS) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read skills: %w", err)
	}

	c := &Catalog{
		skillsByURI: make(map[string]*Skill),
		filesByURI:  make(map[string]*file),
		directories: make(map[string][]*mcp.Resource),
		pageSize:    defaultPageSize,
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("unexpected file %q in the skills root", entry.Name())
		}
		if err := c.loadSkill(fsys, entry.Name()); err != nil {
			return nil, fmt.Errorf("invalid skill %q: %w", entry.Name(), err)
		}
	}
	if len(c.skills) == 0 {
		return nil, errors.New("no skills found")
	}

	slices.SortFunc(c.skills, func(a, b *Skill) int {
		return strings.Compare(a.URI, b.URI)
	})
	for uri := range c.directories {
		slices.SortFunc(c.directories[uri], func(a, b *mcp.Resource) int {
			return strings.Compare(a.URI, b.URI)
		})
	}
	return c, nil
}

// loadSkill validates the skill in the given directory and adds
// its files and directories to the catalog.
func (c *Catalog) loadSkill(fsys fs.FS, name string) error {
	if !nameRegexp.MatchString(name) || len(name) > maxNameLength {
		return fmt.Errorf("name must be 1-%d lowercase alphanumeric characters or hyphens", maxNameLength)
	}

	skillMD, err := fs.ReadFile(fsys, path.Join(name, skillFile))
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", skillFile, err)
	}
	frontmatter, err := parseFrontmatter(skillMD)
	if err != nil {
		return fmt.Errorf("invalid %s frontmatter: %w", skillFile, err)
	}
	if frontmatter.Name() != name {
		return fmt.Errorf("frontmatter name %q does not match the directory name", frontmatter.Name())
	}
	if desc := frontmatter.Description(); strings.TrimSpace(desc) == "" || utf8.RuneCountInString(desc) > maxDescriptionLength {
		return fmt.Errorf("frontmatter description must be a string of 1-%d characters", maxDescriptionLength)
	}

	rootURI := URIScheme + name
	skill := &Skill{
		URI:         rootURI + "/" + skillFile,
		Frontmatter: frontmatter,
	}
	c.directories[rootURI] = []*mcp.Resource{}

	var totalSize int64
	err = fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, name+"/")
		if p == name {
			return nil
		}
		if !validPath(rel) {
			return fmt.Errorf("path %q contains characters that are not allowed in URIs", rel)
		}
		uri := rootURI + "/" + rel
		parent := uri[:strings.LastIndex(uri, "/")]
		if d.IsDir() {
			c.directories[uri] = []*mcp.Resource{}
			c.directories[parent] = append(c.directories[parent], &mcp.Resource{
				URI:      uri,
				Name:     d.Name(),
				MIMEType: DirectoryMIMEType,
			})
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%q is not a regular file", rel)
		}

		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		if len(skill.Resources) >= MaxFilesPerSkill {
			return fmt.Errorf("skill exceeds the limit of %d files", MaxFilesPerSkill)
		}
		totalSize += int64(len(content))
		if totalSize > MaxSkillSize {
			return fmt.Errorf("skill exceeds the size limit of %d bytes", MaxSkillSize)
		}

		resource := &mcp.Resource{
			URI:      uri,
			Name:     d.Name(),
			MIMEType: mimeType(d.Name()),
			Size:     int64(len(content)),
		}
		if rel == skillFile {
			resource.Name = name
			resource.Description = frontmatter.Description()
		}
		f := &file{resource: resource, content: content}
		c.files = append(c.files, f)
		c.filesByURI[uri] = f
		c.directories[parent] = append(c.directories[parent], resource)

		digest := sha256.Sum256(content)
		entry := &Resource{
			URI:    uri,
			Digest: "sha256:" + hex.EncodeToString(digest[:]),
			Size:   int64(len(content)),
		}
		skill.Resources = append(skill.Resources, entry)
		return nil
	})
	if err != nil {
		return err
	}
	if _, ok := c.filesByURI[skill.URI]; !ok {
		return fmt.Errorf("%s is not a regular file", skillFile)
	}

	// The SKILL.md entry comes first, followed by the supporting files in URI order.
	slices.SortFunc(skill.Resources, func(a, b *Resource) int {
		switch {
		case a.URI == skill.URI:
			return -1
		case b.URI == skill.URI:
			return 1
		}
		return strings.Compare(a.URI, b.URI)
	})

	c.skills = append(c.skills, skill)
	c.skillsByURI[skill.URI] = skill
	return nil
}

// parseFrontmatter extracts the YAML frontmatter at the start of a
// SKILL.md and converts it to a JSON object, keeping all fields.
func parseFrontmatter(content []byte) (Frontmatter, error) {
	block, err := extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var fields map[string]any
	if err := yaml.Unmarshal(block, &fields); err != nil {
		return nil, fmt.Errorf("frontmatter must be a YAML mapping: %w", err)
	}
	if fields == nil {
		return nil, errors.New("frontmatter must be a YAML mapping")
	}
	// Convert to JSON values, with numbers as json.Number.
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("frontmatter cannot be converted to JSON: %w", err)
	}
	var frontmatter Frontmatter
	if err := json.Unmarshal(data, &frontmatter); err != nil {
		return nil, err
	}
	if _, ok := frontmatter["name"].(string); !ok {
		return nil, errors.New("name must be a string")
	}
	if _, ok := frontmatter["description"].(string); !ok {
		return nil, errors.New("description must be a string")
	}
	return frontmatter, nil
}

// extractFrontmatter returns the YAML block between the '---' lines at the
// start of a SKILL.md, with CRLF line endings converted to LF.
func extractFrontmatter(content []byte) ([]byte, error) {
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(content, []byte("---\n"))
	if !ok {
		return nil, errors.New("file must start with a '---' line")
	}
	if bytes.HasPrefix(rest, []byte("---\n")) || bytes.Equal(rest, []byte("---")) {
		return nil, errors.New("frontmatter is empty")
	}
	// The YAML block keeps its final newline, which is part of
	// the value of a trailing block scalar.
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		if !bytes.HasSuffix(rest, []byte("\n---")) {
			return nil, errors.New("closing '---' line not found")
		}
		end = len(rest) - len("\n---")
	}
	return rest[:end+1], nil
}

// validPath reports whether every segment of the slash-separated
// path can be used verbatim in a URI path.
func validPath(p string) bool {
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." || url.PathEscape(seg) != seg {
			return false
		}
	}
	return true
}

// mimeType returns the MIME type of a skill file based on its extension.
func mimeType(name string) string {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".sh":
		return "text/x-shellscript"
	case ".py":
		return "text/x-python"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		if mt, _, err := mime.ParseMediaType(t); err == nil {
			return mt
		}
	}
	return "application/octet-stream"
}

// Skills returns the skill entries ordered by URI.
func (c *Catalog) Skills() []*Skill {
	return c.skills
}

// Instructions returns the text appended to the server instructions
// pointing the agent to the URIs of the served skills.
func (c *Catalog) Instructions() string {
	var b strings.Builder
	b.WriteString("Flux agent skills are served as MCP resources. When a task matches a skill, " +
		"read its SKILL.md with resources/read and resolve the files it references relative to the skill URI:\n")
	for _, s := range c.skills {
		fmt.Fprintf(&b, "- %s: %s\n", s.URI, summary(s.Frontmatter.Description()))
	}
	return strings.TrimSpace(b.String())
}

// summary returns the first clause of a skill description,
// ending at the first sentence or em dash separator.
func summary(desc string) string {
	desc = strings.Join(strings.Fields(desc), " ")
	for _, sep := range []string{". ", " — ", " - "} {
		if before, _, found := strings.Cut(desc, sep); found {
			desc = before
		}
	}
	desc = strings.TrimSuffix(desc, ".")
	const maxLen = 160
	if utf8.RuneCountInString(desc) > maxLen {
		runes := []rune(desc)
		desc = string(runes[:maxLen])
		if i := strings.LastIndex(desc, " "); i > 0 {
			desc = desc[:i]
		}
		desc += "…"
	}
	return desc
}

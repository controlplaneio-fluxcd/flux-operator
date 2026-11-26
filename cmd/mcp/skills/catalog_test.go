// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	. "github.com/onsi/gomega"
	"go.yaml.in/yaml/v3"
)

const fixtureSkillMD = `---
name: demo
description: >
  Demo skill for tests. Second sentence.
license: Apache-2.0
metadata:
  version: 1.0
  count: 12345678901234567890
  tags:
    - a
    - b
---

# Demo

Read references/guide.md.
`

// fixtureFS returns a valid skills tree with two skills.
func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"demo/SKILL.md":                   {Data: []byte(fixtureSkillMD)},
		"demo/references/guide.md":        {Data: []byte("# Guide\n")},
		"demo/references/nested/deep.txt": {Data: []byte("deep\n")},
		"demo/references-extra.md":        {Data: []byte("extra\n")},
		"demo/scripts/run.sh":             {Data: []byte("#!/bin/sh\necho ok\n")},
		"demo/assets/schema.json":         {Data: []byte("{}\n")},
		"demo/assets/logo.bin":            {Data: []byte{0xff, 0xfe, 0x00, 0x01}},
		"other/SKILL.md":                  {Data: []byte("---\nname: other\ndescription: Other skill\n---\nbody\n")},
	}
}

func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestLoadEmbedded(t *testing.T) {
	g := NewWithT(t)

	catalog, err := LoadEmbedded()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(catalog.Skills()).ToNot(BeEmpty())

	var previous string
	for _, skill := range catalog.Skills() {
		// Skills are ordered by URI.
		g.Expect(skill.URI > previous).To(BeTrue(), "skill %s order", skill.URI)
		previous = skill.URI

		// The URI ends with the skill name followed by SKILL.md.
		name := skill.Frontmatter.Name()
		g.Expect(skill.URI).To(Equal(URIScheme + name + "/SKILL.md"))
		g.Expect(skill.Frontmatter.Description()).ToNot(BeEmpty())

		// The manifest starts with SKILL.md, is complete, unique,
		// within limits and matches the served bytes.
		g.Expect(skill.Resources).ToNot(BeEmpty())
		g.Expect(skill.Resources[0].URI).To(Equal(skill.URI))
		g.Expect(len(skill.Resources)).To(BeNumerically("<=", MaxFilesPerSkill))
		seen := make(map[string]bool)
		var total int64
		for i, res := range skill.Resources {
			g.Expect(seen[res.URI]).To(BeFalse(), "duplicate %s", res.URI)
			seen[res.URI] = true
			if i > 1 {
				g.Expect(res.URI > skill.Resources[i-1].URI).To(BeTrue(), "resource %s order", res.URI)
			}
			g.Expect(res.URI).To(HavePrefix(URIScheme + name + "/"))
			g.Expect(res.URI).ToNot(ContainSubstring("/evals/"))
			g.Expect(res.Digest).To(MatchRegexp(`^sha256:[0-9a-f]{64}$`))

			f, ok := catalog.filesByURI[res.URI]
			g.Expect(ok).To(BeTrue())
			content := f.content
			g.Expect(res.Size).To(Equal(int64(len(content))))
			g.Expect(res.Digest).To(Equal(sha256Digest(content)))
			total += res.Size
		}
		g.Expect(total).To(BeNumerically("<=", MaxSkillSize))

		// The frontmatter has the fields of the SKILL.md frontmatter,
		// and its string fields are unchanged.
		block, err := extractFrontmatter(catalog.filesByURI[skill.URI].content)
		g.Expect(err).ToNot(HaveOccurred())
		var raw map[string]any
		g.Expect(yaml.Unmarshal(block, &raw)).To(Succeed())
		g.Expect(skill.Frontmatter).To(HaveLen(len(raw)))
		for k, v := range raw {
			g.Expect(skill.Frontmatter).To(HaveKey(k))
			if str, ok := v.(string); ok {
				g.Expect(skill.Frontmatter[k]).To(Equal(str), "frontmatter field %s", k)
			}
		}

		// Every directory of the skill is listed, starting with the skill root.
		_, ok := catalog.directories[strings.TrimSuffix(skill.URI, "/SKILL.md")]
		g.Expect(ok).To(BeTrue())
	}

	// The embedded data is used in the instructions.
	instructions := catalog.Instructions()
	for _, skill := range catalog.Skills() {
		g.Expect(instructions).To(ContainSubstring(skill.URI))
	}
	g.Expect(instructions).ToNot(HaveSuffix("\n"))
}

func TestLoad_Fixture(t *testing.T) {
	g := NewWithT(t)

	fsys := fixtureFS()
	catalog, err := Load(fsys)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(catalog.Skills()).To(HaveLen(2))

	skill, ok := catalog.skillsByURI["skill://demo/SKILL.md"]
	g.Expect(ok).To(BeTrue())

	// All frontmatter fields are preserved.
	data, err := json.Marshal(skill.Frontmatter)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(data)).To(MatchJSON(`{
		"name": "demo",
		"description": "Demo skill for tests. Second sentence.\n",
		"license": "Apache-2.0",
		"metadata": {"version": 1.0, "count": 12345678901234567890, "tags": ["a", "b"]}
	}`))
	g.Expect(string(data)).To(ContainSubstring("12345678901234567890"))

	// SKILL.md comes first, the other files follow in URI order.
	uris := make([]string, 0, len(skill.Resources))
	for _, res := range skill.Resources {
		uris = append(uris, res.URI)
		file := strings.TrimPrefix(res.URI, "skill://")
		g.Expect(res.Digest).To(Equal(sha256Digest(fsys[file].Data)))
		g.Expect(res.Size).To(Equal(int64(len(fsys[file].Data))))
	}
	g.Expect(uris).To(Equal([]string{
		"skill://demo/SKILL.md",
		"skill://demo/assets/logo.bin",
		"skill://demo/assets/schema.json",
		"skill://demo/references-extra.md",
		"skill://demo/references/guide.md",
		"skill://demo/references/nested/deep.txt",
		"skill://demo/scripts/run.sh",
	}))

	// Directories list their direct children in URI order.
	names := func(uri string) []string {
		out := make([]string, 0, len(catalog.directories[uri]))
		for _, r := range catalog.directories[uri] {
			out = append(out, r.Name+"|"+r.MIMEType)
		}
		return out
	}
	g.Expect(names("skill://demo")).To(Equal([]string{
		"demo|text/markdown",
		"assets|inode/directory",
		"references|inode/directory",
		"references-extra.md|text/markdown",
		"scripts|inode/directory",
	}))
	g.Expect(names("skill://demo/references")).To(Equal([]string{
		"guide.md|text/markdown",
		"nested|inode/directory",
	}))
	g.Expect(names("skill://demo/references/nested")).To(Equal([]string{"deep.txt|text/plain"}))
	g.Expect(names("skill://demo/scripts")).To(Equal([]string{"run.sh|text/x-shellscript"}))
	g.Expect(names("skill://demo/assets")).To(Equal([]string{
		"logo.bin|application/octet-stream",
		"schema.json|application/json",
	}))

	// The SKILL.md resource carries the skill name and description.
	md := catalog.filesByURI["skill://demo/SKILL.md"].resource
	g.Expect(md.Name).To(Equal("demo"))
	g.Expect(md.Description).To(ContainSubstring("Demo skill for tests."))

	g.Expect(catalog.Instructions()).To(ContainSubstring("- skill://demo/SKILL.md: Demo skill for tests\n"))
	g.Expect(catalog.Instructions()).To(HaveSuffix("- skill://other/SKILL.md: Other skill"))
}

func TestLoad_Validation(t *testing.T) {
	skill := func(frontmatter string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte(frontmatter + "\n# Body\n")}
	}
	valid := skill("---\nname: demo\ndescription: Demo\n---")

	manyFiles := fstest.MapFS{"demo/SKILL.md": valid}
	for i := range MaxFilesPerSkill {
		manyFiles[fmt.Sprintf("demo/files/%03d.md", i)] = &fstest.MapFile{Data: []byte("x")}
	}

	tests := []struct {
		name    string
		fsys    fstest.MapFS
		wantErr string
	}{
		{
			name:    "empty",
			fsys:    fstest.MapFS{},
			wantErr: "no skills found",
		},
		{
			name:    "file in root",
			fsys:    fstest.MapFS{"demo/SKILL.md": valid, "README.md": {Data: []byte("x")}},
			wantErr: `unexpected file "README.md"`,
		},
		{
			name:    "missing SKILL.md",
			fsys:    fstest.MapFS{"demo/README.md": {Data: []byte("x")}},
			wantErr: "failed to read SKILL.md",
		},
		{
			name:    "no frontmatter",
			fsys:    fstest.MapFS{"demo/SKILL.md": {Data: []byte("# Demo\n")}},
			wantErr: "file must start with a '---' line",
		},
		{
			name:    "unclosed frontmatter",
			fsys:    fstest.MapFS{"demo/SKILL.md": {Data: []byte("---\nname: demo\ndescription: Demo\n")}},
			wantErr: "closing '---' line not found",
		},
		{
			name:    "empty frontmatter",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\n---")},
			wantErr: "frontmatter is empty",
		},
		{
			name:    "frontmatter not a mapping",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\n- name\n---")},
			wantErr: "frontmatter must be a YAML mapping",
		},
		{
			name:    "invalid YAML",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\nname: [demo\n---")},
			wantErr: "invalid SKILL.md frontmatter",
		},
		{
			name:    "missing name",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\ndescription: Demo\n---")},
			wantErr: "name must be a string",
		},
		{
			name:    "missing description",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\nname: demo\n---")},
			wantErr: "description must be a string",
		},
		{
			name:    "empty description",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\nname: demo\ndescription: ' '\n---")},
			wantErr: "frontmatter description must be a string of 1-1024 characters",
		},
		{
			name:    "name mismatch",
			fsys:    fstest.MapFS{"demo/SKILL.md": skill("---\nname: other\ndescription: Demo\n---")},
			wantErr: `frontmatter name "other" does not match the directory name`,
		},
		{
			name:    "invalid directory name",
			fsys:    fstest.MapFS{"Demo_1/SKILL.md": skill("---\nname: Demo_1\ndescription: Demo\n---")},
			wantErr: "name must be 1-64 lowercase alphanumeric characters or hyphens",
		},
		{
			name:    "SKILL.md is a directory",
			fsys:    fstest.MapFS{"demo/SKILL.md/x.md": {Data: []byte("x")}},
			wantErr: "failed to read SKILL.md",
		},
		{
			name:    "path not allowed in URIs",
			fsys:    fstest.MapFS{"demo/SKILL.md": valid, "demo/my file.md": {Data: []byte("x")}},
			wantErr: `path "my file.md" contains characters that are not allowed in URIs`,
		},
		{
			name:    "symlink",
			fsys:    fstest.MapFS{"demo/SKILL.md": valid, "demo/link": {Data: []byte("SKILL.md"), Mode: fs.ModeSymlink}},
			wantErr: `"link" is not a regular file`,
		},
		{
			name:    "too many files",
			fsys:    manyFiles,
			wantErr: "skill exceeds the limit of 512 files",
		},
		{
			name: "too large",
			fsys: fstest.MapFS{
				"demo/SKILL.md":  valid,
				"demo/large.bin": {Data: bytes.Repeat([]byte{0}, MaxSkillSize)},
			},
			wantErr: "skill exceeds the size limit of 16777216 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := Load(tt.fsys)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
		})
	}
}

func TestLoad_Limits(t *testing.T) {
	g := NewWithT(t)

	// A skill at the exact limits is accepted.
	valid := []byte("---\nname: demo\ndescription: Demo\n---\n")
	fsys := fstest.MapFS{"demo/SKILL.md": {Data: valid}}
	for i := range MaxFilesPerSkill - 2 {
		fsys[path.Join("demo/files", fmt.Sprintf("%03d.md", i))] = &fstest.MapFile{Data: []byte("x")}
	}
	used := len(valid) + MaxFilesPerSkill - 2
	fsys["demo/large.bin"] = &fstest.MapFile{Data: bytes.Repeat([]byte{0}, MaxSkillSize-used)}

	catalog, err := Load(fsys)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(catalog.Skills()[0].Resources).To(HaveLen(MaxFilesPerSkill))
}

func TestParseFrontmatter_CRLF(t *testing.T) {
	g := NewWithT(t)

	fm, err := parseFrontmatter([]byte("---\r\nname: demo\r\ndescription: Demo\r\n---\r\nbody\r\n"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fm.Name()).To(Equal("demo"))
	g.Expect(fm.Description()).To(Equal("Demo"))

	fm, err = parseFrontmatter([]byte("---\nname: demo\ndescription: Demo\n---"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fm.Name()).To(Equal("demo"))
}

func TestParseFrontmatter_Strings(t *testing.T) {
	g := NewWithT(t)

	// on and yes stay strings, a trailing block scalar keeps its newline.
	fm, err := parseFrontmatter([]byte("---\nname: demo\ntoggle: on\nanswer: yes\ndescription: |\n  Demo\n---"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fm).To(Equal(Frontmatter{
		"name":        "demo",
		"toggle":      "on",
		"answer":      "yes",
		"description": "Demo\n",
	}))
}

func TestLoad_DescriptionRunes(t *testing.T) {
	g := NewWithT(t)

	// The description limit counts characters, not bytes.
	desc := strings.Repeat("é", maxDescriptionLength)
	fsys := fstest.MapFS{"demo/SKILL.md": {Data: []byte("---\nname: demo\ndescription: " + desc + "\n---\n")}}
	_, err := Load(fsys)
	g.Expect(err).ToNot(HaveOccurred())

	fsys["demo/SKILL.md"].Data = []byte("---\nname: demo\ndescription: " + desc + "é\n---\n")
	_, err = Load(fsys)
	g.Expect(err).To(MatchError(ContainSubstring("frontmatter description must be a string of 1-1024 characters")))
}

func TestSummary(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{desc: "Flux CD expert — answers questions.", want: "Flux CD expert"},
		{desc: "Debug Flux clusters. Use when users ask.", want: "Debug Flux clusters"},
		{desc: "Audit\n  repositories (not clusters) - runs checks", want: "Audit repositories (not clusters)"},
		{desc: "Single sentence.", want: "Single sentence"},
		{desc: strings.Repeat("word ", 50), want: strings.TrimSpace(strings.Repeat("word ", 32)) + "…"},
	}
	for _, tt := range tests {
		g := NewWithT(t)
		g.Expect(summary(tt.desc)).To(Equal(tt.want))
	}
}

func TestMIMEType(t *testing.T) {
	g := NewWithT(t)
	g.Expect(mimeType("SKILL.md")).To(Equal("text/markdown"))
	g.Expect(mimeType("a.MD")).To(Equal("text/markdown"))
	g.Expect(mimeType("a.fields.txt")).To(Equal("text/plain"))
	g.Expect(mimeType("a.sh")).To(Equal("text/x-shellscript"))
	g.Expect(mimeType("a.json")).To(Equal("application/json"))
	g.Expect(mimeType("a.yaml")).To(Equal("application/yaml"))
	g.Expect(mimeType("noext")).To(Equal("application/octet-stream"))
}

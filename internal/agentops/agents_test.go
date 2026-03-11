// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"
)

func TestAgentIDs(t *testing.T) {
	g := NewWithT(t)

	ids := AgentIDs()
	g.Expect(ids).ToNot(BeEmpty())

	// Must be sorted.
	g.Expect(sort.StringsAreSorted(ids)).To(BeTrue(), "AgentIDs must be sorted")

	// No duplicates.
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		g.Expect(seen[id]).To(BeFalse(), "duplicate agent ID: %s", id)
		seen[id] = true
	}
}

func TestFindAgent(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		g := NewWithT(t)
		info := FindAgent("claude-code")
		g.Expect(info).ToNot(BeNil())
		g.Expect(info.Name).To(Equal("Claude Code"))
		g.Expect(info.ProjectPath).To(Equal(".claude/skills"))
	})

	t.Run("not found", func(t *testing.T) {
		g := NewWithT(t)
		info := FindAgent("nonexistent-agent")
		g.Expect(info).To(BeNil())
	})
}

func TestUsesDefaultSkillsDir(t *testing.T) {
	t.Run("default path for universal", func(t *testing.T) {
		g := NewWithT(t)
		info := FindAgent("universal")
		g.Expect(info).ToNot(BeNil())
		g.Expect(UsesDefaultSkillsDir(info)).To(BeTrue())
	})

	t.Run("default path for cursor", func(t *testing.T) {
		g := NewWithT(t)
		info := FindAgent("cursor")
		g.Expect(info).ToNot(BeNil())
		g.Expect(UsesDefaultSkillsDir(info)).To(BeTrue())
	})

	t.Run("custom path", func(t *testing.T) {
		g := NewWithT(t)
		info := FindAgent("claude-code")
		g.Expect(info).ToNot(BeNil())
		g.Expect(UsesDefaultSkillsDir(info)).To(BeFalse())
	})
}

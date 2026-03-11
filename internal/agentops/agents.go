// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"sort"
)

// AgentInfo holds the configuration for an AI agent's skills directory.
type AgentInfo struct {
	// Name is the human-readable agent name.
	Name string

	// ID is the unique identifier used with the --agent flag.
	ID string

	// ProjectPath is the skills directory path relative to the project root.
	ProjectPath string

	// GlobalPath is the global skills directory path (~ is expanded at runtime).
	GlobalPath string
}

// agentRegistry is the sorted list of all known AI agents.
var agentRegistry = []AgentInfo{
	{Name: "AdaL", ID: "adal", ProjectPath: ".adal/skills", GlobalPath: "~/.adal/skills"},
	{Name: "Amp", ID: "amp", ProjectPath: DefaultSkillsDirName, GlobalPath: DefaultGlobalSkillsDirName},
	{Name: "Antigravity", ID: "antigravity", ProjectPath: ".agent/skills", GlobalPath: "~/.gemini/antigravity/skills"},
	{Name: "Augment", ID: "augment", ProjectPath: ".augment/skills", GlobalPath: "~/.augment/skills"},
	{Name: "Claude Code", ID: "claude-code", ProjectPath: ".claude/skills", GlobalPath: "~/.claude/skills"},
	{Name: "Cline", ID: "cline", ProjectPath: DefaultSkillsDirName, GlobalPath: "~/.agents/skills"},
	{Name: "CodeBuddy", ID: "codebuddy", ProjectPath: ".codebuddy/skills", GlobalPath: "~/.codebuddy/skills"},
	{Name: "Codex", ID: "codex", ProjectPath: DefaultSkillsDirName, GlobalPath: "~/.codex/skills"},
	{Name: "Command Code", ID: "command-code", ProjectPath: ".commandcode/skills", GlobalPath: "~/.commandcode/skills"},
	{Name: "Continue", ID: "continue", ProjectPath: ".continue/skills", GlobalPath: "~/.continue/skills"},
	{Name: "Cortex Code", ID: "cortex", ProjectPath: ".cortex/skills", GlobalPath: "~/.snowflake/cortex/skills"},
	{Name: "Crush", ID: "crush", ProjectPath: ".crush/skills", GlobalPath: "~/.config/crush/skills"},
	{Name: "Cursor", ID: "cursor", ProjectPath: DefaultSkillsDirName, GlobalPath: "~/.cursor/skills"},
	{Name: "Droid", ID: "droid", ProjectPath: DefaultSkillsDirName, GlobalPath: DefaultSkillsDirName},
	{Name: "Gemini CLI", ID: "gemini-cli", ProjectPath: DefaultSkillsDirName, GlobalPath: "~/.gemini/skills"},
	{Name: "GitHub Copilot", ID: "github-copilot", ProjectPath: ".github/skills", GlobalPath: "~/.copilot/skills"},
	{Name: "Goose", ID: "goose", ProjectPath: DefaultSkillsDirName, GlobalPath: DefaultGlobalSkillsDirName},
	{Name: "iFlow CLI", ID: "iflow-cli", ProjectPath: ".iflow/skills", GlobalPath: "~/.iflow/skills"},
	{Name: "Junie", ID: "junie", ProjectPath: ".junie/skills", GlobalPath: "~/.junie/skills"},
	{Name: "Kilo Code", ID: "kilo", ProjectPath: ".kilocode/skills", GlobalPath: "~/.kilocode/skills"},
	{Name: "Kimi Code CLI", ID: "kimi-cli", ProjectPath: DefaultSkillsDirName, GlobalPath: DefaultGlobalSkillsDirName},
	{Name: "Kiro", ID: "kiro", ProjectPath: ".kiro/skills", GlobalPath: "~/.kiro/skills"},
	{Name: "Kode", ID: "kode", ProjectPath: ".kode/skills", GlobalPath: "~/.kode/skills"},
	{Name: "MCPJam", ID: "mcpjam", ProjectPath: ".mcpjam/skills", GlobalPath: "~/.mcpjam/skills"},
	{Name: "Mistral Vibe", ID: "mistral-vibe", ProjectPath: ".vibe/skills", GlobalPath: "~/.vibe/skills"},
	{Name: "Mux", ID: "mux", ProjectPath: ".mux/skills", GlobalPath: "~/.mux/skills"},
	{Name: "Neovate", ID: "neovate", ProjectPath: ".neovate/skills", GlobalPath: "~/.neovate/skills"},
	{Name: "OpenClaw", ID: "openclaw", ProjectPath: "skills", GlobalPath: "~/.openclaw/skills"},
	{Name: "OpenCode", ID: "opencode", ProjectPath: DefaultSkillsDirName, GlobalPath: "~/.config/opencode/skills"},
	{Name: "OpenHands", ID: "openhands", ProjectPath: ".openhands/skills", GlobalPath: "~/.openhands/skills"},
	{Name: "Pi", ID: "pi", ProjectPath: ".pi/skills", GlobalPath: "~/.pi/agent/skills"},
	{Name: "Pochi", ID: "pochi", ProjectPath: ".pochi/skills", GlobalPath: "~/.pochi/skills"},
	{Name: "Qoder", ID: "qoder", ProjectPath: ".qoder/skills", GlobalPath: "~/.qoder/skills"},
	{Name: "Qwen Code", ID: "qwen-code", ProjectPath: ".qwen/skills", GlobalPath: "~/.qwen/skills"},
	{Name: "Replit", ID: "replit", ProjectPath: DefaultSkillsDirName, GlobalPath: DefaultGlobalSkillsDirName},
	{Name: "Roo Code", ID: "roo", ProjectPath: ".roo/skills", GlobalPath: "~/.roo/skills"},
	{Name: "Trae", ID: "trae", ProjectPath: ".trae/skills", GlobalPath: "~/.trae/skills"},
	{Name: "Trae CN", ID: "trae-cn", ProjectPath: ".trae/skills", GlobalPath: "~/.trae-cn/skills"},
	{Name: "Universal", ID: "universal", ProjectPath: DefaultSkillsDirName, GlobalPath: DefaultGlobalSkillsDirName},
	{Name: "Windsurf", ID: "windsurf", ProjectPath: ".windsurf/skills", GlobalPath: "~/.codeium/windsurf/skills"},
	{Name: "Zencoder", ID: "zencoder", ProjectPath: ".zencoder/skills", GlobalPath: "~/.zencoder/skills"},
}

// AgentIDs returns a sorted list of all known agent IDs.
func AgentIDs() []string {
	ids := make([]string, len(agentRegistry))
	for i, a := range agentRegistry {
		ids[i] = a.ID
	}
	sort.Strings(ids)
	return ids
}

// FindAgent returns the AgentInfo for the given ID, or nil if not found.
func FindAgent(id string) *AgentInfo {
	for i := range agentRegistry {
		if agentRegistry[i].ID == id {
			return &agentRegistry[i]
		}
	}
	return nil
}

// UsesDefaultSkillsDir returns true if the agent's project path is the
// default skills directory (no symlink needed).
func UsesDefaultSkillsDir(info *AgentInfo) bool {
	return info.ProjectPath == DefaultSkillsDirName
}

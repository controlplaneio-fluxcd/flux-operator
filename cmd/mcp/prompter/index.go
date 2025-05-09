// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

// PromptSet returns a slice of predefined Prompt objects
// with their associated names, descriptions, and handlers.
func (m *Manager) PromptSet() []SystemPrompt {
	return []SystemPrompt{
		m.NewDebugKustomizationPrompt(),
		m.NewDebugHelmReleasePrompt(),
	}
}

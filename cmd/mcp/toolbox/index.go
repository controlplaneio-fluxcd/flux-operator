// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

// ToolSet returns a list of tools supported by the Manager, including
// their names, descriptions, handlers, and read-only status.
func (m *Manager) ToolSet() []SystemTool {
	return []SystemTool{
		m.NewGetFluxInstanceTool(),
		m.NewGetApiVersionsTool(),
		m.NewGetKubernetesLogsTool(),
		m.NewGetKubernetesResourcesTool(),
		m.NewDeleteKubernetesResourceTool(),
		m.NewReconcileSourceTool(),
		m.NewReconcileKustomizationTool(),
		m.NewReconcileHelmReleaseTool(),
		m.NewReconcileResourceSetTool(),
		m.NewSuspendReconciliationTool(),
		m.NewResumeReconciliationTool(),
		m.NewGetKubeconfigContextsTool(),
		m.NewSetKubeconfigContextTool(),
		m.NewSearchFluxDocsTool(),
	}
}

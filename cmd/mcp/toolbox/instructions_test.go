// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
)

func TestManager_Instructions(t *testing.T) {
	tests := []struct {
		name       string
		readOnly   bool
		inCluster  bool
		localFiles bool
	}{
		{name: "all tools"},
		{name: "local files", localFiles: true},
		{name: "local files without diff", readOnly: true, localFiles: true},
		{name: "read-only", readOnly: true},
		{name: "in-cluster", inCluster: true},
		{name: "read-only in-cluster", readOnly: true, inCluster: true},
		{name: "enabled subset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			manager := NewManager(nil, 0, false, tt.readOnly, tt.localFiles)
			instructions := manager.Instructions(tt.inCluster)
			g.Expect(instructions).To(HavePrefix("This server connects to Kubernetes API"))
			g.Expect(instructions).ToNot(HaveSuffix("\n"))

			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
			registered := manager.RegisterTools(server, tt.inCluster)
			g.Expect(registered).ToNot(BeEmpty())

			// Every registered tool is named in the instructions
			// and no unregistered tool is mentioned.
			for tool := range systemTools {
				if manager.shouldRegisterTool(tool, tt.inCluster) {
					g.Expect(instructions).To(ContainSubstring(tool), "expected %s to be mentioned", tool)
				} else {
					g.Expect(instructions).ToNot(ContainSubstring(tool), "expected %s to be omitted", tool)
				}
			}

			if manager.shouldRegisterTool(ToolGetKubernetesLogs, tt.inCluster) {
				g.Expect(instructions).To(ContainSubstring("directly with the workload kind, name and namespace"))
				g.Expect(instructions).To(ContainSubstring("Omit container to read all regular containers"))
				g.Expect(instructions).ToNot(ContainSubstring("list its pods using the matchLabels"))
			}

			if manager.shouldRegisterTool(ToolDiffKubernetesManifest, tt.inCluster) {
				g.Expect(instructions).To(ContainSubstring("Before committing GitOps changes"))
				g.Expect(instructions).To(ContainSubstring("COMPLETE output"))
				g.Expect(instructions).To(ContainSubstring("owner_kind/owner_name/owner_namespace"))
				g.Expect(instructions).To(ContainSubstring("pass its YAML in flux_object"))
				g.Expect(instructions).To(ContainSubstring("ad-hoc manifests need none"))
			}

			localFilesInstruction := "pass the absolute path in yaml_path"
			if tt.localFiles && manager.shouldRegisterTool(ToolDiffKubernetesManifest, tt.inCluster) {
				g.Expect(instructions).To(ContainSubstring(localFilesInstruction))
			} else {
				g.Expect(instructions).ToNot(ContainSubstring(localFilesInstruction))
			}

			if manager.shouldRegisterTool(ToolDiffKubernetesManifest, tt.inCluster) &&
				manager.shouldRegisterTool(ToolApplyKubernetesManifest, tt.inCluster) {
				g.Expect(instructions).To(ContainSubstring("preview the changes with " + ToolDiffKubernetesManifest))
			}

			if tt.readOnly {
				g.Expect(instructions).To(ContainSubstring("read-only mode"))
				g.Expect(instructions).ToNot(ContainSubstring("Actions:"))
			} else {
				g.Expect(instructions).ToNot(ContainSubstring("read-only mode"))
				g.Expect(instructions).To(ContainSubstring("Actions:"))
			}
		})
	}
}

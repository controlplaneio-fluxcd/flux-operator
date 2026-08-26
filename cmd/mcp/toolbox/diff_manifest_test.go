// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleDiffKubernetesManifest(t *testing.T) {
	g := NewWithT(t)
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	tempDir := t.TempDir()
	yamlPath := filepath.Join(tempDir, "manifest.yaml")
	g.Expect(os.WriteFile(yamlPath, []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"), 0o600)).To(Succeed())

	oversizedPath := filepath.Join(tempDir, "oversized.yaml")
	oversizedFile, err := os.Create(oversizedPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(oversizedFile.Truncate(int64(maxYAMLPathSize + 1))).To(Succeed())
	g.Expect(oversizedFile.Close()).To(Succeed())

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: ToolDiffKubernetesManifest,
		},
	}

	tests := []struct {
		testName   string
		arguments  map[string]any
		localFiles bool
		matchErr   string
	}{
		{
			testName: "fails with both yaml inputs",
			arguments: map[string]any{
				"yaml_content": "test: test",
				"yaml_path":    yamlPath,
			},
			matchErr: "exactly one of yaml_content or yaml_path must be set",
		},
		{
			testName:  "fails with neither yaml input",
			arguments: map[string]any{},
			matchErr:  "exactly one of yaml_content or yaml_path must be set",
		},
		{
			testName: "fails with local files disabled",
			arguments: map[string]any{
				"yaml_path": yamlPath,
			},
			matchErr: "yaml_path is only available when the MCP server runs locally with the stdio transport, pass yaml_content instead",
		},
		{
			testName: "fails with relative path",
			arguments: map[string]any{
				"yaml_path": "manifest.yaml",
			},
			localFiles: true,
			matchErr:   "must be an absolute path because the server's working directory is unknown to the agent",
		},
		{
			testName: "fails with nonexistent file",
			arguments: map[string]any{
				"yaml_path": filepath.Join(tempDir, "missing.yaml"),
			},
			localFiles: true,
			matchErr:   "Failed to read yaml_path",
		},
		{
			testName: "fails with directory",
			arguments: map[string]any{
				"yaml_path": tempDir,
			},
			localFiles: true,
			matchErr:   "yaml_path must point to a regular file",
		},
		{
			testName: "fails with oversized file",
			arguments: map[string]any{
				"yaml_path": oversizedPath,
			},
			localFiles: true,
			matchErr:   "yaml_path exceeds the 64 MiB size limit",
		},
		{
			testName: "reads local yaml before creating client",
			arguments: map[string]any{
				"yaml_path": yamlPath,
			},
			localFiles: true,
			matchErr:   "Failed to get Kubernetes client",
		},
		{
			testName: "fails with partial owner reference",
			arguments: map[string]any{
				"yaml_content": "test: test",
				"owner_kind":   "Kustomization",
			},
			matchErr: "owner_kind, owner_name and owner_namespace must all be set",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]any{
				"yaml_content": "test: test",
			},
			matchErr: "Failed to get Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			m.localFiles = test.localFiles
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input diffKubernetesManifestInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleDiffKubernetesManifest(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}

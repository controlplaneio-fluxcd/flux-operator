// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolDiffKubernetesManifest is the name of the diff_kubernetes_manifest tool.
	ToolDiffKubernetesManifest = "diff_kubernetes_manifest"

	maxYAMLPathSize = 64 << 20
)

func init() {
	systemTools[ToolDiffKubernetesManifest] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// diffKubernetesManifestInput defines the input parameters for diffing a Kubernetes manifest.
type diffKubernetesManifestInput struct {
	YAMLContent    string `json:"yaml_content,omitempty" jsonschema:"Multi-doc YAML content to diff."`
	YAMLPath       string `json:"yaml_path,omitempty" jsonschema:"Absolute path to a local multi-doc YAML file to diff."`
	FluxObject     string `json:"flux_object,omitempty" jsonschema:"YAML of the owning Flux resource, when new or changed."`
	OwnerKind      string `json:"owner_kind,omitempty" jsonschema:"Owning Flux resource kind: Kustomization, HelmRelease or ResourceSet."`
	OwnerName      string `json:"owner_name,omitempty" jsonschema:"Name of the owning Flux resource."`
	OwnerNamespace string `json:"owner_namespace,omitempty" jsonschema:"Namespace of the owning Flux resource."`
}

// HandleDiffKubernetesManifest is the handler function for the diff_kubernetes_manifest tool.
func (m *Manager) HandleDiffKubernetesManifest(ctx context.Context, request *mcp.CallToolRequest, input diffKubernetesManifestInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolDiffKubernetesManifest, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if (input.YAMLContent == "") == (input.YAMLPath == "") {
		return NewToolResultError("exactly one of yaml_content or yaml_path must be set")
	}

	manifest := input.YAMLContent
	if input.YAMLPath != "" {
		if !m.localFiles {
			return NewToolResultError("yaml_path is only available when the MCP server runs locally with the stdio transport, pass yaml_content instead")
		}
		if !filepath.IsAbs(input.YAMLPath) {
			return NewToolResultError("yaml_path must be an absolute path because the server's working directory is unknown to the agent")
		}

		fileInfo, err := os.Stat(input.YAMLPath)
		if err != nil {
			return NewToolResultErrorFromErr("Failed to read yaml_path", err)
		}
		if !fileInfo.Mode().IsRegular() {
			return NewToolResultError("yaml_path must point to a regular file")
		}
		if fileInfo.Size() > maxYAMLPathSize {
			return NewToolResultError(fmt.Sprintf("yaml_path exceeds the 64 MiB size limit (%d bytes)", fileInfo.Size()))
		}

		file, err := os.Open(input.YAMLPath)
		if err != nil {
			return NewToolResultErrorFromErr("Failed to read yaml_path", err)
		}
		content, readErr := io.ReadAll(io.LimitReader(file, maxYAMLPathSize+1))
		closeErr := file.Close()
		if readErr != nil {
			return NewToolResultErrorFromErr("Failed to read yaml_path", readErr)
		}
		if closeErr != nil {
			return NewToolResultErrorFromErr("Failed to read yaml_path", closeErr)
		}
		if len(content) > maxYAMLPathSize {
			return NewToolResultError(fmt.Sprintf("yaml_path exceeds the 64 MiB size limit (%d bytes)", len(content)))
		}
		manifest = string(content)
	}

	hasOwnerKind := input.OwnerKind != ""
	hasOwnerName := input.OwnerName != ""
	hasOwnerNamespace := input.OwnerNamespace != ""
	if hasOwnerKind != hasOwnerName || hasOwnerKind != hasOwnerNamespace {
		return NewToolResultError("owner_kind, owner_name and owner_namespace must all be set")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	req := k8s.DiffRequest{
		Manifest:   manifest,
		FluxObject: input.FluxObject,
	}
	if hasOwnerKind {
		req.OwnerRef = &k8s.DiffOwnerRef{
			Kind:      input.OwnerKind,
			Name:      input.OwnerName,
			Namespace: input.OwnerNamespace,
		}
	}

	result, err := kubeClient.Diff(ctx, req)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to diff manifest", err)
	}

	return NewToolResultText(result)
}

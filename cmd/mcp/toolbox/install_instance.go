// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/install"
)

const (
	// ToolInstallFluxInstance is the name of the install_flux_instance tool.
	ToolInstallFluxInstance = "install_flux_instance"
)

// NewInstallFluxInstanceTool creates a new tool for installing Flux Operator and a Flux instance.
func (m *Manager) NewInstallFluxInstanceTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolInstallFluxInstance,
			mcp.WithDescription("This tool installs Flux Operator and Flux instance on the cluster."),
			mcp.WithString("instance_url",
				mcp.Description("The URL pointing to the Flux Instance manifest file."),
				mcp.Required(),
			),
			mcp.WithString("timeout",
				mcp.Description("The installation timeout. Default is 5m."),
			),
		),
		Handler:   m.HandleInstallFluxInstance,
		ReadOnly:  false,
		InCluster: true,
	}
}

// HandleInstallFluxInstance is the handler function for the install_flux_instance tool.
func (m *Manager) HandleInstallFluxInstance(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolInstallFluxInstance, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	now := time.Now()
	instanceURL := mcp.ParseString(request, "instance_url", "")
	if instanceURL == "" {
		return mcp.NewToolResultError("The instance URL cannot be empty"), nil
	}

	timeoutStr := mcp.ParseString(request, "timeout", "5m")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return mcp.NewToolResultError("The timeout is not a valid duration"), nil
	}
	if timeout < 5*time.Minute {
		timeout = 5 * time.Minute
	}
	waitTimeout := timeout - 30*time.Second

	// TODO: stream logs back to the MCP client while the installation is in progress.
	installLog := strings.Builder{}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Step 1: Download the Flux instance manifest and operator manifests

	instance, err := m.fetchInstanceManifest(ctx, instanceURL)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to fetch instance manifest: %w", err), nil
	}

	operatorObjects, err := m.fetchOperatorManifest(ctx, instance)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to fetch operator manifest: %w", err), nil
	}
	installLog.WriteString(fmt.Sprintf("Artifact download completed in %s\n", time.Since(now).Round(time.Second)))

	// Step 2: Create Kubernetes client with impersonation if needed

	cfg, err := m.flags.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	if sess := auth.FromContext(ctx); sess != nil {
		cfg.Impersonate = rest.ImpersonationConfig{
			UserName: sess.UserName,
			Groups:   sess.Groups,
		}
	}

	installer, err := install.NewInstaller(ctx, cfg)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to create installer", err), nil
	}

	// Step 3: Install or upgrade the Flux Operator

	isInstalled, err := installer.IsInstalled(ctx)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed prerequisites", err), nil
	}
	if !isInstalled {
		installLog.WriteString("Installing Flux Operator...\n")
	} else {
		installLog.WriteString("Upgrading Flux Operator...\n")
	}
	multitenant := instance.Spec.Cluster != nil && instance.Spec.Cluster.Multitenant
	cs, err := installer.ApplyOperator(ctx, operatorObjects, multitenant)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to install the operator: %w", err), nil
	}
	installLog.WriteString(cs.String())
	installLog.WriteString("\n")
	if err := installer.WaitFor(ctx, cs, waitTimeout); err != nil {
		return mcp.NewToolResultErrorFromErr("failed to wait for the operator: %w", err), nil
	}
	installLog.WriteString("Flux Operator is ready.\n")

	// Step 4: Install or upgrade the Flux instance

	installLog.WriteString("Installing Flux Instance...\n")
	cs, err = installer.ApplyInstance(ctx, instance)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to install instance: %w", err), nil
	}
	installLog.WriteString(cs.String())
	installLog.WriteString("\n")
	if err := installer.WaitFor(ctx, cs, waitTimeout-30*time.Second); err != nil {
		return mcp.NewToolResultErrorFromErr("failed to wait for the instance: %w", err), nil
	}

	// Step 5: Configure automatic updates

	installLog.WriteString("Configuring automatic updates...\n")
	cs, err = installer.ApplyAutoUpdate(ctx, multitenant)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("failed to configure automatic updates: %w", err), nil
	}
	installLog.WriteString(cs.String())
	installLog.WriteString("\n")
	if err := installer.WaitFor(ctx, cs, waitTimeout-time.Minute); err != nil {
		return mcp.NewToolResultErrorFromErr("failed to wait for automatic updates: %w", err), nil
	}
	installLog.WriteString(fmt.Sprintf("Installation completed in %s\n", time.Since(now).Round(time.Second)))

	return mcp.NewToolResultText(installLog.String()), nil
}

// fetchInstanceManifest downloads and parses the FluxInstance manifest from the given URL.
func (m *Manager) fetchInstanceManifest(ctx context.Context, instanceURL string) (*fluxcdv1.FluxInstance, error) {
	instance := &fluxcdv1.FluxInstance{}
	data, err := install.DownloadManifestFromURL(ctx, instanceURL, authn.DefaultKeychain)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, instance); err != nil {
		return nil, fmt.Errorf("failed to parse Flux instance: %w", err)
	}

	// Set namespace to flux-system
	instance.Namespace = install.DefaultNamespace

	return instance, nil
}

// fetchOperatorManifest downloads and parses the Flux Operator manifest from the distribution artifact.
func (m *Manager) fetchOperatorManifest(ctx context.Context, instance *fluxcdv1.FluxInstance) ([]*unstructured.Unstructured, error) {
	artifactURL := install.DefaultArtifactURL
	if instance.Spec.Distribution.Artifact != "" {
		artifactURL = instance.Spec.Distribution.Artifact
	}

	data, err := install.DownloadFileFromArtifact(
		ctx,
		artifactURL,
		"flux-operator/install.yaml",
		authn.DefaultKeychain,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pull distribution artifact: %w", err)
	}

	objects, err := ssautil.ReadObjects(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("unable to parse flux-operator/install.yaml: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("no Kubernetes objects found in flux-operator/install.yaml")
	}

	return objects, nil
}

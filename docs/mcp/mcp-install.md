---
title: Flux MCP Server Installation
description: FluxCD MCP Server installation guide
---

# Flux MCP Server Installation

This guide walks you through installing, configuring, and using the Flux MCP Server with various AI Agents.

## Prerequisites

Before installing the Flux MCP Server, ensure you have:

- A Kubernetes cluster with Flux Operator installed
- A valid kubeconfig file to access the clusters
- Appropriate permissions to view Flux resources

## Installation Options

### Install with Homebrew

If you are using macOS or Linux, you can install the Flux MCP Server using Homebrew:

```shell
brew install controlplaneio-fluxcd/tap/flux-operator-mcp
```

### Download Pre-built Binaries

The Flux MCP Server is available as a binary executable for Linux, macOS, and Windows.
The `flux-operator-mcp` AMD64 and ARM64 binaries can be downloaded from
GitHub [releases page](https://github.com/controlplaneio-fluxcd/flux-operator/releases).

After downloading the `flux-operator-mcp` archive for your platform and architecture,
unpack it and place the binary in a directory included in your system's `PATH`.

### Build from Source

If you prefer to build from source, clone the repository and build the binary using `make` (requires Go 1.26+):

```shell
git clone https://github.com/controlplaneio-fluxcd/flux-operator.git
cd flux-operator
make mcp-build
```

The `flux-operator-mcp` binary will be available in the `bin` directory relative to the repository root.

## Configuration with AI Agents

The Flux MCP Server is compatible with AI Agents that support the Model Context Protocol (MCP)
using any of the following transport modes:

- Standard Input/Output (`stdio`)
- Stateless Streamable HTTP (`http`)

For Claude Code:

```shell
claude mcp add flux-operator-mcp \
  --env KUBECONFIG=$HOME/.kube/config \
  -- flux-operator-mcp serve
```

For Codex:

```shell
codex mcp add flux-operator-mcp \
  --env KUBECONFIG=$HOME/.kube/config \
  -- flux-operator-mcp serve
```

For OpenCode:

```shell
opencode mcp add flux-operator-mcp \
  --env KUBECONFIG=$HOME/.kube/config \
  -- flux-operator-mcp serve
```

For Cursor, GitHub Copilot, and other agents that support `.mcp.json`:

```json
{
 "mcpServers": {
   "flux-operator-mcp": {
     "command": "flux-operator-mcp",
     "args": ["serve"],
     "env": {
       "KUBECONFIG": "/path/to/.kube/config"
     }
   }
 }
}
```

Replace `/path/to/.kube/config` with the path to your kubeconfig file.
To determine the correct path, you can run: `echo $HOME/.kube/config`.

See the [Configuration Options](mcp-config.md) for more details on how to set up the server
in different modes.

## Testing Your Installation

Test the installation with the following prompts:

- "Which cluster contexts are available in my kubeconfig?"
- "What version of Flux is running in my current cluster?"

If the AI assistant successfully interacts with your cluster and provides relevant information,
your installation is working correctly.

## Troubleshooting

- **Server not found**
    - Verify the path to the binary is correct
    - Ensure the binary has execute permissions
- **AI assistant can't find the tools**
    - Restart the AI assistant application
    - Verify the MCP configuration is correct
    - For VS Code, ensure Agent mode is enabled
- **Kubeconfig not found**
    - Check the path to your kubeconfig
    - Verify the kubeconfig is valid with `kubectl get crds`
- **Permission issues**
    - Ensure your kubeconfig has sufficient permissions 
    - Verify the permissions with `kubectl get fluxinstance -A`

## Upgrading

To upgrade the Flux MCP Server to a newer version:

1. Download the latest binary from the GitHub Releases page
2. Replace your existing binary with the new one
3. Restart any AI assistant applications that use the server

## Uninstallation

To uninstall the Flux MCP Server:

1. Remove the binary from your system
2. Remove the MCP configuration from your AI assistant's settings

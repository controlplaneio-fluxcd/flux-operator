---
title: Flux MCP Server Installation
description: FluxCD MCP Server installation guide
---

# Flux MCP Server Installation

This guide walks you through installing, configuring, and using the Flux MCP Server with various AI assistants.

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

If you prefer to build from source, clone the repository and build the binary using `make` (requires Go 1.24+):

```shell
git clone https://github.com/controlplaneio-fluxcd/flux-operator.git
cd flux-operator
make mcp-build
```

The `flux-operator-mcp` binary will be available in the `bin` directory relative to the repository root.

## Configuration with AI Assistants

The Flux MCP Server is compatible with AI assistants that support the Model Context Protocol (MCP)
using any of the following transport modes:

- Standard Input/Output (`stdio`)
- Server-Sent Events (`sse`)
- Streamable HTTP (`http`)

See the [Configuration Options](mcp-config.md) for more details on how to set up the server
in different modes.

### Claude, Cursor, and Windsurf

Add the following configuration to your AI assistant's settings to enable the Flux MCP Server:

```json
{
 "mcpServers": {
   "flux-operator-mcp": {
     "command": "/path/to/flux-operator-mcp",
     "args": ["serve"],
     "env": {
       "KUBECONFIG": "/path/to/.kube/config"
     }
   }
 }
}
```

Replace `/path/to/flux-operator-mcp` with the actual path to the binary
and `/path/to/.kube/config` with the path to your kubeconfig file.

To determine the correct paths for the binary and kubeconfig, you can use the following commands:

```shell
which flux-operator-mcp
echo $HOME/.kube/config
```

### VS Code Copilot Chat

Add the following configuration to your VS Code settings:

```json
{
 "mcp": {
   "servers": {
     "flux-operator-mcp": {
       "command": "/path/to/flux-operator-mcp",
       "args": ["serve"],
       "env": {
         "KUBECONFIG": "/path/to/.kube/config"
       }
     }
   }
 },
 "chat.mcp.enabled": true
}
```

Replace `/path/to/flux-operator-mcp` with the actual path to the binary
and `/path/to/.kube/config` with the path to your kubeconfig file.

When using GitHub Copilot Chat, enable Agent mode to access the Flux MCP tools.

## Testing Your Installation

Before using the Flux MCP Server, it is important to set up the AI instructions
for your assistant. Copy the rules from the
[instructions.md](https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/mcp/instructions.md)
file and place them into the appropriate settings for your assistant, for more details on how to do this
see the [AI Instructions](mcp-prompting.md#ai-instructions) section.

After the instructions are in place, you can test the installation with the following prompts:

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

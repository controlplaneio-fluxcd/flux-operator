---
title: Flux MCP Server Prompts
description: MCP Server predefined prompts for troubleshooting FluxCD
---

# Flux MCP Server Prompts

The Flux MCP Server comes with a set of predefined prompts that instruct the AI assistant
to perform complex tasks by chaining together multiple MCP [tools](tools.md).

## Debugging Prompts

These prompts are designed to help you quickly identify and resolve issues with your GitOps pipeline.

### debug_flux_kustomization

Troubleshoot a Flux Kustomization and provide root cause analysis for any issues.

**Parameters:**

- `name` (required): The name of the Kustomization
- `namespace` (required): The namespace of the Kustomization
- `cluster` (optional): The cluster context to use

### debug_flux_helmrelease

Troubleshoot a Flux HelmRelease and provide root cause analysis for any issues.

**Parameters:**

- `name` (required): The name of the HelmRelease
- `namespace` (required): The namespace of the HelmRelease
- `cluster` (optional): The cluster context to use

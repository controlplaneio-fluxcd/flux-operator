---
title: Flux MCP Server Prompting Guide
description: FluxCD MCP Server prompt engineering guide
---

# Flux MCP Server Prompting Guide

This guide provides recommendations for configuring your AI assistants with instructions
and offers effective prompting strategies to get the most out of the Flux MCP Server.

## AI Instructions

Providing instructions is crucial for guiding the behavior of your AI assistant
when interacting with the Flux MCP Server. We've created a set of [instructions](instructions.md) (1400 tokens)
that you can use as a starting point.

Copy the rules from the
[instructions.md](https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/mcp/instructions.md)
file and place them into the appropriate settings for your assistant as follows:

- **Claude**: Use the `Project Instructions` section in Claude Desktop
- **Cursor**: Use the `.cursor/rules` dir in your Git repository
- **Windsurf**: Use the `.windsurf/rules` dir in your Git repository
- **GitHub Copilot**: Use the `.github/copilot-instructions.md` file in your Git repository

It is recommended to enhance the instructions with relevant information about your clusters to help the
AI assistant understand your context better. For example, Kubernetes distribution, Cloud provider,
what type of applications are deployed, how secrets are managed.

## Prompting Strategies

For the best experience with the Flux MCP Server [tools](tools.md):

- **Start broad, then narrow**: Begin with general queries about your Flux installation before drilling down
- **Include context**: Mention the namespace, cluster, and relevant details in your requests
- **Chain operations**: For complex workflows, ask the AI to perform a sequence of related operations
- **Verify changes**: After performing modifications, ask for verification of the new state
- **Use documentation**: When in doubt about Flux features, explicitly ask to search the Flux API documentation

## Repository Context

When using an AI chat within your IDE, you can leverage the context of your Git repositories
that contain Kubernetes and Flux resources. This will enable the AI assistant to compare
manifest files with cluster state and provide an accurate analysis.

When using Claude Desktop, you can install the
[filesystem](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem)
MCP server and allow the assistant to access the Kubernetes manifests in your Git repository.

In the Claude project knowledge, add the Flux Operator documentation using the
`https://github.com/controlplaneio-fluxcd/distribution` repository and select
the `docs/operator` folder. This will ensure that the latest Flux Operator API
specifications are available to the model along with guides and examples.

## Example Prompts

Reporting and troubleshooting:

- Analyze the Flux installation in my current cluster and report the status of all components.
- List the clusters in my kubeconfig and compare the Flux instances across them.
- Are there any reconciliation errors in the Flux-managed resources?
- Are the Flux kustomizations and Helm releases configured correctly?
- Based on Flux events, what deployments have been updated today?
- Draw a diagram of the Flux dependency flow in the cluster.
- What is the Git source and revision of the Flux OCI repositories?
- Which Kubernetes deployments are managed by Flux in the current cluster?
- Which images are deployed by Flux in the monitoring namespace?
- Perform a root cause analysis of the last failed deployment in the frontend namespace.

Actions:

- Reconcile the flux-system kustomization with its source in the current cluster.
- Reconcile all the Flux Kustomization from flux-system namespace in the depends-on order, then verify their status.
- Suspend all failing Helm releases in the test namespace, then delete them from the cluster.
- Search for all the suspended Flux Kustomizations in the cluster and resume them.
- Generate a namespace called test and apply it on my current cluster.
- Copy the flux service account and its RBAC from the frontend namespace into test (remove the fluxcd labels).
- Delete the test namespace from my current cluster.

Learning:

- How to configure mutual TLS for Git? Answer using the latest Flux docs.
- What is the role of the interval setting in a Flux Kustomization?  Search the latest docs.
- How to trigger a Flux reconciliation with a webhook? Search the latest docs.

## Predefined Prompts

The Flux MCP Server comes with a set of predefined prompts.
These prompts are designed to help you quickly get started with common tasks such as
troubleshooting Flux Kustomizations and Helm releases.

For a complete list of predefined prompts, refer to the [MCP prompts](prompts.md) documentation.

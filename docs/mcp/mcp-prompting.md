---
title: Flux MCP Server Prompting Guide
description: FluxCD MCP Server prompt engineering guide
---

# Flux MCP Server Prompting Guide

This guide explains how to equip your AI agent with Flux expertise using the
official GitOps Agent Skills and offers example prompts for the Flux MCP Server.

## Agent Skills

The Flux project maintains the official [GitOps Agent Skills](https://github.com/fluxcd/agent-skills)
which give AI agents deep expertise in Flux CD, Kubernetes, and GitOps best practices.
The skills are designed to work together, and the agent automatically selects the right one based on context:

- **gitops-knowledge**: answers questions about Flux CD and generates up-to-date YAML
  for all Flux custom resources, including the Flux Operator APIs.
- **gitops-repo-audit**: audits Flux GitOps repositories for structure, security,
  and operational best practices, and generates a report with prioritized recommendations.
- **gitops-cluster-debug**: debugs and troubleshoots Flux on live Kubernetes clusters using
  the Flux MCP Server. It inspects the Flux installation health, diagnoses HelmRelease,
  Kustomization, and ResourceSet failures, analyzes pod logs, traces dependency chains,
  and produces a root cause analysis report with prioritized remediation steps.

The fastest way to install the skills is with the [Flux Operator CLI](cli.md).
Navigate to your GitOps repository root and run:

```shell
flux-operator skills install ghcr.io/fluxcd/agent-skills --agent claude-code
```

The skills work across compatible agents including Claude Code, Codex, Gemini, and GitHub Copilot.
For the plugin marketplace installation and other methods,
refer to the [agent-skills README](https://github.com/fluxcd/agent-skills#install).

The skills work best in the context of a GitOps repository that contains an `AGENTS.md`
or `CLAUDE.md` file with details about your organization's structure, cluster topology,
Kubernetes distribution, cloud provider, and secret management approach. The agent combines
the skills with the repository context to tailor the analysis and the generated manifests to your setup.

## AI Instructions

The Flux MCP Server advertises a concise set of built-in instructions (under 500 tokens)
to the AI assistant during the MCP initialization handshake.

For AI assistants that don't support agent skills, we've created an extended set of
[instructions](instructions.md) (1400 tokens) with step-by-step troubleshooting
procedures that guide the assistant when interacting with the Flux MCP Server tools.

It is recommended to enhance the instructions with relevant information about your clusters,
such as the Kubernetes distribution, cloud provider, deployed applications, and how secrets are managed.

## Example Prompts

Troubleshooting with the `gitops-cluster-debug` skill:

- Check the Flux installation on my current cluster.
- Debug the failing HelmRelease podinfo in the apps namespace.
- Troubleshoot the Kustomization flux-system/infra-controllers in the staging cluster.

Reporting and analysis:

- Analyze the Flux installation in my current cluster and report the status of all components.
- List the clusters in my kubeconfig and compare the Flux instances across them.
- Are there any reconciliation errors in the Flux-managed resources?
- Are the Flux kustomizations and Helm releases configured correctly?
- Based on Flux events, what deployments have been updated today?
- Draw a diagram of the Flux dependency flow in the cluster.
- What is the Git source and revision of the Flux OCI repositories?
- Which Kubernetes deployments are managed by Flux in the current cluster?
- Which images are deployed by Flux in the monitoring namespace?
- List the Helm releases in the cluster with their chart version constraints and the versions currently deployed.
- Perform a root cause analysis of the last failed deployment in the frontend namespace.
- Preview what changing the staging app overlay would do on the cluster before committing it.

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
- What is the role of the interval setting in a Flux Kustomization? Search the latest docs.
- How to trigger a Flux reconciliation with a webhook? Search the latest docs.

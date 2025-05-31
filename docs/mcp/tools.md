---
title: Flux MCP Server Tools
description: MCP tools for interacting with Kubernetes clusters managed by FluxCD
---

# Flux MCP Server Tools

The Flux Model Context Protocol (MCP) Server provides a comprehensive set of tools
that enable AI assistants to interact with Kubernetes clusters managed by
[Flux Operator](https://github.com/controlplaneio-fluxcd/flux-operator).

## Reporting Tools

These tools gather information from the cluster without making any changes to the system state.

### get_flux_instance

Retrieves detailed information about the Flux installation.

**Parameters:** None

**Output:**

The tool returns comprehensive details about the Flux instance configuration, including
the distribution version information, component status and health, cluster sync statistics.

### get_kubernetes_resources

Retrieves Kubernetes resources from the cluster, including Flux custom resources, their status, and associated events.

**Parameters:**

- `apiVersion` (required): The API version of the resource(s)
- `kind` (required): The kind of the resource(s)
- `name` (optional): The name of a specific resource
- `namespace` (optional): The namespace to query
- `selector` (optional): Label selector in the format `key1=value1,key2=value2`
- `limit` (optional): Maximum number of resources to return

**Output:**

Returns the requested resources in YAML format, including:
- Resource specifications
- Status conditions
- Related events
- Metadata including Flux source references

### get_kubernetes_logs

Retrieves logs from Kubernetes pods, allowing AI assistants to analyze application behavior and troubleshoot issues.

**Parameters:**

- `pod_name` (required): The name of the pod
- `pod_namespace` (required): The namespace of the pod
- `container_name` (required): The name of the container
- `limit` (optional): Maximum number of log lines to return (default: 100)

**Output:**

Returns the specified number of log lines from the requested container, with timestamps and log levels preserved.

### get_kubernetes_api_versions

Retrieves the Kubernetes CRDs registered on the cluster and returns the preferred apiVersion for each kind.

**Parameters:** None

**Output:**

Returns a mapping of Kubernetes resource kinds to their preferred API versions,
which is essential for crafting valid API calls.

## Multi-Cluster Tools

These tools facilitate interaction with multiple Kubernetes clusters, enabling cross-cluster comparisons and operations.

### get_kubeconfig_contexts

Retrieves the available Kubernetes cluster contexts from the kubeconfig.

**Parameters:** None

**Output:**

List of available Kubernetes contexts with their associated cluster name.

### set_kubeconfig_context

Switches the current session to use a specific Kubernetes cluster context, without modifying the kubeconfig file.

**Parameters:**

- `name` (required): The name of the context to set

**Output:**

Confirmation message indicating the context has been switched.

## Reconciliation Tools

These tools trigger reconciliation of Flux resources, causing Flux to synchronize the desired state with the current state.

### reconcile_flux_resourceset

Triggers the reconciliation of a Flux ResourceSet.

**Parameters:**

- `name` (required): The name of the ResourceSet
- `namespace` (required): The namespace of the ResourceSet

**Output:**

Confirmation message and instructions for verifying the reconciliation status.

### reconcile_flux_source

Triggers the reconciliation of Flux sources (GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket).

**Parameters:**

- `kind` (required): The kind of Flux source
- `name` (required): The name of the source
- `namespace` (required): The namespace of the source

**Output:**

Confirmation message and instructions for verifying the reconciliation status.

### reconcile_flux_kustomization

Triggers the reconciliation of a Flux Kustomization.

**Parameters:**

- `name` (required): The name of the Kustomization
- `namespace` (required): The namespace of the Kustomization
- `with_source` (optional): Whether to also reconcile the source (default: false)

**Output:**

Confirmation message and instructions for verifying the reconciliation status.

### reconcile_flux_helmrelease

Triggers the reconciliation of a Flux HelmRelease.

**Parameters:**

- `name` (required): The name of the HelmRelease
- `namespace` (required): The namespace of the HelmRelease
- `with_source` (optional): Whether to also reconcile the source (default: false)

**Output:**

Confirmation message and instructions for verifying the reconciliation status.

## Suspend/Resume Tools

These tools allow for pausing and resuming the reconciliation of Flux resources.

### suspend_flux_reconciliation

Suspends the reconciliation of a Flux resource.

**Parameters:**

- `apiVersion` (required): The API version of the resource
- `kind` (required): The kind of the resource
- `name` (required): The name of the resource
- `namespace` (required): The namespace of the resource

**Output:**

Confirmation message indicating the resource has been suspended.

### resume_flux_reconciliation

Resumes the reconciliation of a previously suspended Flux resource.

**Parameters:**

- `apiVersion` (required): The API version of the resource
- `kind` (required): The kind of the resource
- `name` (required): The name of the resource
- `namespace` (required): The namespace of the resource

**Output:**

Confirmation message indicating the resource has been resumed.

## Apply Tool

This tool allows creating or updating Kubernetes resources in the cluster.
If the resources already exist and are managed by Flux, the tool will error out unless
explicitly told to overwrite them.

### apply_kubernetes_manifest

Applies a YAML manifest on the cluster using Kubernetes server-side apply.

**Parameters:**

- `yaml_content` (required): The multi-doc YAML content
- `overwrite` (optional): Whether to overwrite resources managed by Flux (default: false)

**Output:**

The list of applied resources in the format `kind/namespace/name [created|updated|unchanged]`.

## Deletion Tool

This tool enables the removal of resources from your cluster.

### delete_kubernetes_resource

Deletes a Kubernetes resource from the cluster.

**Parameters:**

- `apiVersion` (required): The API version of the resource
- `kind` (required): The kind of the resource
- `name` (required): The name of the resource
- `namespace` (required for namespaced resources): The namespace of the resource

**Output:**

Confirmation message indicating the resource has been deleted.

## Documentation Tool

This tool provides access to the latest Flux documentation.

### search_flux_docs

Searches the Flux documentation for specific information, ensuring the AI assistant can provide up-to-date guidance.

**Parameters:**

- `query` (required): The search query
- `limit` (optional): Maximum number of results to return (default: 1)

**Output:**

Relevant documentation from the Flux project that matches the search query.

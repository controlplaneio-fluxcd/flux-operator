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

### get_kubernetes_metrics

Retrieves CPU and Memory usage for Kubernetes pods, allowing AI assistants to monitor resource consumption and performance.
This tool depends on the Kubernetes metrics-server being installed in the cluster.

**Parameters:**

- `pod_name` (optional): The name of the pod, when not specified all pods are selected.
- `pod_namespace` (required): The namespace of the pods.
- `pod_selector` (optional): Label selector in the format `key1=value1,key2=value2`
- `limit` (optional): Maximum number of metrics to return (default: 100)

**Output:**

Returns the metrics for the specified pods, including CPU and Memory for each container, in YAML format.

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

## Scopes and the `tools/list` request

**Note:** The feature described in this section is available only with the Streamable HTTP
transport mode and when [authentication](config-api.md#authentication) is configured.

[Scopes](config-api.md#scopes) are a part of the Flux MCP Server authentication and
authorization system. Credentials can have a set of scopes on them to indicate to
the Flux MCP Server which operations are allowed for that credential. For responding
to the `tools/list` request, the server checks the scopes of the credential to
dynamically filter the list of available tools out of those remaining after
considering if the MCP server is running in read-only mode. In other words, the tools
advertised in the `tools/list` request will be those that are not eliminated by the
read-only mode and that are not eliminated by the scopes granted to the credential.

Furthermore, the Flux MCP Server leverages the `_meta` field of the `tools/list`
response (as defined by the MCP specification) to advertise the available scopes
in `_meta.scopes`. Those will be the scopes that can be useful for the tools
available in the server taking the read-only mode into account.

Each scope has the following fields:

- `name`: The scope identifier. Will always have the prefix `toolbox:`.
- `description`: A short human-readable description of what the scope allows.
- `tools`: The list of tools the scope grants access to, discarding any tools that
  are not available due to the read-only mode if enabled.

The advertised scopes for a given instance of the Flux MCP Server can be inspected
by running the following command:

```bash
flux-operator-mcp debug scopes <Flux MCP URL>
```

This command will make a `tools/list` request to the pointed MCP URL and will print
in the JSON format the content of the `_meta.scopes` field returned in the response.

Output when *not* running in read-only mode:

```json
[
  {
    "description": "Allow all read-only toolbox operations.",
    "name": "toolbox:read_only",
    "tools": [
      "get_flux_instance",
      "get_kubernetes_api_versions",
      "get_kubernetes_logs",
      "get_kubernetes_metrics",
      "get_kubernetes_resources",
      "search_flux_docs"
    ]
  },
  {
    "description": "Allow all toolbox operations.",
    "name": "toolbox:read_write",
    "tools": [
      "apply_kubernetes_manifest",
      "delete_kubernetes_resource",
      "get_flux_instance",
      "get_kubernetes_api_versions",
      "get_kubernetes_logs",
      "get_kubernetes_metrics",
      "get_kubernetes_resources",
      "reconcile_flux_helmrelease",
      "reconcile_flux_kustomization",
      "reconcile_flux_resourceset",
      "reconcile_flux_source",
      "resume_flux_reconciliation",
      "search_flux_docs",
      "suspend_flux_reconciliation"
    ]
  },
  {
    "description": "Allow applying Kubernetes manifests.",
    "name": "toolbox:apply_kubernetes_manifest",
    "tools": [
      "apply_kubernetes_manifest"
    ]
  },
  {
    "description": "Allow deleting Kubernetes resources.",
    "name": "toolbox:delete_kubernetes_resource",
    "tools": [
      "delete_kubernetes_resource"
    ]
  },
  {
    "description": "Allow getting a FluxInstance resource.",
    "name": "toolbox:get_flux_instance",
    "tools": [
      "get_flux_instance"
    ]
  },
  {
    "description": "Allow getting available Kubernetes APIs.",
    "name": "toolbox:get_kubernetes_api_versions",
    "tools": [
      "get_kubernetes_api_versions"
    ]
  },
  {
    "description": "Allow getting logs from pods.",
    "name": "toolbox:get_kubernetes_logs",
    "tools": [
      "get_kubernetes_logs"
    ]
  },
  {
    "description": "Allow getting metrics from the Kubernetes API.",
    "name": "toolbox:get_kubernetes_metrics",
    "tools": [
      "get_kubernetes_metrics"
    ]
  },
  {
    "description": "Allow getting Kubernetes resources.",
    "name": "toolbox:get_kubernetes_resources",
    "tools": [
      "get_kubernetes_resources"
    ]
  },
  {
    "description": "Allow reconciling HelmRelease resources.",
    "name": "toolbox:reconcile_flux_helmrelease",
    "tools": [
      "reconcile_flux_helmrelease"
    ]
  },
  {
    "description": "Allow reconciling Kustomization resources.",
    "name": "toolbox:reconcile_flux_kustomization",
    "tools": [
      "reconcile_flux_kustomization"
    ]
  },
  {
    "description": "Allow reconciling ResourceSet resources.",
    "name": "toolbox:reconcile_flux_resourceset",
    "tools": [
      "reconcile_flux_resourceset"
    ]
  },
  {
    "description": "Allow reconciling Flux source resources.",
    "name": "toolbox:reconcile_flux_source",
    "tools": [
      "reconcile_flux_source"
    ]
  },
  {
    "description": "Allow resuming the reconciliation of Flux resources.",
    "name": "toolbox:resume_flux_reconciliation",
    "tools": [
      "resume_flux_reconciliation"
    ]
  },
  {
    "description": "Allow searching the Flux documentation.",
    "name": "toolbox:search_flux_docs",
    "tools": [
      "search_flux_docs"
    ]
  },
  {
    "description": "Allow suspending the reconciliation of Flux resources.",
    "name": "toolbox:suspend_flux_reconciliation",
    "tools": [
      "suspend_flux_reconciliation"
    ]
  }
]
```

Output when running in read-only mode:

```json
[
  {
    "description": "Allow all read-only toolbox operations.",
    "name": "toolbox:read_only",
    "tools": [
      "get_flux_instance",
      "get_kubernetes_api_versions",
      "get_kubernetes_logs",
      "get_kubernetes_metrics",
      "get_kubernetes_resources",
      "search_flux_docs"
    ]
  },
  {
    "description": "Allow getting a FluxInstance resource.",
    "name": "toolbox:get_flux_instance",
    "tools": [
      "get_flux_instance"
    ]
  },
  {
    "description": "Allow getting available Kubernetes APIs.",
    "name": "toolbox:get_kubernetes_api_versions",
    "tools": [
      "get_kubernetes_api_versions"
    ]
  },
  {
    "description": "Allow getting logs from pods.",
    "name": "toolbox:get_kubernetes_logs",
    "tools": [
      "get_kubernetes_logs"
    ]
  },
  {
    "description": "Allow getting metrics from the Kubernetes API.",
    "name": "toolbox:get_kubernetes_metrics",
    "tools": [
      "get_kubernetes_metrics"
    ]
  },
  {
    "description": "Allow getting Kubernetes resources.",
    "name": "toolbox:get_kubernetes_resources",
    "tools": [
      "get_kubernetes_resources"
    ]
  },
  {
    "description": "Allow searching the Flux documentation.",
    "name": "toolbox:search_flux_docs",
    "tools": [
      "search_flux_docs"
    ]
  }
]
```

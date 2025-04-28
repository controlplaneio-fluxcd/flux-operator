# flux-operator-mcp

This in an **experimental** Model Context Protocol Server for interacting with
Kubernetes clusters managed by the [Flux Operator](https://fluxcd.control-plane.io/operator/).

The MCP Server is designed to assist Flux users and ControlPlane's support team
in analyzing and troubleshooting Flux Operator installations and GitOps continuous
delivery pipelines across environments.

Example prompts:

- Analyze the Flux installation in my current cluster and report the status of all components.
- List the clusters in my kubeconfig and compare the Flux instances across then.
- Are there any reconciliation errors in the Flux managed resources?
- Are the Flux kustomizations and Helm releases configured correctly?
- Based on Flux events, what deployments have been updated today?
- Draw a diagram of the Flux dependency flow in the cluster.
- What is the Git source and revision of the Flux OCI repositories?
- Which Kubernetes deployments are managed by Flux in the cluster?
- Which images are deployed by Flux in the monitoring namespace?
- Reconcile all the Flux sources in the dependsOn order, then verify their status.
- Suspend all failing Helm releases in the test namespace, then delete them from the cluster.
- Search for all the suspended Flux resources in the cluster and resume them.

Recommended Claude setup:

- Create a project dedicated to Flux Operator.
- Set the project instructions to "Use the Flux Operator MCP Server
  to analyze and troubleshoot GitOps pipelines on Kubernetes clusters."
- In the project knowledge, add the Flux Operator documentation using the
  `https://github.com/controlplaneio-fluxcd/distribution` repository
  and select the `docs/operator` folder. This will ensure that the latest
  Flux Operator API specifications are available to the model along with guides and examples.

## Build from source

Clone the repository:

```shell
git clone https://github.com/controlplaneio-fluxcd/flux-operator.git
cd flux-operator
```

Build the MCP server binary (Go 1.24+ is required):

```shell
make mcp-build
```

## Installation

The MCP server is a standalone binary available as a binary executable for Linux, macOS and Windows.
The AMD64 and ARM64 binaries can be downloaded from
GitHub [releases page](https://github.com/controlplaneio-fluxcd/flux-operator/releases).

### Usage with Claude Desktop

Add the binary to the Claude Desktop configuration (change the paths to your username):

```json
{
    "mcpServers": {
      "flux-operator-mcp": {
          "command": "/Users/username/src/flux-operator/bin/flux-operator-mcp",
          "args": ["serve"],
          "env": {
            "KUBECONFIG": "/Users/username/.kube/config"
          }
        }
      }
}
```

Note that on macOS the config file is located at `~/Library/Application Support/Claude/claude_desktop_config.json`.

### Usage with VS Code

Add the following configuration to the vscode `settings.json` file:

```json
{
  "mcp": {
    "servers": {
      "flux-operator-mcp": {
        "command": "/Users/username/src/flux-operator/bin/flux-operator-mcp",
        "args": ["serve"],
        "env": {
          "KUBECONFIG": "/Users/username/.kube/config"
        }
      }
    }
  },
  "chat.mcp.enabled": true
}
```

Note that you need to toggle Agent mode in the Copilot Chat to use the Flux Operator MCP tools.

## MCP Prompts and Tools

### Predefined prompts

The MCP server provides a set of predefined prompts that can be used to troubleshoot Flux:

- `debug_flux_kustomization`: This prompt instructs the model to troubleshoot a Flux Kustomization and provide root cause analysis.
  - `name` - The name of the Kustomization (required).
  - `namespace` - The namespace of the Kustomization (required).
  - `cluster` - The name of the cluster (optional).
- `debug_flux_helmrelease`: This prompt instructs the model to troubleshoot a Flux HelmRelease and provide root cause analysis.
  - `name` - The name of the HelmRelease (required).
  - `namespace` - The namespace of the HelmRelease (required).
  - `cluster` - The name of the cluster (optional).

### Reporting tools

The MCP server provides a set of tools for generating reports about the state of the cluster:

- `get_flux_instance_report`: This tool retrieves the Flux instance and a detailed report about Flux controllers and their status.
  - `name` - The name of the Flux instance (optional).
  - `namespace` - The namespace of the Flux instance (optional).
- `get_kubernetes_resources`: This tool retrieves Kubernetes resources including Flux own resources, their status, and events.
  - `apiVersion`: The API version of the resource(s) (required).
  - `kind`: The kind of the resource(s) (required).
  - `name`: The name of the resource (optional).
  - `namespace`: The namespace of the resource(s) (optional).
  - `selector`: The label selector in the format `key1=value1,key2=value2` (optional).
  - `limit`: The maximum number of resources to return (optional).
- `get_kubernetes_api-versions`: This tool retrieves the CRDs registered on the cluster and returns the preferred apiVersion for each kind.
  - No arguments required

The output of the reporting tools is formatted as a multi-doc YAML.

### Reconciliation tools

The MCP server provides a set of tools for triggering the reconciliation of Flux resources:

- `reconcile_flux_resourceset`: This tool triggers the reconciliation of the Flux ResourceSet.
  - `name` - The name of the resource to reconcile (required).
  - `namespace` - The namespace of the resource to reconcile (required).
- `reconcile_flux_source`: This tool triggers the reconciliation of the Flux sources (GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket).
  - `name` - The name of the resource to reconcile (required).
  - `namespace` - The namespace of the resource to reconcile (required).
- `reconcile_flux_kustomization`: This tool triggers the reconciliation of the Flux Kustomization including its source (GitRepository, OCIRepository, Bucket).
  - `name` - The name of the resource to reconcile (required).
  - `namespace` - The namespace of the resource to reconcile (required).
  - `with_source` - Trigger the reconciliation of the Flux Kustomization source (optional).
- `reconcile_flux_helmrelease`: This tool triggers the reconciliation of the Flux HelmRelease including its source (OCIRepository, HelmChart).
  - `name` - The name of the resource to reconcile (required).
  - `namespace` - The namespace of the resource to reconcile (required).
  - `with_source` - Trigger the reconciliation of the Flux HelmRelease source (optional).

The output of the reconciliation tools tells the model how to verify the status of the reconciled resource.

### Suspend / Resume tools

The MCP server provides a set of tools for suspending and resuming the reconciliation of Flux resources:

- `suspend_flux_resource`: This tool suspends the reconciliation of a Flux resource (Kustomization, HelmRelease, ResourceSet, OCIRepository, etc.).
  - `apiVersion` - The API version of the resource (required).
  - `kind` - The kind of the resource (required).
  - `name` - The name of the resource (required).
  - `namespace` - The namespace of the resource (required).
- `resume_flux_resource`: This tool resumes the reconciliation of a Flux resource.
  - `apiVersion` - The API version of the resource (required).
  - `kind` - The kind of the resource (required).
  - `name` - The name of the resource (required).
  - `namespace` - The namespace of the resource (required).

### Deletion tool

The MCP server provides a tool for deleting Kubernetes resources:

- `delete_kubernetes_resource`: This tool triggers the deletion of a Kubernetes resource.
  - `apiVersion` - The API version of the resource (required).
  - `kind` - The kind of the resource (required).
  - `name` - The name of the resource (required).
  - `namespace` - The namespace of the resource (required for namespaced resources).

### Multi-cluster tools

The MCP server provides a set of tools for multi-cluster operations:

- `get_kubeconfig_contexts`: This tool retrieves the Kubernetes clusters contexts found in the kubeconfig.
  - No arguments required
- `set_kubeconfig_context`: This tool sets the context to a specific cluster for the current session.
  - `name` - The name of the context to set (required).

Note that the `set_kubeconfig_context` tool does not alter the kubeconfig file,
it only sets the context for the current session. 

## Security Considerations

The data returned by the MCP server may contain sensitive information found in Flux resources,
such as container images, Git repository URLs, and Helm chart names.
By default, the MCP Server masks the values of the Kubernetes Secrets data values,
but it is possible to disable this feature by setting the `--mask-secrets=false` flag.

The MCP server exposes tools that alter the state of the cluster, such as suspending,
resuming, and deleting Flux resources. To disable these tools, the MCP server can be
configured to run in read-only mode by setting the `--read-only` flag.

The MCP server uses the `KUBECONFIG` environment variable to read the configuration and
authenticate to Kubernetes clusters. It is possible to specify a user or service account
that the MCP server will impersonate when connecting to the cluster.

Example configuration for impersonating a service account with read-only permissions:

```json
{
    "mcpServers": {
      "flux-operator-mcp": {
          "command": "/Users/stefanprodan/src/flux-operator/bin/flux-operator-mcp",
          "args": [
            "serve",
            "--read-only",
            "--kube-as=system:serviceaccount:flux-system:flux-operator"
          ],
          "env": {
            "KUBECONFIG": "/Users/stefanprodan/.kube/config"
          }
        }
      }
}
```

## License

The Flux Operator MCP Server is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

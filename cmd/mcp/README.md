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
- How to configure mutual TLS for Git? Answer using the Flux docs search tool.

Recommended Claude setup:

- Create a project dedicated to Flux Operator.
- Set the project instructions to the content from [ai-instructions.md](ai-instructions.md).
- In the project knowledge, add the Flux Operator documentation using the
  `https://github.com/controlplaneio-fluxcd/distribution` repository
  and select the `docs/operator` folder. This will ensure that the latest
  Flux Operator API specifications are available to the model along with guides and examples.

Recommended VS Code setup:

- Use the Git repository containing the Flux manifests as the workspace. This will 
  allow Copilot to compare the current state of the cluster with the desired state in Git.
- Create a `.github/copilot-instructions.md` file with the content from [ai-instructions.md](ai-instructions.md).
- Start a Copilot chat session by asking a question about the Flux instance e.g. 
  `What is the status of the Flux instance on my current cluster?`. This will ensure that
  Copilot has access to the latest information about the cluster and Flux API versions.
- When asking questions about the Flux API, it is recommended to append the following sentence
  to the prompt: `Answer using the Flux docs search tool.`. This will ensure that Copilot
  uses the latest information from the Flux documentation.

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

The MCP server is available as a binary executable for Linux, macOS, and Windows.
The `flux-operator-mcp` AMD64 and ARM64 binaries can be downloaded from
GitHub [releases page](https://github.com/controlplaneio-fluxcd/flux-operator/releases).

By default, the `flux-operator-mcp serve` command starts the server using the
Standard Input/Output (stdio) transport which is compatible with Claude Desktop,
Cursor, Windsurf, VS Code Copilot Chat, and other AI tools.

To use Server-Sent Events (SSE), start the server with:

```shell
flux-operator-mcp serve --transport sse --port 8080
```

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

Note that on macOS the Claude config file is located at
`~/Library/Application Support/Claude/claude_desktop_config.json`.

The same configuration can be used with Cursor and Windsurf.

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

- `get_flux_instance`: This tool retrieves the Flux instance and a detailed report about Flux controllers and their status.
  - `name` - The name of the Flux instance (optional).
  - `namespace` - The namespace of the Flux instance (optional).
- `get_kubernetes_resources`: This tool retrieves Kubernetes resources including Flux own resources, their status, and events.
  - `apiVersion`: The API version of the resource(s) (required).
  - `kind`: The kind of the resource(s) (required).
  - `name`: The name of the resource (optional).
  - `namespace`: The namespace of the resource(s) (optional).
  - `selector`: The label selector in the format `key1=value1,key2=value2` (optional).
  - `limit`: The maximum number of resources to return (optional).
- `get_kubernetes_logs` : This tool retrieves the most recent logs of a Kubernetes pod.
  - `pod_name` - The name of the pod (required).
  - `pod_namespace` - The namespace of the pod (required).
  - `containe_name` - The name of the container (required).
  - `limit` - The maximum number of lines to return (default 100).
- `get_kubernetes_api_versions`: This tool retrieves the CRDs registered on the cluster and returns the preferred apiVersion for each kind.
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

- `suspend_flux_reconciliation`: This tool suspends the reconciliation of a Flux resource (Kustomization, HelmRelease, ResourceSet, OCIRepository, etc.).
  - `apiVersion` - The API version of the resource (required).
  - `kind` - The kind of the resource (required).
  - `name` - The name of the resource (required).
  - `namespace` - The namespace of the resource (required).
- `resume_flux_reconciliation`: This tool resumes the reconciliation of a Flux resource.
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

### Documentation tool

The MCP server provides a tool for searching the Flux documentation:

- `search_flux_docs`: This tool searches the latest Flux documentation and returns up-to-date information.
  - `query` - The search query (required).
  - `limit` - The maximum number of results to return (default 1).

Note that most AI models are trained on data up to a certain date and may not have the latest information.
When asking questions about the Flux APIs or features, it is recommended to append the following sentence
to the prompt: `Answer using the Flux docs search tool.`.

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

On the local machine file system, the MCP server performs only read operations.
It reads the `KUBECONFIG` environment variable then reads the config file.
If the config file contains `exec` instructions to authenticate to the cluster,
the MCP server will execute the command in the same way as `kubectl` does.
Note that running the MCP server in a container is possible
only if the config contains static credentials.

## License

The Flux Operator MCP Server is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

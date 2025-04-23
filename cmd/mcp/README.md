# flux-operator-mcp

This in an **experimental** Model Context Protocol Server for interacting with
Kubernetes clusters managed by the [Flux Operator](https://fluxcd.control-plane.io/operator/).

The MCP server primarily goal is helping Flux users and ControlPlane's support team to analyze and
troubleshoot [Flux Enterprise](https://fluxcd.control-plane.io/distribution/) installations.

Example prompts:

- Analyze the Flux installation in my cluster and report the status of all components.
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
  to analyse and troubleshoot GitOps pipelines on Kubernetes clusters."
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

Add the binary to the Claude Desktop configuration (change the paths to your user):

```json
{
    "mcpServers": {
      "flux-operator-mcp": {
          "command": "/Users/stefanprodan/src/flux-operator/bin/flux-operator-mcp",
          "args": ["serve"],
          "env": {
            "KUBECONFIG": "/Users/stefanprodan/.kube/config"
          }
        }
      }
}
```

Note that on macOS the config file is located at `~/Library/Application Support/Claude/claude_desktop_config.json`.

## MCP Tools

### Reporting tools

The MCP server provides a set of tools for generating reports about the state of the cluster:

- `get-flux-instance`: This tool retrieves the Flux instance and a detailed report about Flux controllers and their status.
- `get-flux-resourceset`: This tool retrieves the Flux ResourceSets and ResourceSetInputProviders including their status and events.
- `get-flux-source`: This tool retrieves the Flux sources (GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket) including their status and events.
- `get-flux-kustomization`: This tool retrieves the Flux Kustomizations including their status and events.
- `get-flux-helmrelease`: This tool retrieves the Flux HelmReleases including their status and events.
- `get-kubernetes-resource`: This tool retrieves the Kubernetes resources managed by Flux.

All the reporting tools allow filtering the output by:

- `apiVersion`: The API version of the resource (required for `get-kubernetes-resource`).
- `kind`: The kind of the resource (required for `get-kubernetes-resource`).
- `name`: The name of the resource (optional).
- `namespace`: The namespace of the resource (optional).
- `labelSelector`: The label selector in the format `label-key=label-value` (optional).

The output of the reporting tools is formatted as a multi-doc YAML.

### Reconciliation tools

The MCP server provides a set of tools for triggering the reconciliation of Flux resources:

- `reconcile-flux-resourceset`: This tool triggers the reconciliation of the Flux ResourceSet.
- `reconcile-flux-source`: This tool triggers the reconciliation of the Flux sources (GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket).
- `reconcile-flux-kustomization`: This tool triggers the reconciliation of the Flux Kustomization including its source (GitRepository, OCIRepository, Bucket).
- `reconcile-flux-helmrelease`: This tool triggers the reconciliation of the Flux HelmRelease including its source (OCIRepository, HelmChart).

The reconciliation tools accept the following arguments:

- `name` - The name of the resource to reconcile (required).
- `namespace` - The namespace of the resource to reconcile (required).
- `withSource` - Trigger the reconciliation of the Flux Kustomization or HelmRelease source (optional).

The output of the reconciliation tools tells the model how to verify the status of the reconciled resource.

### Suspend / Resume tools

The MCP server provides a set of tools for suspending and resuming the reconciliation of Flux resources:

- `suspend-flux-resource`: This tool suspends the reconciliation of a Flux resource (Kustomization, HelmRelease, ResourceSet, OCIRepository, etc.).
- `resume-flux-resource`: This tool resumes the reconciliation of a Flux resource.

The suspend and resume tools accept the following arguments:

- `apiVersion` - The API version of the resource (required).
- `kind` - The kind of the resource (required).
- `name` - The name of the resource (required).
- `namespace` - The namespace of the resource (required).

### Deletion tool

The MCP server provides a tool for deleting Kubernetes resources:

- `delete-kubernetes-resource`: This tool triggers the deletion of a Kubernetes resource.

The deletion tool accept the following arguments:

- `apiVersion` - The API version of the resource (required).
- `kind` - The kind of the resource (required).
- `name` - The name of the resource (required).
- `namespace` - The namespace of the resource (required for namespaced resources).

## Security Considerations

The data returned by the MCP server may contain sensitive information found in Flux resources,
such as container images, Git repository URLs, and Helm chart names.
By default, the MCP Server masks the values of the Kubernetes Secrets data values,
but it is possible to disable this feature by setting the `--mask-secrets=false` flag.

The MCP server exposes tools that alter the state of the cluster, such as suspending,
resuming, and deleting Flux resources. To disable these tools, the MCP server can be
configured to run in read-only mode by setting the `--read-only` flag.

The MCP server uses the `KUBECONFIG` environment variable to read the configuration and
authenticate to the cluster. The server will use the default context set the kubeconfig file.
It is possible to specify a different context and a user or service account that the MCP server
will impersonate when connecting to the cluster.

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

The Flux Operator is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

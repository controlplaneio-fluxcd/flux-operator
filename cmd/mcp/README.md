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
- Create a report of all Flux resources and their status in the monitoring namespace.
- Draw a diagram of the Flux dependency flow in the cluster.
- Which Kubernetes deployments are managed by Flux in the cluster?
- Which images are deployed by Flux in the monitoring namespace?
- Reconcile the podinfo Helm release in the frontend namespace.
- Reconcile all the Flux sources in the dependsOn order, then verify their status.

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

## Security Considerations

The data returned by the MCP server may contain sensitive information found in Flux resources,
such as container images, Git repository URLs, and Helm chart names.
By default, the MCP Server masks the values of the Kubernetes Secrets data values,
but it is possible to disable this feature by setting the `--mask-secrets=false` flag.

The MCP server exposes tools that alter the state of the cluster, such as
deleting Kubernetes resources. To disable these tools, the MCP server can be
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

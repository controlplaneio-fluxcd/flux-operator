# flux-operator-mcp

This in an **experimental** Model Context Protocol Server for interacting with
Kubernetes clusters managed by the [Flux Operator](https://fluxcd.control-plane.io/operator/).

The MCP server primarily goal is helping Flux users and ControlPlane's support team to analyze and
troubleshoot [Flux Enterprise](https://fluxcd.control-plane.io/distribution/) installations.

Example prompts:

- Analyze the Flux installation in my cluster and report the status of all components.
- Are there any reconciliation errors in the Flux managed resources?
- Are the Flux kustomizations and Helm releases configured correctly?
- Create a report of all Flux resources in the cluster and their status.
- Draw a diagram of the Flux dependency flow in the cluster.
- Which Kubernetes deployments are managed by Flux in the cluster?
- Which images are deployed by Flux in the monitoring namespace?
- Reconcile the Flux infra-components kustomization in the monitoring namespace.
- Reconcile the Flux podinfo Helm release in the frontend namespace.
- Reconcile all the Flux sources in the cluster, then verify their status.

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

The MCP server is designed to prevent alterations to the cluster state, besides triggering
the reconciliation of Flux resources no other write operation is exposed to the client.

The data returned by the server is limited to the Flux resources spec and their status which
may contain sensitive information such as container images, Git repository URLs, and Helm chart names.
The server does not expose sensitive information stored in Kubernetes secrets,
unless Flux substitutions are used to set inline values in HelmRelease and Kustomization resources.

The MCP server uses the `KUBECONFIG` environment variable to read the configuration and 
authenticate to the cluster. The server will use the default context set the kubeconfig file.
It is possible to specify a different context and a user or service account that the MCP server
will impersonate when connecting to the cluster.

Example configuration for impersonating a service account:

```json
{
    "mcpServers": {
      "flux-operator-mcp": {
          "command": "/Users/stefanprodan/src/flux-operator/bin/flux-operator-mcp",
          "args": [
            "serve",
            "--kube-context=kind-kind",
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

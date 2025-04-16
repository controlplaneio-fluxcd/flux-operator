# flux-operator-mcp

This in an **experimental** Model Context Protocol Server for interacting with
Kubernetes clusters managed by the [Flux Operator](https://fluxcd.control-plane.io/operator/).

The MCP server primarily goal is helping ControlPlane's support team to analyze and
troubleshoot [Flux Enterprise](https://fluxcd.control-plane.io/distribution/) installations.

Example prompts:

- Analyze the Flux installation in my cluster and report the status of all components.
- Are there any reconciliation errors in the Flux managed resources?
- Are the Flux kustomizations and Helm releases configured correctly?
- Create a report of all Flux resources in the cluster and their status.
- Draw a diagram of the Flux dependency flow in the cluster.
- Which Kubernetes deployments are managed by Flux in the cluster?
- Reconcile the Flux infra-components kustomization in the monitoring namespace.

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

## License

The Flux Operator is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

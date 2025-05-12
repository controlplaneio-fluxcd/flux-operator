# Flux MCP Server

The Flux MCP Server connects AI assistants directly to your Kubernetes clusters running Flux Operator,
enabling GitOps analysis and troubleshooting through natural language.

Using AI assistants with the Flux MCP Server, you can:

- Debug GitOps pipelines end-to-end from Flux resources to application logs
- Get intelligent root cause analysis for failed deployments
- Compare Flux configurations and Kubernetes resources between clusters
- Visualize Flux dependencies with diagrams generated from the cluster state
- Instruct Flux to perform operations using conversational prompts

## Quickstart

Install the Flux MCP Server using Homebrew:

```shell
brew install controlplaneio-fluxcd/tap/flux-operator-mcp
```

For other installation options, refer to the [installation guide](https://fluxcd.control-plane.io/mcp/install/).

Add the following configuration to your AI assistant's MCP settings:

```json
{
  "flux-operator-mcp":{
    "command":"flux-operator-mcp",
    "args":[
      "serve",
      "--read-only=false"
    ],
    "env":{
      "KUBECONFIG":"/path/to/.kube/config"
    }
  }
}
```

Replace `/path/to/.kube/config` with the absolute path to your kubeconfig file,
you can find it with: `echo $HOME/.kube/config`.

Copy the AI rules from
[instructions.md](https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/mcp/instructions.md)
and place them into the appropriate file for your assistant.

Restart the AI assistant app and test the MCP Server with the following prompts:

- "Which cluster contexts are available in my kubeconfig?"
- "What version of Flux is running in my current cluster?"

For more information on how to use the MCP Server with Claude, Cursor, GitHub Copilot,
and other assistants, please refer to the [documentation website](https://fluxcd.control-plane.io/mcp/).

## Documentation

- [Flux MCP Server Overview](https://fluxcd.control-plane.io/mcp/)
- [Installation Guide](https://fluxcd.control-plane.io/mcp/install/)
- [Transport Modes and Security Configurations](https://fluxcd.control-plane.io/mcp/config/)
- [Effective Prompting Guide](https://fluxcd.control-plane.io/mcp/prompt-engineering/)
- [MCP Tools Reference](https://fluxcd.control-plane.io/mcp/tools/)
- [MCP Prompts Reference](https://fluxcd.control-plane.io/mcp/prompts/)

## License

The MCP Server is open-source and part of the [Flux Operator](https://github.com/controlplaneio-fluxcd/flux-operator)
project licensed under the [AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

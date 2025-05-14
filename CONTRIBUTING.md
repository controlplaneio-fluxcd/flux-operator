# Contributing

Flux Operator is [AGPL-3.0 licensed](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
and accepts contributions via GitHub pull requests. This document outlines
some of the conventions on how to make it easier to get your contribution accepted.

We gratefully welcome improvements to code and documentation!

## Certificate of Origin

By contributing to this project, you agree to the Developer Certificate of
Origin ([DCO](https://developercertificate.org/)). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the contribution.

## Project Code Structure

The project is structured as a [Go module](https://go.dev/doc/modules/developing) with the following components:

### Flux Operator Kubernetes Controller

The Flux Operator is built using the Kubernetes [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
framework and the [Flux runtime SDK](https://pkg.go.dev/github.com/fluxcd/pkg/runtime).

Packages:
- `api/` - contains the API definitions for the Flux Operator Kubernetes CRDs
- `cmd/operator/` - contains the entrypoint of the Kubernetes controller
- `internal/controller` - contains the controller-runtime reconcilers and watchers

To test and build the operator binary, run:

```shell
make test build
```

The build command writes the binary at `bin/flux-operator` relative to the repository root.

To run the controller locally, first create a Kubernetes cluster using `kind`:

```shell
kind create cluster
```

To build the container image and deploy the operator to the kind cluster, run:

```shell
IMG=flux-operator:dev make build docker-build load-image deploy
```

To run the integration tests against the kind cluster, run:

```shell
make test-e2e
```

### Flux Operator CLI

The Flux Operator command-line interface (CLI) is built using the [Cobra](https://github.com/spf13/cobra) library.

Packages:
- `cmd/cli/` - contains the commands for the Flux Operator CLI

To test and build the CLI binary, run:

```shell
make test cli-build
```

The build command writes the binary at `bin/flux-operator-cli` relative to the repository root.

To run the CLI locally, use the binary built in the previous step:

```shell
./bin/flux-operator-cli --help
```

### Flux Operator MCP Server

The Flux Operator MCP Server is built using the [mcp-go](https://github.com/mark3labs/mcp-go) library.

Packages:
- `cmd/mcp/k8s` - contains the Kubernetes client
- `cmd/mcp/prompter` - contains the MCP prompts
- `cmd/mcp/toolbox` - contains the MCP tools
- `cmd/mcp/main.go` - contains the server entrypoint

To test and build the MCP Server binary, run:

```shell
make test mcp-build
```

The build command writes the binary at `bin/flux-operator-mcp` relative to the repository root.

To run the MCP Server using stdio, add the following configuration to your AI assistant's settings:

```json
{
  "mcpServers": {
    "flux-operator-mcp": {
      "command": "/path/to/bin/flux-operator-mcp",
      "args": ["serve"],
      "env": {
        "KUBECONFIG": "/path/to/.kube/config"
      }
    }
  }
}
```

Replace `/path/to/bin/flux-operator-mcp` with the absolute path to the binary
and `/path/to/.kube/config` with the absolute path to your kubeconfig file.

After rebuilding the MCP Server binary, you need to restart the AI assistant app to test the new build.

To run the MCP Server using SSE, use the following command:

```shell
export KUBECONFIG=$HOME/.kube/config
./bin/flux-operator-mcp serve --transport sse --port 8080
```

To connect to the server from VS Code, use the following configuration:

```json
{
  "mcp": {
    "servers": {
      "flux-operator-mcp": {
        "type": "sse",
        "url": "http://localhost:8080/sse"
      }
    }
  }
}
```

After rebuilding the MCP Server binary, you need to restart the server to test the new build.

## Project Documentation Structure

The project documentation is written in Markdown.
To contribute to the documentation, you can edit the Markdown files in the following locations:

### API documentation (flux-operator repository)

- [docs/api](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/docs/api/v1) - contains the API documentation for the Flux Operator Kubernetes CRDs
- [docs/mcp](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/docs/mcp) - contains the documentation for the MCP Server tools and prompts

### Website (distribution repository)

- [docs/operator](https://github.com/controlplaneio-fluxcd/distribution/tree/main/docs/operator) - contains the user guides for the Flux Operator
- [docs/mcp](https://github.com/controlplaneio-fluxcd/distribution/tree/main/docs/mcp) - contains the user guides for the MCP Server

## Acceptance policy

These things will make a PR more likely to be accepted:

- a well-described requirement
- tests for new code
- tests for old code!
- new code and tests follow the conventions in old code and tests
- a good commit message (see below)
- all code must abide [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- names should abide [What's in a name](https://talks.golang.org/2014/names.slide#1)
- code must build on Linux, macOS and Windows via plain `go build`
- code should have appropriate test coverage, and tests should be written
  to work with `go test`

Before opening a PR, please check that your code passes the following:

```shell
make test lint
```

In general, we will merge a PR once one maintainer has endorsed it.
For significant changes, more people may become involved, and you might
get asked to resubmit the PR or divide the changes into more than one PR.

### Format of the Commit Message

For this project we prefer the following rules for good commit messages:

- Limit the subject to 50 characters and write as the continuation
  of the sentence "If applied, this commit will ..."
- Explain what and why in the body, if more than a trivial change;
  wrap it at 72 characters.

The [following article](https://chris.beams.io/posts/git-commit/#seven-rules)
has some more helpful advice on documenting your work.

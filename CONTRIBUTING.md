# Contributing

Flux Operator is [AGPL-3.0 licensed](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
and accepts contributions via GitHub pull requests. This document outlines
some of the conventions on how to make it easier to get your contribution accepted.

We gratefully welcome improvements to code and documentation!

## Certificate of Origin

By contributing to this project, you agree to the Developer Certificate of
Origin ([DCO](https://developercertificate.org/)). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the contribution.

> You must sign-off your commits with your name and email address using `git commit -s`.

## Project Overview

The project is structured as a [Go module](https://go.dev/doc/modules/developing) with the following components:

- [Flux Operator Kubernetes Controller](#flux-operator-kubernetes-controller)
- [Flux Operator CLI](#flux-operator-cli)
- [Flux Operator MCP Server](#flux-operator-mcp-server)
- [Flux Status Web UI](#flux-status-web-ui)

The documentation is structured as follows:

- [API documentation](#api-documentation-flux-operator-repository)
- [User Guides](#website-distribution-repository)

## Source Code Structure

### Flux Operator Kubernetes Controller

The Flux Operator is built using the Kubernetes [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
framework and the [Flux runtime SDK](https://pkg.go.dev/github.com/fluxcd/pkg/runtime).

The test framework is based on controller-runtime's [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest)
and [Gomega](https://pkg.go.dev/github.com/onsi/gomega).

Packages:

- [api](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/api/) - contains the API definitions for the Flux Operator Kubernetes CRDs
- [cmd/operator](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/operator/) - contains the entrypoint of the Kubernetes controller
- [internal/controller](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/controller/) - contains the reconcilers for `FluxInstance`, `FluxReport`, `ResourceSet`, and `ResourceSetInputProvider` resources
- [internal/builder](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/builder/) - contains utilities for building and templating Flux manifests
- [internal/entitlement](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/entitlement/) - contains metering and entitlement validation logic
- [internal/gitprovider](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/gitprovider/) - contains integrations for GitHub, GitLab, and Azure DevOps
- [internal/inventory](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/inventory/) - contains inventory management for tracking applied resources
- [internal/reporter](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/reporter/) - contains cluster reporting and metrics collection utilities
- [internal/schedule](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/schedule/) - contains scheduling utilities for automated operations

To test and build the operator binary, run:

```shell
make test build
```

The build command writes the binary at `bin/flux-operator` relative to the repository root.

To run the controller locally, first create a Kubernetes cluster using `kind`:

```shell
kind create cluster
```

To run the integration tests against the kind cluster, run:

```shell
make test-e2e
```

For more information on how to run the controller locally and perform manual testing, refer to the
[development](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/docs/dev#local-development) guide.

### Flux Operator CLI

The Flux Operator command-line interface (CLI) is built using the [Cobra](https://github.com/spf13/cobra) library.

Packages:

- [cmd/cli](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/cli/) - contains the commands for the Flux Operator CLI

To test and build the CLI binary, run:

```shell
make cli-test cli-build
```

The build command writes the binary at `bin/flux-operator-cli` relative to the repository root.

To run the CLI locally, use the binary built in the previous step:

```shell
./bin/flux-operator-cli --help
```

The CLI commands documentation is maintained in the
[cmd/cli/README.md](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/cli/README.md) file.

### Flux Operator MCP Server

The Flux Operator MCP Server is built using the [mcp-go](https://github.com/mark3labs/mcp-go) library.

Packages:

- [cmd/mcp/k8s](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/mcp/k8s/) - contains the Kubernetes client
- [cmd/mcp/prompter](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/mcp/prompter/) - contains the MCP prompts
- [cmd/mcp/toolbox](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/mcp/toolbox/) - contains the MCP tools
- [cmd/mcp/main.go](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/cmd/mcp/main.go) - contains the server entrypoint

To test and build the MCP Server binary, run:

```shell
make mcp-test mcp-build
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

### Flux Status Page

The Flux Status Web UI is a single-page application (SPA) built using [Preact](https://preactjs.com/),
[Tailwind CSS](https://tailwindcss.com/), and [Vite](https://vite.dev/).

The test framework is based on [Vitest](https://vitest.dev/) with [jsdom](https://github.com/jsdom/jsdom)
for DOM simulation and [@testing-library/preact](https://testing-library.com/docs/preact-testing-library/intro/)
for component testing.

Packages:

- [web/src](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/web/src/) - contains the Preact components and utilities
- [web/src/components](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/web/src/components/) - contains the UI components
- [web/src/utils](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/web/src/utils/) - contains utility functions for theming, time formatting, etc.
- [web/src/mock](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/web/src/mock/) - contains mock data for development
- [web/dist](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/web/dist/) - build output embedded in the Go binary via `web/embed.go`
- [internal/web](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/internal/web/) - contains the Go HTTP server, API routes, and embedded frontend serving

To test and build the web UI, run:

```shell
make web-test web-build
```

The build command writes the production assets to the `web/dist/` directory, which is embedded
into the Go binary and served by the status web server.

To run the web UI locally with mock data:

```shell
make web-dev-mock
```

This starts a Vite dev server with hot module replacement at `http://localhost:5173`.

To run the web UI with a live backend connected to Kubernetes:

```shell
# Terminal 1: Start the status web server
make web-run

# Terminal 2: Start the Vite dev server
make web-dev
```

The Vite dev server will proxy API requests to the Go backend running on port 35000.

To run the web server as cluster-admin (for testing user actions that require elevated permissions),
first create a `web-config.yaml` file with the following content:

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: http://localhost:9080
  userActions:
    authType: Anonymous
  authentication:
    type: Anonymous
    anonymous:
      username: cluster-admin
      groups:
        - system:masters
```

Then run the web server with the config:

```shell
make web-run WEB_RUN_ARGS=--web-config=web-config.yaml
```

## Project Documentation Structure

The project documentation is written in Markdown.
To contribute to the documentation, you can edit the Markdown files in the following locations:

### API documentation (flux-operator repository)

- [docs/api](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/docs/api/v1) - contains the API documentation for the Flux Operator Kubernetes CRDs
- [docs/mcp](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/docs/mcp) - contains the documentation for the MCP Server tools and prompts

### Website (distribution repository)

- [docs/operator](https://github.com/controlplaneio-fluxcd/distribution/tree/main/docs/operator) - contains the user guides for the Flux Operator
- [docs/mcp](https://github.com/controlplaneio-fluxcd/distribution/tree/main/docs/mcp) - contains the user guides for the MCP Server

The documentation website is built using [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/).

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
make all
```

In general, we will merge a PR once one maintainer has endorsed it.
For significant changes, more people may become involved, and you might
get asked to resubmit the PR or divide the changes into more than one PR.

### Format of the Commit Message

For this project, we prefer the following rules for good commit messages:

- Limit the subject to 50 characters and write as the continuation
  of the sentence "If applied, this commit will ..."
- Explain what and why in the body, if more than a trivial change;
  wrap it at 72 characters.

The [following article](https://chris.beams.io/posts/git-commit/#seven-rules)
has some more helpful advice on documenting your work.

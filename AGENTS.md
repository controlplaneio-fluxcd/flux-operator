# AGENTS instructions

For detailed information on source code structure and development guidelines,
refer to the project's [CONTRIBUTING.md](CONTRIBUTING.md) file.

## Project Overview

This project is a Kubernetes operator for managing the lifecycle of Flux CD.
It provides a declarative API for installing, configuring, and upgrading the Flux distribution.
The operator extends Flux with self-service capabilities, deployment windows, and preview environments.

The project is structured as a Go module with the following components:

* **Flux Operator Kubernetes Controller**: The main component of the project. It is built using the Kubernetes controller-runtime managing the following CRDs:
    - **FluxInstance CRD**: Manages the Flux controllers installation and configuration
    - **FluxReport CRD**: Reflects the state of a Flux installation
    - **ResourceSet CRD**: Manages groups of Kubernetes and Flux resources based on input matrices
    - **ResourceSetInputProvider CRD**: Fetches input values from external services (GitHub, GitLab, Azure DevOps, Container Registries)
* **Flux Operator CLI**: A command-line interface for interacting with the Flux Operator. It is built using the Cobra library.
* **Flux Operator MCP Server**: A server that connects AI assistants to Kubernetes clusters running the operator. It is built using the mcp-go library.
* **Flux Status Page**: A web UI for displaying the status of the Flux GitOps pipelines. It is built using Preact, Vite, and Tailwind CSS.

## Directory Structure

```bash
├── api/            # Go API definitions for Kubernetes CRDs
├── cmd/            # Main entrypoint for the binaries
│   ├── cli/        # Flux Operator CLI
│   ├── mcp/        # Flux Operator MCP Server
│   └── operator/   # Flux Operator Kubernetes Controller
├── config/         # Kubernetes manifests for deploying the operator
├── docs/           # Kubernetes APIs and MCP tools documentation
├── hack/           # Scripts for development, building, and releases
├── internal/       # Internal Go packages
│   ├── controller/ # Controller reconciliation logic
│   └── web/        # Backend for the Flux Status Page
├── test/           # End-to-end tests with Kubernetes Kind
└── web/            # Preact frontend for the Flux Status Page
```

## Rules of Engagement for AI Agents

- Do not deviate from the established patterns in the codebase
- New files must have a license header matching existing files
- All new features must have associated documentation
- Never run `git tag` and never push tags
- Follow the Go code style used in the project
- Add proper doc comments for new functions and types
- After modifying a function or type, update its doc comment
- Add comments for complex logic but don't comment obvious code
- Read existing tests before writing new ones
- Run individual tests when debugging, run the full test suite when done
- Replace `interface{}` with `any` type alias
- Use `go doc` to read func signatures in external packages

## Development Commands for AI Agents

- Run `make generate` after modifying types in the `api` dir
- Run `make fmt lint` after completing a task and fix lint errors
- Run `make test` to execute all Go tests
- Run `make cli-test` for Flux Operator CLI specific tests
- Run `make mcp-test` for Flux Operator MCP server specific tests
- Run `make web-test` for Flux Status Page frontend tests

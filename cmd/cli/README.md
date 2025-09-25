---
title: Flux Operator CLI
description: Flux Operator command line tool installation and usage guide
---

# Flux Operator CLI

The Flux Operator CLI is a command line tool that allows you to manage the Flux Operator resources
in your Kubernetes clusters. It provides a convenient way to interact with the operator
and perform various operations.

## Installation

The Flux Operator CLI is available as a binary executable for Linux, macOS, and Windows. The binaries
can be downloaded from GitHub [releases page](https://github.com/controlplaneio-fluxcd/flux-operator/releases).

If you are using macOS or Linux, you can install the CLI using Homebrew:

```shell
brew install controlplaneio-fluxcd/tap/flux-operator
```

To configure your shell to load `flux-operator` Bash completions add to your profile:

```shell
echo "source <(flux-operator completion bash)" >> ~/.bash_profile
```

Zsh, Fish, and PowerShell are also supported with their own sub-commands.

### Container Image

The Flux Operator CLI is also available as a container image, which can be used in CI pipelines
or Kubernetes Jobs. The image contains the `flux-operator` CLI binary and the `kubectl` binary.

The multi-arch image (Linux AMD64/ARM64) is hosted on GitHub Container Registry at
`ghcr.io/controlplaneio-fluxcd/flux-operator-cli`.

```shell
version=$(gh release view --repo controlplaneio-fluxcd/flux-operator --json tagName -q '.tagName')
docker run --rm -it --entrypoint=flux-operator ghcr.io/controlplaneio-fluxcd/flux-operator-cli:$version help
docker run --rm -it --entrypoint=kubectl ghcr.io/controlplaneio-fluxcd/flux-operator-cli:$version help
```

## Commands

The Flux Operator CLI provides commands to manage the Flux Operator resources.
Except for the `build` commands, all others require access to the Kubernetes cluster
and the Flux Operator to be installed.

The CLI connects to the cluster using the `~.kube/config` file, similar to `kubectl`.

All commands display help information and example usage when run with the `-h` or `--help` flag.

### Build Commands

The `flux-operator build` commands are used to build and validate the Flux Operator resources.
These commands do not require access to a Kubernetes cluster and can be run in any environment.

The following commands are available:

- `flux-operator build instance`: Generates the Flux Kubernetes manifests from a FluxInstance definition.
    - `-f, --file`: Path to the FluxInstance YAML manifest (required).
- `flux-operator build rset`: Generates the Kubernetes manifests from a ResourceSet definition.
    - `-f, --file`: Path to the ResourceSet YAML manifest (required).
    - `--inputs-from`: Path to the ResourceSet inputs YAML manifest.
    - `--inputs-from-provider`: Path to the ResourceSetInputProvider static type YAML manifest.

### Get Commands

The `flux-operator get` commands are used to retrieve information about the Flux Operator resources in the cluster.

The following commands are available:

- `flux-operator get instance`: Retrieves the FluxInstance resource in the cluster.
- `flux-operator get rset`: Retrieves the ResourceSet resources in the cluster.
- `flux-operator get rsip`: Retrieves the ResourceSetInputProvider resources in the cluster.
- `flux-operator get all`: Retrieves all Flux resources in the cluster (supports filtering by ready status).

Arguments:

- `-n, --namespace`: Specifies the namespace to filter the resources.
- `-A, --all-namespaces`: Retrieves resources from all namespaces.

### Export Commands

The `flux-operator export` commands are used to export the Flux Operator resources in YAML format.
The exported resources can be used for backup, migration, or inspection purposes.

The following commands are available:

- `flux-operator export report`: Exports the FluxReport resource containing the distribution status and version information.
- `flux-operator export resource <kind>/<name>`: Exports a Flux resource from the specified namespace.

Arguments:

- `-n, --namespace`: Specifies the namespace scope of the command.

### Reconcile Commands

The `flux-operator reconcile` commands are used to trigger the reconciliation of the Flux Operator resources.

The following commands are available:

- `flux-operator reconcile instance <name>`: Reconciles the FluxInstance resource in the cluster.
- `flux-operator reconcile rset <name>`: Reconciles the ResourceSet resource in the cluster.
- `flux-operator reconcile rsip <name>`: Reconciles the ResourceSetInputProvider resource in the cluster.
- `flux-operator reconcile resource <kind>/<name>`: Reconciles a Flux resource in the specified namespace.
- `flux-operator reconcile all`: Reconciles all Flux resources in the cluster (supports filtering by ready status).

Arguments:

- `-n, --namespace`: Specifies the namespace scope of the command.
- `--wait`: Waits for the reconciliation to complete before returning.

### Suspend/Resume Commands

The `flux-operator suspend` and `flux-operator resume` commands are used
to suspend or resume the reconciliation of the Flux Operator resources.

The following commands are available:

- `flux-operator suspend instance <name>`: Suspends the reconciliation of the FluxInstance resource in the cluster.
- `flux-operator resume instance <name>`: Resumes the reconciliation of the FluxInstance resource in the cluster.
- `flux-operator suspend rset <name>`: Suspends the reconciliation of the ResourceSet resource in the cluster.
- `flux-operator resume rset <name>`: Resumes the reconciliation of the ResourceSet resource in the cluster.
- `flux-operator suspend rsip <name>`: Suspends the reconciliation of the ResourceSetInputProvider resource in the cluster.
- `flux-operator resume rsip <name>`: Resumes the reconciliation of the ResourceSetInputProvider resource in the cluster.
- `flux-operator suspend resource <kind>/<name>`: Suspends the reconciliation of the Flux resource in the cluster.
- `flux-operator resume resource <kind>/<name>`: Resumes the reconciliation of the Flux resource in the cluster.

Arguments:

- `-n, --namespace`: Specifies the namespace scope of the command.
- `--wait`: On resume, waits for the reconciliation to complete before returning.

### Statistics Command

This command is used to retrieve statistics about the Flux resources
including their reconciliation status and the amount of cumulative storage used for each source type.

- `flux-operator stats`: Displays statistics about the Flux resources in the cluster.

### Trace Command

This command is used to trace Kubernetes objects throughout the GitOps delivery pipeline
to identify which Flux reconciler manages them and from which source they originate.

- `flux-operator trace <kind>/<name>`: Trace a Kubernetes object to its Flux reconciler and source.

Arguments:

- `-n, --namespace`: Specifies the namespace scope of the command.

### Tree Commands

The `flux-operator tree` commands are used to visualize the Flux-managed Kubernetes objects in a tree format
by recursively traversing the Flux resources such as ResourceSets, Kustomizations and HelmReleases.

The following commands are available:

- `flux-operator tree rset <name>`: Print a tree view of the ResourceSet managed objects.
- `flux-operator tree ks <name>`: Print a tree view of the Flux Kustomization managed objects.
- `flux-operator tree hr <name>`: Print a tree view of the Flux HelmRelease managed objects.

Arguments:

- `-n, --namespace`: Specifies the namespace scope of the command.

### Wait Commands

The `flux-operator wait` commands are used to wait for Flux Operator resources to become ready.
These commands will poll the resource status until it reaches a ready state or times out.
If the resource is not created or its status is not ready within the specified timeout,
the command will return an error.

The following commands are available:

- `flux-operator wait instance <name>`: Wait for a FluxInstance to become ready.
- `flux-operator wait rset <name>`: Wait for a ResourceSet to become ready.
- `flux-operator wait rsip <name>`: Wait for a ResourceSetInputProvider to become ready.

Arguments:

- `-n, --namespace`: Specifies the namespace scope of the command.
- `--timeout`: The length of time to wait before giving up (default 1m).

### Create Secret Commands

The `flux-operator create secret` commands are used to create Kubernetes secrets specific to Flux.
These commands can be used to create or update secrets directly in the cluster, or to export them in YAML format.

The following commands are available:

- `flux-operator create secret basic-auth`: Create a Kubernetes Secret containing basic auth credentials.
  - `--username`: Set the username for basic authentication (required).
  - `--password`: Set the password for basic authentication (required if --password-stdin is not used).
  - `--password-stdin`: Read the password from stdin.
- `flux-operator create secret githubapp`: Create a Kubernetes Secret containing GitHub App credentials.
  - `--app-id`: GitHub App ID (required).
  - `--app-installation-id`: GitHub App Installation ID (required).
  - `--app-private-key-file`: Path to GitHub App private key file (required).
  - `--app-base-url`: GitHub base URL for GitHub Enterprise Server (optional).
- `flux-operator create secret proxy`: Create a Kubernetes Secret containing HTTP/S proxy credentials.
  - `--address`: Set the proxy address (required).
  - `--username`: Set the username for proxy authentication (optional).
  - `--password`: Set the password for proxy authentication (optional).
  - `--password-stdin`: Read the password from stdin.
- `flux-operator create secret registry`: Create a Kubernetes Secret containing registry credentials.
  - `--server`: Set the registry server (required).
  - `--username`: Set the username for registry authentication (required).
  - `--password`: Set the password for registry authentication (required if --password-stdin is not used).
  - `--password-stdin`: Read the password from stdin.
- `flux-operator create secret sops`: Create a Kubernetes Secret containing SOPS decryption keys.
  - `--age-key-file`: Path to Age private key file (can be used multiple times).
  - `--gpg-key-file`: Path to GPG private key file (can be used multiple times).
  - `--age-key-stdin`: Read Age private key from stdin.
  - `--gpg-key-stdin`: Read GPG private key from stdin.
- `flux-operator create secret ssh`: Create a Kubernetes Secret containing SSH credentials.
  - `--private-key-file`: Path to SSH private key file (required).
  - `--public-key-file`: Path to SSH public key file (optional).
  - `--knownhosts-file`: Path to SSH known_hosts file (required).
  - `--password`: Password for encrypted SSH private key (optional).
  - `--password-stdin`: Read the password from stdin.
- `flux-operator create secret tls`: Create a Kubernetes Secret containing TLS certs.
  - `--tls-crt-file`: Path to TLS client certificate file.
  - `--tls-key-file`: Path to TLS client private key file.
  - `--ca-crt-file`: Path to CA certificate file (optional).

Arguments:

- `-n, --namespace`: Specifies the namespace to create the secret in.
- `--annotation`: Set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2).
- `--label`: Set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2).
- `--immutable`: Set the immutable flag on the Secret.
- `--export`: Export secret in YAML format to stdout instead of creating it in the cluster.

### Version Command

This command is used to display the version of the CLI, of the Flux Operator
and of the Flux distribution running in the cluster.

- `flux-operator version`: Displays the version information for the CLI and the Flux Operator.
    - `--client`:  If true, shows the client version only (no server required).

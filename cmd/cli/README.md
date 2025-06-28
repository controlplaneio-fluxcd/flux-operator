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

## Commands

The Flux Operator CLI provides commands to manage the Flux Operator resources.
Except for the `build` commands, all others require access to the Kubernetes cluster
and the Flux Operator to be installed.

The CLI connects to the cluster using the `~.kube/config` file, similar to `kubectl`.

### Build Commands

The `flux-operator build` commands are used to build and validate the Flux Operator resources.
These commands do not require access to a Kubernetes cluster and can be run in any environment.

#### `flux-operator build instance`

The build instance command performs the following steps:

1. Reads the FluxInstance YAML manifest from the specified file.
2. Validates the instance definition and sets default values.
3. Pulls the distribution OCI artifact from the registry using the Docker config file for authentication.
   If not specified, the artifact is pulled from 'oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests'.
4. Builds the Flux Kubernetes manifests according to the instance specifications and kustomize patches.
5. Prints the multi-doc YAML containing the Flux Kubernetes manifests to stdout.

Example usage:

```shell
# Build the given FluxInstance and print the generated manifests
flux-operator build instance -f flux.yaml

# Pipe the FluxInstance definition to the build command
cat flux.yaml | flux-operator build instance -f -

# Build a FluxInstance and print a diff of the generated manifests
flux-operator build instance -f flux.yaml | \
  kubectl diff --server-side --field-manager=flux-operator -f -
```

#### `flux-operator build rset`

The build rset command performs the following steps:

1. Reads the ResourceSet YAML manifest from the specified file.
2. Validates the ResourceSet definition and sets default values.
3. Extracts the inputs from the ResourceSet and validates the templates.
4. Builds the Kubernetes manifests according to the ResourceSet specifications and templates.
5. Prints the multi-doc YAML containing the Kubernetes manifests to stdout.

Example usage:

```shell
# Build the given ResourceSet and print the generated objects
flux-operator build rset -f my-resourceset.yaml

# Build a ResourceSet by providing the inputs from a file
flux-operator build rset -f my-resourceset.yaml \
--inputs-from my-resourceset-inputs.yaml

# Pipe the ResourceSet manifest to the build command
cat my-resourceset.yaml | flux-operator build rset -f -

# Build a ResourceSet and print a diff of the generated objects
flux-operator build rset -f my-resourceset.yaml | \
kubectl diff --server-side --field-manager=flux-operator -f -
```

### Get Commands

The `flux-operator get` commands are used to retrieve information about the Flux Operator resources in the cluster.

The following commands are available:

- `flux-operator get instance`: Retrieves the FluxInstance resource in the cluster.
- `flux-operator get rset`: Retrieves the ResourceSet resources in the cluster.
- `flux-operator get rsip`: Retrieves the ResourceSetInputProvider resources in the cluster.
- `flux-operator get resources`: Retrieves all Flux resources in the cluster.

Arguments:

- `-n, --namespace`: Specifies the namespace to filter the resources.
- `-A, --all-namespaces`: Retrieves resources from all namespaces.

### Export Commands

The `flux-operator export` commands are used to export the Flux Operator resources in YAML format.
The exported resources can be used for backup, migration, or inspection purposes.

The following commands are available:

- `flux-operator export report`: Exports the FluxReport resource containing the distribution status and version information.
- `flux-operator export resource <kind>/<name> -n <namespace>`: Exports a Flux resource from the specified namespace.

### Reconcile Commands

The `flux-operator reconcile` commands are used to trigger the reconciliation of the Flux Operator resources.

The following commands are available:

- `flux-operator reconcile instance <name> -n <namespace>`: Reconciles the FluxInstance resource in the cluster.
- `flux-operator reconcile rset <name> -n <namespace>`: Reconciles the ResourceSet resource in the cluster.
- `flux-operator reconcile rsip <name> -n <namespace>`: Reconciles the ResourceSetInputProvider resource in the cluster.
- `flux-operator reconcile resource <kind>/<name> -n <namespace>`: Reconciles a Flux resource in the specified namespace.

### Suspend/Resume Commands

The `flux-operator suspend` and `flux-operator resume` commands are used
to suspend or resume the reconciliation of the Flux Operator resources.

The following commands are available:

- `flux-operator suspend instance <name> -n <namespace>`: Suspends the reconciliation of the FluxInstance resource in the cluster.
- `flux-operator resume instance <name> -n <namespace>`: Resumes the reconciliation of the FluxInstance resource in the cluster.
- `flux-operator suspend rset <name> -n <namespace>`: Suspends the reconciliation of the ResourceSet resource in the cluster.
- `flux-operator resume rset <name> -n <namespace>`: Resumes the reconciliation of the ResourceSet resource in the cluster.
- `flux-operator suspend rsip <name> -n <namespace>`: Suspends the reconciliation of the ResourceSetInputProvider resource in the cluster.
- `flux-operator resume rsip <name> -n <namespace>`: Resumes the reconciliation of the ResourceSetInputProvider resource in the cluster.
- `flux-operator suspend resource <kind>/<name> -n <namespace>`: Suspends the reconciliation of the Flux resource in the cluster.
- `flux-operator resume resource <kind>/<name> -n <namespace>`: Resumes the reconciliation of the Flux resource in the cluster.

### Statistics Command

The `flux-operator stats` command is used to retrieve statistics about the Flux resources
including their reconciliation status and the amount of cumulative storage used for each source type.

### Version Command

The `flux-operator version` command is used to display the version of the CLI, of the Flux Operator
and of the Flux distribution running in the cluster.

Arguments:

- `--client`: If true, shows the client version only (no server required).

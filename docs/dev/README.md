# Flux Operator Dev Documentation

## Release Procedure

### Flux Operator

1. Switch to the `main` branch and pull the latest commit of the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator).
2. Run `make prep-release` that increments the minor version and opens a PR with the changes. (Or set a version with `NEXT_VERSION=v1.2.3 make prep-release`.)
3. Merge the PR and pull the latest commit from the `main` branch locally.
4. Run `make release` that creates a new tag for the release and pushes it to the remote repository.
5. Wait for the `release` GitHub Workflow to finish.

### Helm Chart

1. Run the `update` GitHub Workflow in the [`controlplaneio-fluxcd/charts` repository](https://github.com/controlplaneio-fluxcd/charts/actions/workflows/update.yaml).
2. Merge the PR opened by the `update` GitHub Workflow.
3. Wait for the `test` workflow to pass on the `main` branch.
4. Tag the `main` branch with the new next semver from [`controlplaneio-fluxcd/charts` repository](https://github.com/controlplaneio-fluxcd/charts/tags), e.g. `git tag -s -m "v0.2.0" "v0.2.0"`.
5. Wait for the `release` GitHub Workflow to finish.
6. After the Helm chart is published, the new version will be available at [artifacthub.io/packages/helm/flux-operator/flux-operator](https://artifacthub.io/packages/helm/flux-operator/flux-operator).

### OperatorHub Bundle

1. Validate the new version by running the `e2e-olm` GitHub Workflow in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e-olm.yaml).
2. Generate the OLM manifests locally by running `make build-olm-manifests`.
3. Fork the [OperatorHub.io repository](https://github.com/k8s-operatorhub/community-operators) and clone it locally.
4. Create a new branch from `main`, e.g. `flux-operator-1.0.0`.
5. Copy the OLM manifests from the `flux-operator/bin/olm/1.0.0` dir to the `community-operators/operators/flux-operator/1.0.0`.
6. Commit the changes with the title `operator flux-operator (1.0.0)` and push the branch to the fork.
7. Open a PR in the upstream repository and wait for the CI to pass.
8. After the PR is merged, the new version will be available at [operatorhub.io/operator/flux-operator](https://operatorhub.io/operator/flux-operator).

### RedHat OpenShift Bundle

1. Generate the OLM manifests for the UBI version locally by running `make build-olm-manifests-ubi`.
2. Fork the [redhat-openshift-ecosystem/community-operators-prod repository](https://github.com/redhat-openshift-ecosystem/community-operators-prod) and clone it locally.
3. Create a new branch from `main`, e.g. `flux-operator-1.0.0`.
4. Copy the OLM manifests from the `flux-operator/bin/olm/1.0.0` dir to the `community-operators-prod/operators/flux-operator/1.0.0`.
5. Commit the changes with the title `operator flux-operator (1.0.0)` and push the branch to the fork.
6. Open a PR in the upstream repository and wait for the CI to pass.
7. After the PR is merged, the new version will be available in the OpenShift Container Platform catalog.

### Homebrew Tap

1. Trigger the `release` GitHub Workflow in the [`controlplaneio-fluxcd/homebrew-tap` repository](https://github.com/controlplaneio-fluxcd/homebrew-tap/blob/main/.github/workflows/release.yml).
2. Merge the PR opened by the `release` workflow.
3. Verify the new version is available by running `brew upgrade flux-operator flux-operator-mcp` on your local machine.

### Documentation Website

1. Trigger the `vendor-operator-docs` GitHub Workflow in the [`controlplaneio-fluxcd/distribution` repository](https://github.com/controlplaneio-fluxcd/distribution/blob/main/.github/workflows/vendor-operator-docs.yaml).
2. Merge the PR opened by the `vendor-operator-docs` workflow.
3. Trigger the `docs` GitHub Workflow in the [`controlplaneio-fluxcd/distribution` repository](https://github.com/controlplaneio-fluxcd/distribution/blob/main/.github/workflows/docs.yaml).
4. Wait for the `docs` workflow to finish and verify the changes on the [Flux Operator documentation site](https://fluxcd.control-plane.io/operator/).

## Manifests Release Procedure

### Manifests Update for a New Flux Version

1. Create a new branch from `main`, e.g. `flux-v2.x.x` in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator).
2. Generate the manifests for the latest Flux version by running `make vendor-flux`.
3. Build the manifests with images digests by running `make build-manifests`.
4. Write an end-to-end test for the upgrade if the new Flux version is a minor release.
5. Run `make mcp-build-search-index` to rebuild the docs index if the new Flux version is a minor release. 
6. Commit changes and open a PR.
7. After the PR is merged, publish the OCI artifact with the manifests by running the [`push-manifests` GitHub Workflow](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/push-manifests.yml).

### Manifests Update for Enterprise CVE Fixes

1. Create a new branch from `main`, e.g. `enterprise-cve-fixes` in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator).
2. Rebuild the Flux manifests with the latest image patches by running `make vendor-flux`.
3. Commit changes and open a PR.
4. After the PR is merged, publish the OCI artifact with the manifests by running the [`push-manifests` GitHub Workflow](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/push-manifests.yml).

## Local Development

### Prerequisites

- [Go](https://golang.org/doc/install) 1.25+
- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

### Building

After code changes, run the following command:

```sh
make all
```

### Unit Testing

Unit tests can be run with:

```sh
make test
```

### End-to-End Testing

First, create a cluster named `kind`:

```sh
kind create cluster
```

End-to-end tests can be run with:

```sh
make test-e2e
```

### Manual Testing

Build and run the operator in a Kind cluster:

```sh
IMG=flux-operator:test1 make docker-build load-image deploy
```

Make sure to increment the `test1` tag for each new build.

Apply the instance from the `config/samples` dir:

```sh
kubectl -n flux-system apply -f config/samples/fluxcd_v1_fluxinstance.yaml
```

Check the logs:

```sh
kubectl -n flux-system logs deployment/flux-operator --follow
```

To test the Flux Operator CLI, build it with:

```sh
make cli-build
```

Then run the CLI with:

```sh
./bin/flux-operator-cli get instance flux -n flux-system
```

To clean up the resources, run:

```sh
kubectl -n flux-system delete fluxinstance/flux
make undeploy
```

### Upgrading the Go Version

To upgrade Go to the latest minor version, follow these steps:

1. Bump the `go` minor version in the `go.mod` file.
2. Bump `GOLANGCI_LINT_VERSION` to match the new Go version.
3. Run `make` to validate that the build and tests pass with the new Go version.
4. Bump the `golang` image tag in all `Dockerfile`s.
5. Bump the `go-version` in all GitHub workflows.
6. Bump the Go version in [prerequisites](#prerequisites) section of this document.

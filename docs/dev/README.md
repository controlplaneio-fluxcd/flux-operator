# Flux Operator Dev Documentation

## Release Procedure

### Flux Operator

1. Create a new release branch from `main`, e.g. `release-v2.0.0` in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator).
2. Bump the version in the `newTag` field from `config/manager/kustomization.yaml`.
3. Commit the changes with the title `Release v2.0.0`, push and open a PR.
4. After the PR is merged, tag the `main` branch with the new version, e.g. `git tag -s -m "v2.0.0" "v2.0.0"`.
5. Wait for the `release` GitHub Workflow to finish.

### Helm Chart

1. Run the `update` GitHub Workflow in the [`controlplaneio-fluxcd/charts` repository](https://github.com/controlplaneio-fluxcd/charts/actions/workflows/update.yaml).
2. Merge the PR opened by the `update` GitHub Workflow.
3. Wait for the `test` workflow to pass on the `main` branch.
4. Tag the `main` branch with the new chart version, e.g. `git tag -s -m "v2.0.0" "v2.0.0"`.
5. Wait for the `release` GitHub Workflow to finish.
6. After the Helm chart is published, the new version will be available at [artifacthub.io/packages/helm/flux-operator/flux-operator](https://artifacthub.io/packages/helm/flux-operator/flux-operator).

### OperatorHub Bundle

1. Validate the new version by running the `e2e-olm` GitHub Workflow in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e-olm.yml).
2. Generate the OLM manifests locally by running `make build-olm-manifests`.
3. Fork the [OperatorHub.io repository](https://github.com/k8s-operatorhub/community-operators) and clone it locally.
4. Create a new branch from `main`, e.g. `flux-operator-2.0.0`.
5. Copy the OLM manifests from the `flux-operator/bin/olm/2.0.0` dir to the `community-operators/operators/flux-operator/2.0.0`.
6. Commit the changes with the title `operator flux-operator (2.0.0)` and push the branch to the fork.
7. Open a PR in the upstream repository and wait for the CI to pass.
8. After the PR is merged, the new version will be available at [operatorhub.io/operator/flux-operator](https://operatorhub.io/operator/flux-operator).

## Manifests Release Procedure

## Manifests Update for a New Flux Version

1. Create a new branch from `main`, e.g. `flux-v2.x.x` in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator).
2. Generate the manifests for the latest Flux version by running `make vendor-flux`.
3. Build the manifests with images digests by running `make build-manifests`.
4. Write an end-to-end test for the upgrade if the new Flux version is a minor release.
5. Commit changes and open a PR.
6. After the PR is merged, publish the OCI artifact with the manifests by running the [`push-manifests` GitHub Workflow](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/push-manifests.yml).

## Manifests Update for Enterprise CVE Fixes

1. Create a new branch from `main`, e.g. `enterprise-cve-fixes` in the [`controlplaneio-fluxcd/flux-operator` repository](https://github.com/controlplaneio-fluxcd/flux-operator).
2. Rebuild the Flux manifests with the latest image patches by running `make build-manifests`.
3. Commit changes and open a PR.
4. After the PR is merged, publish the OCI artifact with the manifests by running the [`push-manifests` GitHub Workflow](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/push-manifests.yml).

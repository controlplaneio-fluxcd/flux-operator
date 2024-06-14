# Flux Operator Dev Documentation

## Release Procedure

1. Create a new release branch from `main`, e.g. `release-v2.0.0`.
2. Bump the version in the `newTag` field from `config/manager/kustomization.yaml`.
3. Generate the OLM manifests by adding the new version to `config/operatorhub/flux-operator`.
4. Open a PR with the changes.
5. After the PR is merged, tag the `main` branch with the new version, e.g. `git tag -s -m "v2.0.0" "v2.0.0"`.
6. After the release workflow finishes, trigger the OLM end-to-end tests by running the `e2e-olm` GitHub Workflow.
7. Open a PR in the [OperatorHub.io repository](https://github.com/k8s-operatorhub/community-operators) to update the Flux Operator.
8. Run the `update` GitHub Workflow in the [`controlplaneio-fluxcd/charts` repository](https://github.com/controlplaneio-fluxcd/charts/actions/workflows/update.yaml).
9. Merge the PR with the chart changes and tag the `main` branch of the `controlplaneio-fluxcd/charts` repository to release the new chart version.

## Flux Manifests Update

1. Generate the manifests for the latest Flux version by running `make vendor-flux`.
2. Test the Flux version locally by running `make test-e2e`.
3. Open a PR with the changes.
4. After the PR is merged, publish the OCI artifacts with the manifests by running the `push-manifests` GitHub Workflow.

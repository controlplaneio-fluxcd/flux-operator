---
title: Migration
description: Migrating existing Flux installations to Flux Operator managed instances
---

# Flux Bootstrap Migration

Assuming you have a cluster bootstrapped with the Flux CLI or the Terraform Provider,
you can migrate to an operator-managed Flux with zero downtime.

## Install the Flux Operator

Install the Flux Operator in the same namespace where Flux is deployed, for example using Helm:

```shell
helm install flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system
```

Or by using an alternative installation method described in the [installation guide](operator-install.md).

## Create a Flux Instance

Create a `FluxInstance` resource named **flux** in the `flux-system` namespace using
the same configuration as for `flux bootstrap`. 

For example, if you have bootstrapped the cluster with the following command:

```shell
flux bootstrap github \
  --owner=my-org \
  --repository=my-fleet \
  --branch=main \
  --path=clusters/my-cluster
```

The equivalent `FluxInstance` configuration would look like this:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
  cluster:
    type: kubernetes
    multitenant: false
    networkPolicy: true
    domain: "cluster.local"
  sync:
    kind: GitRepository
    url: "ssh://git@github.com/my-org/my-fleet.git"
    ref: "refs/heads/main"
    path: "clusters/my-cluster"
    pullSecret: "flux-system"
```

!!! note "Kustomize patches"

    Note that if you have customized the Flux manifests, you should copy the Kustomize patches
    from `flux-system/kustomization.yaml` in the `FluxInstance` under `.spec.kustomize.patches`.
    For more information, see the [instance customization guide](instance-customization.md).

Apply the `FluxInstance` resource to the cluster:

```shell
kubectl apply -f flux-instance.yaml
```

Once the resource is reconciled, the operator will take over the management of the Flux components,
the Flux GitRepository and Kustomization.

To verify that the migration was successful, check the status of the `FluxInstance`:

```shell
kubectl -n flux-system get fluxinstance flux
```

Running the trace command should result in a "Not managed by Flux" message:

```shell
flux trace kustomization flux-system
```

## Cleanup the repository

To finalize the migration, remove the Flux manifests from the Git repository:

1. Checkout the main branch of the Flux repository that was used to bootstrap the cluster.
2. Delete the `flux-system` directory from the repository `clusters/my-cluster` directory.
3. Optionally, place the `FluxInstance` YAML manifest in the `clusters/my-cluster` directory.
4. Commit and push the changes to the Flux repository.

## Automating Flux Operator upgrades

If the Flux Operator is installed with Helm, you can automate the upgrade process using a Flux `HelmRelease`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  inputs:
    - version: "*"
  resources:
   - apiVersion: source.toolkit.fluxcd.io/v1
     kind: OCIRepository
     metadata:
      name: << inputs.provider.name >>
      namespace: << inputs.provider.namespace >>
     spec:
      interval: 30m
      url: oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator
      layerSelector:
        mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
        operation: copy
      ref:
        semver: << inputs.version | quote >>
   - apiVersion: helm.toolkit.fluxcd.io/v2
     kind: HelmRelease
     metadata:
        name: << inputs.provider.name >>
        namespace: << inputs.provider.namespace >>
     spec:
      interval: 30m
      releaseName: << inputs.provider.name >>
      serviceAccountName: << inputs.provider.name >>
      chartRef:
        kind: OCIRepository
        name: << inputs.provider.name >>
```

Commit and push the manifest to the Flux repository, and the operator will be automatically upgraded
when a new Helm chart version is released.

## Migration from Git to OCI artifacts

To decouple the Flux reconciliation from Git and use OCI artifacts as the delivery mechanism
for the cluster desired state, the following procedure can be followed:

1. Migrate the Flux custom resources such as Flux `Kustomization` and `HelmRelease` to use `OCIRepository` as `sourceRef`.
2. Create a repository in a container registry that both the CI tooling and Flux can access.
3. Create a CI workflow that reacts to changes in the Git repository and publishes the Kubernetes manifests
   to the OCI repository.
4. Configure the `FluxInstance` to use the OCI repository as the source of the cluster's desired state.

To exemplify the migration, we will use GitHub but the same procedure can be applied to GitLab,
Azure DevOps and other providers.

### Prepare the Flux manifests

Create a new branch called `oci-artifacts` in the Git repository that was used for bootstrap.

Update all the Flux `Kustomization` manifests to use `OCIRepository` instead of `GitRepository`:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
spec:
  sourceRef:
    kind: OCIRepository
    name: flux-system
```

If you have `HelmRelease` resources using a `GitRepository`, update them to use `OCIRepository`.

Commit and push the changes to the `oci-artifacts` branch.

### Publish the manifests to the OCI repository

Create a GitHub Actions workflow that uses the Flux CLI to publish the manifests to GitHub Container Registry:

```yaml
name: publish-artifact

on:
  workflow_dispatch:
  push:
    branches:
      - 'main'
      - 'oci-artifacts'

permissions:
  packages: write

jobs:
  flux-push:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Setup Flux CLI
        uses: fluxcd/flux2/action@main       
      - name: Push immutable artifact
        run: |
          flux push artifact \
            oci://ghcr.io/${{ github.repository }}/manifests:$(git rev-parse --short HEAD) \
            --source="$(git config --get remote.origin.url)" \
            --revision="$(git branch --show-current)@sha1:$(git rev-parse HEAD)" \
            --creds flux:${{ secrets.GITHUB_TOKEN }} \
            --path="./"
      - name: Tag artifact as latest
        run: |
          flux tag artifact \
            oci://ghcr.io/${{ github.repository }}/manifests:$(git rev-parse --short HEAD) \
            --creds flux:${{ secrets.GITHUB_TOKEN }} \
            --tag latest
```

Commit and push the workflow to the `oci-artifacts` branch.

Run the workflow manually in the GitHub UI and verify that the manifests
are published to the GitHub Container Registry with:

```shell
flux pull artifact oci://ghcr.io/my-org/my-fleet/manifests:latest \
    --creds flux:${GITHUB_TOKEN} \
    --output-dir ./manifests
```

### Create the image pull secret

Create an image pull secret in the `flux-system` namespace that contains
a GitHub token with read access to the GitHub Container Registry:

```shell
flux create secret oci ghcr-auth \
    --url=ghcr.io \
    --username=flux \
    --password=${GITHUB_TOKEN}
```

### Update the FluxInstance to use OCI artifacts

Update the `FluxInstance` to use `OCIRepository` and the image pull secret:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  sync:
    kind: OCIRepository
    url: "oci://ghcr.io/my-org/my-fleet/manifests"
    ref: "latest"
    path: "clusters/my-cluster"
    pullSecret: "ghcr-auth"
```

Commit and push the `FluxInstance` changes to the `oci-artifacts` branch and
wait for the GitHub workflow to publish the manifests.

Apply the `FluxInstance` to the cluster and verify that the operator has reconfigured
Flux to use the `OCIRepository`:

```shell
kubectl apply -f flux-instance.yaml
kubectl -n flux-system wait fluxinstance/flux --for=condition=Ready

flux get source oci flux-system
flux get kustomization flux-system
```

Finally, merge the `oci-artifacts` branch into `main` and delete the `oci-artifacts` branch.
The GitHub Actions workflow will continue to publish the manifests to the GitHub Container Registry
on every push to the `main` branch and Flux will reconcile the cluster state accordingly.

# flux-operator

[![release](https://img.shields.io/github/release/controlplaneio-fluxcd/flux-operator/all.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/releases)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/flux-operator)](https://artifacthub.io/packages/helm/flux-operator/flux-operator)
[![Operator Hub](https://img.shields.io/badge/Operator_Hub-flux--operator-9cf.svg)](https://operatorhub.io/operator/flux-operator)
[![e2e](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml)
[![license](https://img.shields.io/github/license/controlplaneio-fluxcd/flux-operator.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](https://fluxcd.control-plane.io/distribution/security/)

The Flux Operator is a Kubernetes CRD controller that manages
the lifecycle of CNCF [Flux CD](https://fluxcd.io) and the
[ControlPlane enterprise distribution](https://github.com/controlplaneio-fluxcd/distribution).

> [!IMPORTANT]
> Note that this project in under active development.
> The APIs and features specs are described in
> [RFC-0001](https://github.com/controlplaneio-fluxcd/distribution/tree/main/rfcs/0001-flux-operator/README.md).

## Features

- Provides a declarative API for the installation, configuration and upgrade of Flux.
- Automates the patching of hotfixes and CVEs affecting the Flux controllers container images.
- Simplifies the configuration of multi-tenancy lockdown on shared Kubernetes clusters.
- Allows syncing the cluster state from Git repositories, OCI artifacts and S3-compatible storage.
- Provides a security-first approach to the Flux deployment and FIPS compliance.
- Incorporates best practices for running Flux at scale with persistent storage and vertical scaling.
- Manages the update of Flux custom resources and prevents disruption during the upgrade process.
- Facilitates a clean uninstall and reinstall process without affecting the Flux-managed workloads.
- Provides first-class support for OpenShift, Azure, AWS, GCP and other marketplaces.

## Documentation

- [Flux operator installation](https://fluxcd.control-plane.io/operator/install/)
- [Flux controllers configuration](https://fluxcd.control-plane.io/operator/flux-config/)
- [Cluster sync configuration](https://fluxcd.control-plane.io/operator/flux-sync/)
- [Migration of bootstrapped clusters](https://fluxcd.control-plane.io/operator/flux-bootstrap-migration/)
- [FluxInstance API reference](https://fluxcd.control-plane.io/operator/fluxinstance/)

## Quickstart Guide

### Install the Flux Operator

Install the Flux Operator in the `flux-system` namespace, for example using Helm:

```shell
helm install flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system
```

> [!NOTE]
> The Flux Operator can be installed using Helm, OperatorHub, kubectl and other methods.
> For more information, refer to the
> [installation guide](https://fluxcd.control-plane.io/operator/install/).

### Install the Flux Controllers

Create a [FluxInstance](https://fluxcd.control-plane.io/operator/fluxinstance/) resource
in the `flux-system` namespace to install the latest Flux stable version:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "1h"
    fluxcd.controlplane.io/reconcileTimeout: "5m"
spec:
  distribution:
    version: "2.x"
    registry: "ghcr.io/fluxcd"
    artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - image-reflector-controller
    - image-automation-controller
  cluster:
    type: kubernetes
    multitenant: false
    networkPolicy: true
    domain: "cluster.local"
  kustomize:
    patches:
      - target:
          kind: Deployment
          name: "(kustomize-controller|helm-controller)"
        patch: |
          - op: add
            path: /spec/template/spec/containers/0/args/-
            value: --concurrent=10
          - op: add
            path: /spec/template/spec/containers/0/args/-
            value: --requeue-dependency=5s
```

> [!NOTE]
> The Flux instance can be customized in various ways.
> For more information, refer to the
> [configuration guide](https://fluxcd.control-plane.io/operator/flux-config/).

### Sync from a Git Repository

To sync the cluster state from a Git repository, add the following configuration to the `FluxInstance`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  sync:
    kind: GitRepository
    url: "https://github.com/my-org/my-fleet.git"
    ref: "refs/heads/main"
    path: "clusters/my-cluster"
    pullSecret: "flux-system"
```

If the source repository is private, the Kubernetes secret must be created in the `flux-system` namespace
and should contain the credentials to clone the repository:

```shell
flux create secret git flux-system \
  --url=https://github.com/my-org/my-fleet.git \
  --username=git \
  --password=$GITHUB_TOKEN
```

> [!NOTE]
> For more information on how to configure syncing from Git repositories,
> container registries and S3-compatible storage, refer to the
> [cluster sync guide](https://fluxcd.control-plane.io/operator/flux-sync/).

## License

The Flux Operator is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

The project is developed by CNCF Flux core maintainers part of the [ControlPlane](https://control-plane.io) team.

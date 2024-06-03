# flux-operator

[![release](https://img.shields.io/github/release/controlplaneio-fluxcd/flux-operator/all.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/releases)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/flux-operator)](https://artifacthub.io/packages/helm/flux-operator/flux-operator)
[![e2e](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml)
[![license](https://img.shields.io/github/license/controlplaneio-fluxcd/flux-operator.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](#supply-chain-security)

The Flux Operator is a Kubernetes CRD controller that manages
the lifecycle of CNCF [Flux CD](https://fluxcd.io) and the
[ControlPlane enterprise distribution](https://github.com/controlplaneio-fluxcd/distribution).

> [!IMPORTANT]
> Note that this project in under active development.
> The APIs and features specs are described in
> [RFC-0001](https://github.com/controlplaneio-fluxcd/distribution/tree/main/rfcs/0001-flux-operator/README.md).

## Features

- Provide a declarative API for the installation and upgrade of the Flux distribution.
- Automate patching for hotfixes and CVEs affecting the Flux controllers container images.
- Provide first-class support for OpenShift, Azure, AWS, GCP and other marketplaces.
- Simplify the configuration of multi-tenancy lockdown on shared Kubernetes clusters.
- Provide a security-first approach to the Flux deployment and FIPS compliance.
- Incorporate best practices for running Flux at scale with persistent storage, sharding and horizontal scaling.
- Manage the update of Flux custom resources and prevent disruption during the upgrade process.
- Facilitate a clean uninstall and reinstall process without affecting the Flux-managed workloads.

## Installation

The Flux Operator can be installed using the
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator)
available in the ControlPlane registry:

```shell
helm install flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system \
  --create-namespace
```

Or by using the Kubernetes manifests published on the releases page:

```shell
kubectl apply -f https://github.com/controlplaneio-fluxcd/flux-operator/releases/latest/download/install.yaml
```

## Usage

The Flux Operator comes with a Kubernetes CRD called `FluxInstance`. A single custom resource of this kind
can exist in a Kubernetes cluster with the name `flux` that must be created in the same
namespace where the operator is deployed.

> [!NOTE]
> The `FluxInstance` API documentation is available at
> [docs/api/v1](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/docs/api/v1/fluxinstance.md).

### Upstream Distribution

To install the upstream distribution of Flux, create the following `FluxInstance` resource:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - image-reflector-controller
    - image-automation-controller
  cluster:
    type: kubernetes
    networkPolicy: true
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

The operator will reconcile the `FluxInstance` resource and install
the latest Flux stable version with the specified components.

### Enterprise Distribution

To install the FIPS-compliant distribution of Flux, create the following `FluxInstance` resource:

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
    version: "2.3.x"
    registry: "ghcr.io/controlplaneio-fluxcd/distroless"
    imagePullSecret: "flux-enterprise-auth"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
  cluster:
    type: openshift
    multitenant: true
    networkPolicy: true
    domain: "cluster.local"
  storage:
    class: "standard"
    size: "10Gi"
```

Every hour, the operator will check for updates in the ControlPlane
[distribution repository](https://github.com/controlplaneio-fluxcd/distribution).
If a new patch version is available, the operator will update the Flux components by pinning the
container images to the latest digest published in the ControlPlane registry.

Note that the `flux-enterprise-auth` Kubernetes secret must be created in the `flux-system` namespace
and should contain the credentials to pull the enterprise images:

```shell
kubectl create secret docker-registry flux-enterprise-auth \
  --namespace flux-system \
  --docker-server=ghcr.io \
  --docker-username=flux \
  --docker-password=$ENTERPRISE_TOKEN
```

### Migration of a bootstrap cluster

To migrate a cluster that was bootstrapped, after the flux-operator is installed
and the `FluxInstance` resource is created, the following steps are required:

1. Checkout the branch of the Flux repository that was used to bootstrap the cluster.
2. Replace the contents of the `flux-system/gok-components.yaml` with the `FluxInstance` YAML manifest.
3. Remove all controllers patches from the `flux-system/kustomization.yaml`.
4. Commit and push the changes to the Flux repository.

## Supply Chain Security

The build, release and provenance portions of the ControlPlane distribution supply chain meet
[SLSA Build Level 3](https://slsa.dev/spec/v1.0/levels).

### Software Bill of Materials

The ControlPlane images come with SBOMs in SPDX format for each CPU architecture.

Example of extracting the SBOM from the flux-operator image:

```shell
docker buildx imagetools inspect \
    ghcr.io/controlplaneio-fluxcd/flux-operator:v0.2.0 \
    --format "{{ json (index .SBOM \"linux/amd64\").SPDX}}"
```

### Signature Verification

The ControlPlane images are signed using Sigstore Cosign and GitHub OIDC.

Example of verifying the signature of the flux-operator image:

```shell
cosign verify ghcr.io/controlplaneio-fluxcd/flux-operator:v0.2.0 \
  --certificate-identity-regexp=^https://github\\.com/controlplaneio-fluxcd/.*$ \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

### SLSA Provenance Verification

The provenance attestations are generated at build time with Docker Buildkit and
include facts about the build process such as:

- Build timestamps
- Build parameters and environment
- Version control metadata
- Source code details
- Materials (files, scripts) consumed during the build

Example of extracting the SLSA provenance JSON for the flux-operator image:

```shell
docker buildx imagetools inspect \
  ghcr.io/controlplaneio-fluxcd/flux-operator:v0.2.0 \
  --format "{{ json (index .Provenance \"linux/amd64\").SLSA}}"
```

The provenance of the build artifacts is generated with the official
[SLSA GitHub Generator](https://github.com/slsa-framework/slsa-github-generator).

Example of verifying the provenance of the flux-operator image:

```shell
cosign verify-attestation --type slsaprovenance \
  --certificate-identity-regexp=^https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml.*$ \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  ghcr.io/controlplaneio-fluxcd/flux-operator:v0.2.0
```

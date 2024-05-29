# flux-operator

[![release](https://img.shields.io/github/release/controlplaneio-fluxcd/flux-operator/all.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/releases)
[![e2e](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml)
[![license](https://img.shields.io/github/license/controlplaneio-fluxcd/flux-operator.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](#supply-chain-security)

The Flux Operator is a Kubernetes CRD controller that manages
the lifecycle of the [Flux CD](https://fluxcd.io) distribution.

> [!IMPORTANT]
> Note that this project in under active development.
> The APIs may change in a backwards incompatible manner.

## Features

- Provide a declarative API for the installation and upgrade of CNCF Flux and the [ControlPlane enterprise distribution](https://github.com/controlplaneio-fluxcd/distribution).
- Automate patching for hotfixes and CVEs affecting the Flux controllers container images.
- Provide first-class support for OpenShift, Azure, AWS, GCP and other marketplaces.
- Simplify the configuration of multi-tenancy lockdown on shared Kubernetes clusters.
- Provide a security-first approach to the Flux deployment and FIPS compliance.
- Incorporate best practices for running Flux at scale with persistent storage, sharding and horizontal scaling.
- Manage the update of Flux custom resources and prevent disruption during the upgrade process.
- Facilitate a clean uninstall and reinstall process without affecting the Flux-managed workloads.

## Installation

The Flux Operator can be installed using the Kubernetes manifests published on the releases page:

```shell
kubectl apply -f https://github.com/controlplaneio-fluxcd/flux-operator/releases/latest/download/install.yaml
```

## Usage

The Flux Operator comes with a Kubernetes CRD called `FluxInstance`. A single custom resource of this kind
can exist in a Kubernetes cluster with the name `flux` that must be created in the same
namespace where the operator is deployed.

The following is an example of a `FluxInstance` custom resource:

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
    type: openshift
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

## Supply Chain Security

The build, release and provenance portions of the ControlPlane distribution supply chain meet
[SLSA Build Level 3](https://slsa.dev/spec/v1.0/levels).

### Software Bill of Materials

The ControlPlane images come with SBOMs in SPDX format for each CPU architecture.

Example of extracting the SBOM from the flux-operator image:

```shell
docker buildx imagetools inspect \
    ghcr.io/controlplaneio-fluxcd/flux-operator:v0.0.2 \
    --format "{{ json (index .SBOM \"linux/amd64\").SPDX}}"
```

### Signature Verification

The ControlPlane images are signed using Sigstore Cosign and GitHub OIDC.

Example of verifying the signature of the flux-operator image:

```shell
cosign verify ghcr.io/controlplaneio-fluxcd/flux-operator:v0.0.2 \
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
  ghcr.io/controlplaneio-fluxcd/flux-operator:v0.0.2 \
  --format "{{ json (index .Provenance \"linux/amd64\").SLSA}}"
```

The provenance of the build artifacts is generated with the official
[SLSA GitHub Generator](https://github.com/slsa-framework/slsa-github-generator).

Example of verifying the provenance of the flux-operator image:

```shell
cosign verify-attestation --type slsaprovenance \
  --certificate-identity-regexp=^https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml.*$ \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  ghcr.io/controlplaneio-fluxcd/flux-operator:v0.0.2
```

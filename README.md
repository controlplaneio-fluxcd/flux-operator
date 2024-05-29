# flux-operator

[![e2e](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml)

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
          labelSelector: "app.kubernetes.io/component in (kustomize-controller, helm-controller)"
        patch: |
          - op: add
            path: /spec/template/spec/containers/0/args/-
            value: --concurrent=10
          - op: add
            path: /spec/template/spec/containers/0/args/-
            value: --requeue-dependency=10s
```

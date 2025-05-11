# flux-operator

[![release](https://img.shields.io/github/release/controlplaneio-fluxcd/flux-operator/all.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/releases)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/flux-operator)](https://artifacthub.io/packages/helm/flux-operator/flux-operator)
[![Operator Hub](https://img.shields.io/badge/Operator_Hub-flux--operator-9cf.svg)](https://operatorhub.io/operator/flux-operator)
[![e2e](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml)
[![license](https://img.shields.io/github/license/controlplaneio-fluxcd/flux-operator.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](https://fluxcd.control-plane.io/distribution/security/)

The Flux Operator is a Kubernetes CRD controller that manages
the lifecycle of CNCF [Flux CD](https://fluxcd.io) and the
[ControlPlane enterprise distribution](https://github.com/controlplaneio-fluxcd/distribution). The operator extends Flux with self-service
capabilities and preview environments for GitLab and GitHub pull requests testing.

## Features

**Autopilot for Flux CD** - The operator offers an alternative to the Flux Bootstrap procedure, it
removes the operational burden of managing Flux across fleets of clusters by fully automating the
installation, configuration, and upgrade of the Flux controllers based on a declarative API.

**Advanced Configuration** - The operator simplifies the configuration of Flux multi-tenancy lockdown,
sharding, horizontal and vertical scaling, persistent storage, and allows fine-tuning the Flux
controllers with Kustomize patches. The operator streamlines the transition from Git as the delivery
mechanism for the cluster desired state to OCI artifacts and S3-compatible storage.

**Deep Insights** - The operator provides deep insights into the delivery pipelines managed by Flux,
including detailed reports and Prometheus metrics about the Flux controllers
readiness status, reconcilers statistics, and cluster state synchronization.

**Self-Service Environments** - The operator [ResourceSet API](https://fluxcd.control-plane.io/operator/resourcesets/introduction/)
enables platform teams to define their own application standard as a group of Flux and Kubernetes resources
that can be templated, parameterized and deployed as a single unit on self-service environments.
The ResourceSet API integrates with GitLab and GitHub pull requests to create ephemeral environments
for testing and validation.

**AI-Assisted GitOps** - The [Flux MCP Server](https://fluxcd.control-plane.io/mcp/) connects
AI assistants to Kubernetes clusters running the operator, enabling seamless interaction
through natural language. It serves as a bridge between AI tools and GitOps pipelines,
allowing you to analyze deployment across environments, troubleshoot issues,
and perform operations using conversational prompts.

**Enterprise Support** - The operator is a key component of the ControlPlane
[Enterprise offering](https://fluxcd.control-plane.io/pricing/), and is designed to automate the
rollout of new Flux versions, CVE patches and hotfixes to production environments in a secure and reliable way.
The operator is end-to-end tested along with the ControlPlane Flux distribution on
Red Hat OpenShift, Amazon EKS, Azure AKS and Google GKE.

## Quickstart Guide

### Install the Flux Operator

Install the Flux Operator in the `flux-system` namespace, for example using Helm:

```shell
helm install flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system
```

> [!NOTE]
> The Flux Operator can be installed using Helm, Terraform, OperatorHub, kubectl and other methods.
> For more information, refer to the
> [installation guide](https://fluxcd.control-plane.io/operator/install/).

### Install the Flux Controllers

Create a [FluxInstance](https://fluxcd.control-plane.io/operator/fluxinstance/) resource
named `flux` in the `flux-system` namespace to install the latest Flux stable version:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "1h"
    fluxcd.controlplane.io/reconcileArtifactEvery: "10m"
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
  # distribution omitted for brevity
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

### Monitor the Flux Installation

To monitor the Flux deployment status, check the
[FluxReport](https://fluxcd.control-plane.io/operator/fluxreport/)
resource in the `flux-system` namespace:

```shell
kubectl get fluxreport/flux -n flux-system -o yaml
```

The report is update at regular intervals and contains information about the deployment
readiness status, the distribution details, reconcilers statistics, Flux CRDs versions,
the cluster sync status and more.

## ResourceSet APIs

The Flux Operator [ResourceSet APIs](https://fluxcd.control-plane.io/operator/resourcesets/introduction/)
offer a high-level abstraction for defining and managing Flux resources and related Kubernetes
objects as a single unit.
The ResourceSet API is designed to reduce the complexity of GitOps workflows and to
enable self-service for developers and platform teams.

Guides:

- [Using ResourceSets for Application Definitions](https://fluxcd.control-plane.io/operator/resourcesets/app-definition/)
- [Ephemeral Environments for GitHub Pull Requests](https://fluxcd.control-plane.io/operator/resourcesets/github-pull-requests/)
- [Ephemeral Environments for GitLab Merge Requests](https://fluxcd.control-plane.io/operator/resourcesets/gitlab-merge-requests/)

## Documentation

- Installation
  - [Flux operator installation](https://fluxcd.control-plane.io/operator/install/)
- Flux Configuration
  - [Flux controllers configuration](https://fluxcd.control-plane.io/operator/flux-config/)
  - [Flux instance customization](https://fluxcd.control-plane.io/operator/flux-kustomize/)
  - [Cluster sync configuration](https://fluxcd.control-plane.io/operator/flux-sync/)
  - [Flux controllers sharding](https://fluxcd.control-plane.io/operator/flux-sharding/)
  - [Flux monitoring and reporting](https://fluxcd.control-plane.io/operator/monitoring/)
  - [Migration of bootstrapped clusters](https://fluxcd.control-plane.io/operator/flux-bootstrap-migration/)
- CRD references
  - [FluxInstance API reference](https://fluxcd.control-plane.io/operator/fluxinstance/)
  - [FluxReport API reference](https://fluxcd.control-plane.io/operator/fluxreport/)
  - [ResourceSet API reference](https://fluxcd.control-plane.io/operator/resourceset/)
  - [ResourceSetInputProvider API reference](https://fluxcd.control-plane.io/operator/resourcesetinputprovider/)

## License

The Flux Operator is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

The project is developed by CNCF Flux core maintainers part of the [ControlPlane](https://control-plane.io) team.

# flux-operator

[![release](https://img.shields.io/github/release/controlplaneio-fluxcd/flux-operator/all.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/releases)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/flux-operator)](https://artifacthub.io/packages/helm/flux-operator/flux-operator)
[![Operator Hub](https://img.shields.io/badge/Operator_Hub-flux--operator-9cf.svg)](https://operatorhub.io/operator/flux-operator)
[![e2e](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/actions/workflows/e2e.yaml)
[![license](https://img.shields.io/github/license/controlplaneio-fluxcd/flux-operator.svg)](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](https://fluxcd.control-plane.io/distribution/security/)

The Flux Operator is a Kubernetes CRD controller that manages
the lifecycle of CNCF [Flux CD](https://fluxcd.io) and the [ControlPlane enterprise distribution](https://github.com/controlplaneio-fluxcd/distribution).
The operator extends Flux with self-service capabilities, deployment windows,
and preview environments for GitHub, GitLab and Azure DevOps pull requests testing.

---

<p align="center">
  <a href="https://fluxoperator.dev">
    <img src="https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/logo/flux-operator-banner.png" width="100%">
  </a>
</p>

---

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
The [Flux Web UI](https://fluxoperator.dev/web-ui/) offers a real-time view of the GitOps pipelines,
allowing you to monitor deployments, track reconciliation status, and troubleshoot issues.

**Self-Service Environments** - The operator [ResourceSet API](https://fluxoperator.dev/docs/resourcesets/introduction/)
enables platform teams to define their own application standard as a group of Flux and Kubernetes resources
that can be templated, parameterized and deployed as a single unit on self-service environments.
The ResourceSet API integrates with Git pull requests to create ephemeral environments
for testing and validation.

**AI-Assisted GitOps** - The [Flux MCP Server](https://fluxoperator.dev/mcp-server/) connects
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
> [installation guide](https://fluxoperator.dev/docs/guides/install/).

### Install the Flux Controllers

Create a [FluxInstance](https://fluxoperator.dev/docs/crd/fluxinstance/) resource
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
    size: medium
    multitenant: false
    networkPolicy: true
    domain: "cluster.local"
  kustomize:
    patches:
      - target:
          kind: Deployment
        patch: |
          - op: replace
            path: /spec/template/spec/nodeSelector
            value:
              kubernetes.io/os: linux
          - op: add
            path: /spec/template/spec/tolerations
            value:
              - key: "CriticalAddonsOnly"
                operator: "Exists"
```

> [!NOTE]
> The Flux instance can be customized in various ways.
> For more information, refer to the
> [configuration guide](https://fluxoperator.dev/docs/instance/controllers/).

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
> [cluster sync guide](https://fluxoperator.dev/docs/instance/sync/).

### Monitor the Flux Installation

To monitor the Flux deployment status, check the
[FluxReport](https://fluxoperator.dev/docs/crd/fluxreport/)
resource in the `flux-system` namespace:

```shell
kubectl get fluxreport/flux -n flux-system -o yaml
```

The report is update at regular intervals and contains information about the deployment
readiness status, the distribution details, reconcilers statistics, Flux CRDs versions,
the cluster sync status and more.

### Access the Flux Web UI

To access the [Flux Web UI](https://fluxoperator.dev/web-ui/),
you can port-forward the operator service:

```shell
kubectl -n flux-system port-forward svc/flux-operator 9080:9080
```

Note that the Flux Web UI can be configured with [Ingress](https://fluxoperator.dev/docs/web-ui/ingress/)
and [Single Sign-On](https://fluxoperator.dev/docs/web-ui/user-management/) for secure external access.

## ResourceSet APIs

The Flux Operator [ResourceSet APIs](https://fluxoperator.dev/docs/resourcesets/introduction/)
offer a high-level abstraction for defining and managing Flux resources and related Kubernetes
objects as a single unit.
The ResourceSet API is designed to reduce the complexity of GitOps workflows and to
enable self-service for developers and platform teams.

Guides:

- [Using ResourceSets for Application Definitions](https://fluxoperator.dev/docs/resourcesets/app-definition/)
- [Using ResourceSets for Time-Based Delivery](https://fluxoperator.dev/docs/resourcesets/time-based-delivery/)
- [Ephemeral Environments for GitHub Pull Requests](https://fluxoperator.dev/docs/resourcesets/github-pull-requests/)
- [Ephemeral Environments for GitLab Merge Requests](https://fluxoperator.dev/docs/resourcesets/gitlab-merge-requests/)

## Documentation

- Installation
  - [Flux Operator installation](https://fluxoperator.dev/docs/guides/install/)
  - [Migration of bootstrapped clusters](https://fluxoperator.dev/docs/guides/migration/)
  - [Flux Operator CLI](https://fluxoperator.dev/docs/guides/cli/)
- Flux Configuration
  - [Flux controllers configuration](https://fluxoperator.dev/docs/instance/controllers/)
  - [Flux instance customization](https://fluxoperator.dev/docs/instance/customization/)
  - [Cluster sync configuration](https://fluxoperator.dev/docs/instance/sync/)
  - [Flux controllers sharding](https://fluxoperator.dev/docs/instance/sharding/)
  - [Flux monitoring and reporting](https://fluxoperator.dev/docs/instance/monitoring/)
- CRD references
  - [FluxInstance API reference](https://fluxoperator.dev/docs/crd/fluxinstance/)
  - [FluxReport API reference](https://fluxoperator.dev/docs/crd/fluxreport/)
  - [ResourceSet API reference](https://fluxoperator.dev/docs/crd/resourceset/)
  - [ResourceSetInputProvider API reference](https://fluxoperator.dev/docs/crd/resourcesetinputprovider/)

## Contributing

We welcome contributions to the Flux Operator project via GitHub pull requests.
Please see the [CONTRIBUTING](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/CONTRIBUTING.md)
guide for details on how to set up your development environment and start contributing to the project.

## License

The Flux Operator is an open-source project licensed under the
[AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).

The project is developed by CNCF Flux core maintainers part of the [ControlPlane](https://control-plane.io) team.

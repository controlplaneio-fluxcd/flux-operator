---
title: Installation
description: Installing Flux Operator on Kubernetes and OpenShift clusters
---

# Flux Operator Installation

The Flux Operator is designed to run in a Kubernetes cluster on Linux nodes (AMD64 or ARM64)
and is compatible with all major Kubernetes distributions. The operator is written in Go and
statically compiled as a single binary with no external dependencies.

## Install methods

The Flux Operator can be installed with Helm, Terraform, Operator Lifecycle Manager (OLM),
the `flux-operator` CLI, or `kubectl`.
It is recommended to install the operator in a dedicated namespace, such as `flux-system`.

### Helm

The Flux Operator can be installed using the
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator)
available in GitHub Container Registry:

```shell
helm install flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system \
  --create-namespace
```

### Terraform

Installing the Flux Operator with Terraform is possible using the
[Helm provider](https://registry.terraform.io/providers/hashicorp/helm/latest/docs):

```hcl
resource "helm_release" "flux_operator" {
  name             = "flux-operator"
  namespace        = "flux-system"
  repository       = "oci://ghcr.io/controlplaneio-fluxcd/charts"
  chart            = "flux-operator"
  create_namespace = true
}

resource "helm_release" "flux_instance" {
  depends_on = [helm_release.flux_operator]

  name       = "flux"
  namespace  = "flux-system"
  repository = "oci://ghcr.io/controlplaneio-fluxcd/charts"
  chart      = "flux-instance"

  values = [
    file("values/components.yaml")
  ]
}
```

For more information of how to configure the Flux instance with Terraform,
see the Flux Operator
[terraform module example](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/config/terraform).

### Operator Lifecycle Manager

The Flux Operator can be installed on OpenShift using the bundle published on OperatorHub
at [operatorhub.io/operator/flux-operator](https://operatorhub.io/operator/flux-operator).

Example subscription manifest:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  channel: stable
  name: flux-operator
  source: operatorhubio-catalog
  sourceNamespace: olm
  config:
    env:
      - name: DEFAULT_SERVICE_ACCOUNT
        value: "flux-operator"
      - name: REPORTING_INTERVAL
        value: "30s"
```

The Flux Operator is also available in the OpenShift and OKD
[production-ready catalog](https://github.com/redhat-openshift-ecosystem/community-operators-prod).

!!! note "Environment Variables"
    
    Flux Operator supports various environment variables to customize its behavior on OpenShift.
    Please see the operator [configuration guide](operator-config.md#environment-variables) for more details.

### Flux Operator CLI

The Flux Operator can be installed using the
[flux-operator CLI](https://fluxoperator.dev/docs/guides/cli/),
which bootstraps a Kubernetes cluster with the Flux Operator and a Flux instance.

Install the CLI with Homebrew:

```shell
brew install controlplaneio-fluxcd/tap/flux-operator
```

Install the Flux Operator and a Flux instance from a configuration file:

```shell
flux-operator install -f flux-instance.yaml
```

### Kubectl

The Flux Operator can be installed with `kubectl` by
applying the Kubernetes manifests published on the releases page:

```shell
kubectl apply -f https://github.com/controlplaneio-fluxcd/flux-operator/releases/latest/download/install.yaml
```

!!! warning "Development and testing"

    This method is intended for development and testing purposes. On production environments,
    it is recommended to use [Helm](#helm) or [Terraform](#terraform).

## Uninstall

The recommended way to uninstall the Flux Operator and Flux instance is using the
[flux-operator CLI](https://fluxoperator.dev/docs/guides/cli/):

```shell
flux-operator -n flux-system uninstall --keep-namespace
```

The `uninstall` command safely removes the Flux Operator and Flux controllers
without affecting the Kubernetes objects or Helm releases reconciled by Flux.
It is safe to re-install the Flux Operator later to resume managing the existing resources.

Alternatively, you can uninstall manually by first deleting the `FluxInstance` resources:

```shell
kubectl -n flux-system delete fluxinstances --all
```

The operator will uninstall Flux from the cluster without affecting the Flux-managed workloads.

Then uninstall the Flux Operator with your preferred method, e.g. Helm:

```shell
helm -n flux-system uninstall flux-operator
```

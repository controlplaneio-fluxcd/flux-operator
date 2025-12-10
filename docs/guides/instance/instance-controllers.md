---
title: Flux Controllers Configuration
description: Flux Operator configuration guide for FluxCD controllers
---

# Flux Controllers Configuration

The Flux Operator comes with a Kubernetes CRD called [FluxInstance](fluxinstance.md).
A single custom resource of this kind can exist in a Kubernetes cluster with the name
**flux** that must be created in the same namespace where the operator is deployed.

The `FluxInstance` resource is used to install and configure the automated update
of the Flux distribution.

## Default configuration

Example of a minimal `FluxInstance` resource:

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
    artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
  cluster:
    type: kubernetes
    size: medium
```

Save the above manifest to a file and apply it with `kubectl`:

```shell
kubectl apply -f flux-instance.yaml
```

The operator will reconcile the `FluxInstance` resource and install
the latest upstream Flux version in the `2.7` range with the specified components.
To verify the installation status:

```shell
kubectl -n flux-system get fluxinstance flux
```

Every hour, the operator will check for Flux patch releases and apply them if available.
To make the operator check for updates immediately:

```shell
kubectl -n flux-system annotate --overwrite \
  fluxinstance flux reconcile.fluxcd.io/requestedAt="$(date +%s)"
```

To uninstall the Flux instance:

```shell
kubectl -n flux-system delete fluxinstance flux
```

## Enterprise Distribution configuration

To deploy the enterprise distribution of Flux, point the operator to the ControlPlane registry:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/controlplaneio-fluxcd/distroless"
    imagePullSecret: "flux-enterprise-auth"
    artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
```

!!! tip "Automated CVE patching"

    The operator will check for updates to the ControlPlane distribution by pulling
    the OCI artifact from `ghcr.io/controlplaneio-fluxcd` registry every hour.
    If a new patch version is available, the operator will update the Flux components by pinning the
    container images to the latest digest published in the ControlPlane registry.

To access the ControlPlane registry, the `flux-enterprise-auth` Kubernetes secret must be
created in the `flux-system` namespace and should contain the credentials to pull the enterprise images:

```shell
kubectl create secret docker-registry flux-enterprise-auth \
  --namespace flux-system \
  --docker-server=ghcr.io \
  --docker-username=flux \
  --docker-password=$ENTERPRISE_TOKEN
```

## Custom configuration

The Flux distribution can be customized by specifying the components to install,
the cluster type, multitenancy, network policy, storage class and size, and kustomize patches.

For example, to install the latest Flux version with the multi-tenancy lockdown enabled
and persistent storage for the source-controller:

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
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
    artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - image-reflector-controller
    - image-automation-controller
    - source-watcher
  cluster:
    type: kubernetes
    size: large
    multitenant: true
    networkPolicy: true
    domain: "cluster.local"
  storage:
    class: "standard"
    size: "10Gi"
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

To find out more about the available configuration options, refer to the
[FluxInstance API reference](fluxinstance.md).


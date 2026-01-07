---
title: Flux Web UI Standalone Install
description: Flux Status Web UI setup guide for standalone installation.
---

# Flux Web UI Standalone Installation

This guide will walk you through the steps to set up the Flux Status Web UI as a dedicated deployment
in your Kubernetes cluster, separate from the Flux Operator. This setup is useful for environments
where you want to isolate the Web UI from the main Flux Operator installation.

## Prerequisites

The Kubernetes cluster should have the following components:

- Flux Operator installed in the `flux-system` namespace.
- A [FluxInstance](fluxinstance.md) deployed in the `flux-system` namespace.

## Install with Helm

You can install the Flux Web UI using the Flux Operator [Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator),
by creating a dedicated Helm release with `web.serverOnly` enabled and `installCRDs` disabled.
This ensures that only the Web UI components are installed, without affecting the existing Flux Operator installation.

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web
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
      releaseName: flux-web
      serviceAccountName: flux-operator
      chartRef:
        kind: OCIRepository
        name: << inputs.provider.name >>
      values:
        fullnameOverride: flux-web
        installCRDs: false
        web:
          serverOnly: true
```

!!! warning "Deploy on cluster not managed by Flux Operator"

    If you want to install the Web UI on clusters bootstrapped with the Flux CLI,
    you must set `installCRDs: true` as the Flux Operator CRDs are required for the Web UI to function.
    Note that we officially support installing the Web UI only on clusters managed by Flux Operator.

## Access the Web UI

Once the Helm release is applied, the Flux Web UI will be deployed in the `flux-system` namespace.
You can access the Web UI on `http://localhost:9080` by port-forwarding the service:

```shell
kubectl -n flux-system port-forward svc/flux-web 9080:9080
```

To expose the Web UI externally, you can configure an Ingress or Gateway API resource as described in the
[Web Ingress documentation](web-ingress.md). Make sure to secure the Web UI with TLS and
[Single Sign-On](web-user-management.md#single-sign-on).

---
title: Flux Web UI Standalone Install
description: Flux Status Web UI setup guide for standalone installation.
---

# Flux Web UI Standalone Installation

This guide will walk you through the steps to set up the Flux Status Web UI as a dedicated deployment
in your Kubernetes cluster, separate from the Flux Operator. This setup is useful for OpenShift clusters
or environments where you want to isolate the Web UI from the main Flux Operator installation.

## Prerequisites

The Kubernetes cluster should have the following components:

- Flux Operator installed in the `flux-system` namespace.
- A [FluxInstance](fluxinstance.md) deployed in the `flux-system` namespace.

## Client ID and Secret

First, create a Kubernetes Secret to store the client ID and client secret that Flux Operator
will use to authenticate with Dex. We'll use `flux-web` as the client ID and
generate a random client secret using `openssl`:

```bash
kubectl create secret generic flux-web-client \
  --from-literal=client-id=flux-web \
  --from-literal=client-secret=$(openssl rand -hex 32) \
  -n flux-system
```

Note that in a production setup, you should either store the Kubernetes Secret encrypted in Git for Flux to sync it,
or use an external secret management solution with external-secrets to inject the secret into the cluster.

## Dex Configuration

We'll deploy Dex using a [ResourceSet](resourceset.md) that generates a Helm release and the necessary resources:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: dex
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "30m"
    fluxcd.controlplane.io/reconcileTimeout: "5m"
spec:
  wait: true
  inputs:
    - domain: "example.com"
      ingressClass: "nginx"
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: dex
    - apiVersion: v1
      kind: Secret
      metadata:
        name: flux-web-client
        namespace: dex
        annotations:
          fluxcd.controlplane.io/copyFrom: "flux-system/flux-web-client"
    - apiVersion: v1
      kind: Secret
      metadata:
        name: cluster-tls
        namespace: dex
        annotations:
          fluxcd.controlplane.io/copyFrom: "flux-system/cluster-tls"
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: HelmRepository
      metadata:
        name: dex
        namespace: dex
      spec:
        interval: 1h
        url: https://charts.dexidp.io
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: dex
        namespace: dex
      spec:
        chart:
          spec:
            chart: dex
            version: '*'
            sourceRef:
              kind: HelmRepository
              name: dex
        interval: 1h
        releaseName: dex
        values:
          config:
            issuer: https://dex.<< inputs.issuer >>
            storage:
              type: memory
            staticClients:
              - id: flux-web
                redirectURIs:
                  - https://flux.<< inputs.domain >>/oauth2/callback
            enablePasswordDB: true
            staticPasswords:
              - email: "admin@<< inputs.domain >>"
                # bcrypt hash of the string "password": $(echo password | htpasswd -BinC 10 admin | cut -d: -f2)
                hash: "$2y$10$KR7JHCQ1BxNAKBOR/ixKqevGKtvtZnpgwvV/jF80eN5zLHVHx24E2"
                username: "admin"
                userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
          ingress:
            enabled: true
            className: << inputs.ingressClass >>
            hosts:
              - host: dex.<< inputs.domain >>
                paths:
                  - path: /
                    pathType: ImplementationSpecific
            tls:
              - secretName: cluster-tls
                hosts:
                  - "*.<< inputs.domain >>"
        valuesFrom:
          - kind: Secret
            name: flux-web-client
            valuesKey: client-id
            targetPath: config.staticClients[0].name
          - kind: Secret
            name: flux-web-client
            valuesKey: client-secret
            targetPath: config.staticClients[0].secret
```

Make sure to replace the inputs with your actual domain name and Ingress class name.

Note that we assume that a wildcard TLS certificate for your domain has been provisioned
in the `flux-system` namespace with the name `cluster-tls`.

After creating the ResourceSet, Dex will be available at `https://dex.<your-domain>` and configured
with a local OIDC provider with a single user (`admin@<your-domain>` / `password`).

## Flux Operator Configuration

Assuming that you have already deployed the Flux Operator using the Helm chart, you can enable SSO for the Web UI
by updating the [ResourceSet](resourceset.md) as follows:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  inputs:
    - domain: "example.com"
      ingressClass: "nginx"
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
        semver: '*'
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
      values:
        web:
          config:
            baseURL: https://flux.<< inputs.domain >>
            authentication:
              type: OAuth2
              oauth2:
                provider: OIDC
                issuerURL: https://dex.<< inputs.domain >>
          ingress:
            enabled: true
            className: << inputs.ingressClass >>
            hosts:
              - host: flux.<< inputs.domain >>
                paths:
                  - path: /
                    pathType: Prefix
            tls:
              - secretName: cluster-tls
                hosts:
                  - "*.<< inputs.domain >>"
        valuesFrom:
          - kind: Secret
            name: flux-web-client
            valuesKey: client-id
            targetPath: web.config.authentication.oauth2.clientID
          - kind: Secret
            name: flux-web-client
            valuesKey: client-secret
            targetPath: web.config.authentication.oauth2.clientSecret
```

Make sure to replace the inputs with your actual domain name and Ingress class name.

After updating the [ResourceSet](resourceset.md), the Flux Web UI will be accessible at `https://flux.<your-domain>`.

# User RBAC Configuration

We can now create a ClusterRole and ClusterRoleBinding to grant the `admin@<your-domain>` user
read-only access to all resources in the cluster:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web-admin
  namespace: flux-system
spec:
  inputs:
    - domain: "example.com"
  resources:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRole
      metadata:
        name: flux-web-admin
      rules:
        - apiGroups: ["*"]
          resources: ["*"]
          verbs: ["get", "list", "watch"]
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRoleBinding
      metadata:
        name: flux-web-admin
      subjects:
        - kind: User
          name: admin@<< inputs.domain >>
          apiGroup: rbac.authorization.k8s.io
      roleRef:
        kind: ClusterRole
        name: flux-web-admin
        apiGroup: rbac.authorization.k8s.io
```

Make sure to replace the inputs with your actual domain name.

Apply the [ResourceSet](resourceset.md) and log in to the Flux Web UI using the Dex identity provider
with the `admin@<your-domain>` user. You should have access in the UI to view all resources in the cluster.

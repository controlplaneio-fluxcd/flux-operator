---
title: Flux Web UI SSO with Dex
description: Flux Status Web UI SSO guide using Dex as the identity provider.
---

# Flux Web UI SSO with Dex

Flux Operator supports Single Sign-On (SSO) for the Web UI using Dex as the identity provider.
Dex is an open-source OIDC provider that can federate multiple identity sources
such as GitHub, GitLab, Google, Microsoft, OpenShift and many others.
The complete list of supported connectors can be found in the [Dex documentation](https://dexidp.io/docs/connectors/).

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
    - domain: "<your-domain>"
      ingressClass: "<your-ingress-class>"
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
              baseURL: "https://flux.<< inputs.domain >>"
              authentication:
                type: OAuth2
                oauth2:
                  provider: OIDC
                  issuerURL: "https://dex.<< inputs.domain >>"
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

Make sure to replace the inputs with your actual domain name and Ingress class name. A similar configuration
can be applied if you're exposing the Web UI using [Gateway API](web-ingress.md#gateway-api-configuration).

Note that we assume that a wildcard TLS certificate for your domain has been provisioned
in the `flux-system` namespace with the name `cluster-tls`. You should adjust the configuration
according to your TLS setup.

## Dex Configuration with Static Users

To deploy Dex as a standalone OIDC provider with a static user:

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
    - domain: "<your-domain>"
      ingressClass: "<your-ingress-class>"
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
            version: "*"
            sourceRef:
              kind: HelmRepository
              name: dex
        interval: 1h
        releaseName: dex
        values:
          config:
            issuer: "https://dex.<< inputs.domain >>"
            storage:
              type: memory
            staticClients:
              - id: flux-web
                redirectURIs:
                  - "https://flux.<< inputs.domain >>/oauth2/callback"
            enablePasswordDB: true
            staticPasswords:
              - email: "admin@<< inputs.domain >>"
                # bcrypt hash of the string "password": $(echo password | htpasswd -BinC 10 admin | cut -d: -f2)
                hash: "$2y$10$KR7JHCQ1BxNAKBOR/ixKqevGKtvtZnpgwvV/jF80eN5zLHVHx24E2"
                username: "admin"
                userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
          ingress:
            enabled: true
            className: "<< inputs.ingressClass >>"
            hosts:
              - host: "dex.<< inputs.domain >>"
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

After creating the ResourceSet, Dex will be available at `https://dex.<your-domain>` and configured
with a local OIDC provider with a single user (`admin@<your-domain>` / `password`).

### Users RBAC Configuration

We can now create a ClusterRole and ClusterRoleBinding to grant the `admin@<your-domain>` user
read-only access to all resources in the cluster:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web-users
  namespace: flux-system
spec:
  inputs:
    - domain: "<your-domain>"
      username: "admin"
  resources:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRoleBinding
      metadata:
        name: flux-web-<< inputs.username >>
      subjects:
        - kind: User
          name: << inputs.username >>@<< inputs.domain >>
          apiGroup: rbac.authorization.k8s.io
      roleRef:
        kind: ClusterRole
        name: flux-web-admin
        apiGroup: rbac.authorization.k8s.io
```

Make sure to replace the inputs with your actual domain name.

Apply the [ResourceSet](resourceset.md) and log in to the Flux Web UI using the Dex identity provider
with the `admin@<your-domain>` user. You should have access in the UI to view all resources in the cluster.

Note that the `flux-web-admin` is a predefined role included with the Flux Operator
that grants full access to Flux resources including the ability to perform actions.
See the [user management RBAC](web-user-management.md#role-based-access-control)
section for more information about predefined roles.

## Dex Configuration with GitHub

To deploy Dex with a GitHub connector, you need to create a GitHub OAuth App in your GitHub organization.
Set the callback URL to `https://dex.<your-domain>/callback` and note down the client ID and client secret.

Create a Kubernetes Secret to store the GitHub OAuth App credentials:

```bash
kubectl create secret generic dex-github-client \
  --from-literal=client-id=<your-github-client-id> \
  --from-literal=client-secret=<your-github-client-secret> \
  -n flux-system
```

Deploy Dex with the GitHub connector:

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
    - domain: "<your-domain>"
      ingressClass: "<your-ingress-class>"
      org: "<your-github-org>"
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: dex
    - apiVersion: v1
      kind: Secret
      metadata:
        name: dex-github-client
        namespace: dex
        annotations:
          fluxcd.controlplane.io/copyFrom: "flux-system/dex-github-client"
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
            version: "*"
            sourceRef:
              kind: HelmRepository
              name: dex
        interval: 1h
        releaseName: dex
        values:
          config:
            issuer: "https://dex.<< inputs.domain >>"
            storage:
              type: memory
            staticClients:
              - id: flux-web
                redirectURIs:
                  - "https://flux.<< inputs.domain >>/oauth2/callback"
            connectors:
              - type: github
                id: github
                name: GitHub
                config:
                  redirectURI: "https://dex.<< inputs.domain >>/callback"
                  orgs:
                    - name: "<< inputs.org >>"
                  teamNameField: slug
          ingress:
            enabled: true
            className: "<< inputs.ingressClass >>"
            hosts:
              - host: "dex.<< inputs.domain >>"
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
          - kind: Secret
            name: dex-github-client
            valuesKey: client-id
            targetPath: config.connectors[0].config.clientID
          - kind: Secret
            name: dex-github-client
            valuesKey: client-secret
            targetPath: config.connectors[0].config.clientSecret
```

Make sure to set the inputs to your actual domain name, Ingress class, and GitHub organization.

It is recommended to configure Dex with [persistent storage](https://dexidp.io/docs/configuration/storage/) 
to avoid losing user sessions on restarts. On production systems, consider decreasing
the [tokens expiry](https://dexidp.io/docs/configuration/tokens/) which defaults to 24 hours,
as the ID token holds the session RBAC permissions.

### Group RBAC Configuration

Assuming that your GitHub organization has a team named `admins`, you can create a ClusterRoleBinding
to grant cluster wide access to members of that team:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web-admins
  namespace: flux-system
spec:
  inputs:
    - org: "<your-github-org>"
      team: "admins"
  resources:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRoleBinding
      metadata:
        name: flux-web-<< inputs.team >>
      subjects:
        - kind: Group
          name: "<< inputs.org >>:<< inputs.team >>"
          apiGroup: rbac.authorization.k8s.io
      roleRef:
        kind: ClusterRole
        name: flux-web-admin
        apiGroup: rbac.authorization.k8s.io
```

Assuming that your GitHub organization has dev teams that should have access to resources
in their respective namespaces:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web-devs
  namespace: flux-system
spec:
  inputs:
    - org: "<your-github-org>"
      team: "dev-team-1"
      namespace: "apps-1"
    - org: "<your-github-org>"
      team: "dev-team-2"
      namespace: "apps-2"
  resources:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: flux-web-<< inputs.team >>
        namespace: "<< inputs.namespace >>"
      subjects:
        - kind: Group
          name: "<< inputs.org >>:<< inputs.team >>"
          apiGroup: rbac.authorization.k8s.io
      roleRef:
        kind: ClusterRole
        name: flux-web-admin
        apiGroup: rbac.authorization.k8s.io
```

The above example assumes that you have two dev teams (`dev-team-1` and `dev-team-2`)
that should have access to resources in the `apps-1` and `apps-2` namespaces respectively.

## Further Reading

Dex supports a wide range of identity providers and connectors, to use GitLab, Google, Microsoft or others,
refer to the [Dex documentation](https://dexidp.io/docs/connectors/).

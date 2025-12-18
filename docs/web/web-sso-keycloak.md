---
title: Flux Web UI SSO with Keycloak
description: Flux Status Web UI SSO guide using Keycloak as the identity provider.
---

# Flux Web UI SSO with Keycloak

Flux Operator supports Single Sign-On (SSO) for the Web UI using [Keycloak](https://www.keycloak.org/)
as the identity provider.

We assume that you have already deployed Keycloak in your Kubernetes cluster
available at `https://keycloak.<your-domain>`.

## Client ID and Secret

In the Keycloak admin console, create a new client for the Flux Web UI:

1. Navigate to **Clients** and click **Create client**
2. Set the **Client type** to `OpenID Connect`
3. Set the **Client ID** to `flux-web`
4. Click **Next** and enable **Client authentication**
5. In the **Authentication flow** section, enable **Standard flow** and **Direct access grants**
6. Click **Next** and configure the access settings:
   - **Home URL**: `https://flux.<your-domain>`
   - **Valid redirect URIs**: `https://flux.<your-domain>/oauth2/callback`
7. Click **Save** to create the client
8. Go to the **Credentials** tab and copy the **Client secret**

Create a Kubernetes Secret to store the OAuth2 client ID and client secret
for the Flux Web UI in the `flux-system` namespace:

```bash
kubectl create secret generic flux-web-client \
  --from-literal=client-id=flux-web \
  --from-literal=client-secret=KEYCLOAK-CLIENT-SECRET \
  -n flux-system
```

## Flux Operator Configuration

Assuming that you have already deployed the Flux Operator using the Helm chart, you can enable SSO for the Web UI
by updating Helm values to configure OAuth2 authentication with Keycloak as the OIDC provider.

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
                  issuerURL: "https://keycloak.<< inputs.domain >>"
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

## Group RBAC Configuration

Keycloak allows you to manage user groups and assign users to those groups.
You can leverage this feature to assign RBAC roles to groups and control access to the Flux Web UI.

Assuming that your organization has a group named `admins`, you can create a ClusterRoleBinding
to grant cluster wide access to members of that group:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web-admins
  namespace: flux-system
spec:
  inputs:
    - group: "admins"
  resources:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRoleBinding
      metadata:
        name: flux-web-<< inputs.group >>
      subjects:
        - kind: Group
          name: "<< inputs.group >>"
          apiGroup: rbac.authorization.k8s.io
      roleRef:
        kind: ClusterRole
        name: admin
        apiGroup: rbac.authorization.k8s.io
```

Assuming that your organization has dev teams that should have access to resources
in their respective namespaces:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-web-devs
  namespace: flux-system
spec:
  inputs:
    - group: "dev-team-1"
      namespace: "apps-1"
    - group: "dev-team-2"
      namespace: "apps-2"
  resources:
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: flux-web-<< inputs.group >>
        namespace: "<< inputs.namespace >>"
      subjects:
        - kind: Group
          name: "<< inputs.group >>"
          apiGroup: rbac.authorization.k8s.io
      roleRef:
        kind: ClusterRole
        name: admin
        apiGroup: rbac.authorization.k8s.io
```

The above example assumes that you have two dev teams (`dev-team-1` and `dev-team-2`)
that should have access to resources in the `apps-1` and `apps-2` namespaces respectively.

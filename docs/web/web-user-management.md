---
title: Flux Web UI User Management
description: Flux Status Web UI user management guide.
---

# Flux Web UI User Management

By default, the Flux Operator exposes the Web UI without authentication,
providing read-only access to all Flux resources in the cluster.

The Web UI is strictly read-only. Users can view the current state of Flux reconcilers
and their managed resources, but cannot access sensitive data such as Kubernetes Secrets and ConfigMaps.

We recommend restricting access to the Web UI using [Single Sign-On](#single-sign-on)
and Kubernetes Role-Based Access Control (RBAC) policies.

## Anonymous Access

By default, the Web UI runs under the `flux-operator` Kubernetes service account.
Cluster admins can assign a different identity to the Web UI by creating a
Kubernetes `ClusterRoleBinding` that binds a specific user to a set of permissions.

For example, to grant read-only access to all resources in the cluster to the `flux-web` user:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: flux-web-view
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: flux-web-global-view
subjects:
  - kind: User
    name: flux-web
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: flux-web-view
  apiGroup: rbac.authorization.k8s.io
```

To assign the `flux-web` identity to the Web UI, set the following values in the Flux Operator
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator):

```yaml
web:
  config:
    authentication:
      type: Anonymous
      anonymous:
        username: flux-web
```

For more information about configuring the Web UI, see the [Web Config API](web-config-api.md) documentation.

## Single Sign-On

To restrict access to the Web UI, you can enable authentication using SSO with
OAuth 2.0 providers, like OpenID Connect (OIDC).

Assuming you have a federated OIDC provider, such as Dex or Keycloak,
you can configure the Web UI to use OAuth2 authentication by setting the following values in the Flux Operator
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator):

```yaml
web:
  config:
    baseURL: https://flux-operator.example.com
    authentication:
      type: OAuth2
      oauth2:
        provider: OIDC
        clientID: <DEX-CLIENT-ID>
        clientSecret: <DEX-CLIENT-SECRET>
        issuerURL: https://dex.example.com
```

For a complete guide on configuring SSO with Dex, see the [Flux Web UI SSO with Dex](web-sso-dex.md) documentation.

### Claims Mapping to RBAC

By default, the Flux Operator uses the `email` and `groups` claims from the identity provider
to impersonate Kubernetes users and groups, enforcing Role-Based Access Control (RBAC) policies.

!!! note "Custom claims mapping"

    The operator supports custom claims mapping to adapt to different identity providers.
    See the [Web Config API](web-config-api.md) documentation for using CEL expressions
    to parse and map claims to Kubernetes users and groups.

Cluster admins can create Kubernetes RBAC resources to grant specific permissions to users or groups.
The Flux Operator enforces these permissions when users access the Web UI, ensuring they can only view
the resources they are authorized to see.

For example, if your OIDC provider includes a `groups` claim in the user's ID token, you can create
a `ClusterRoleBinding` to grant read-only access to members of the `platform-team` group:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: flux-web-view
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: flux-web-platform-team
subjects:
  - kind: Group
    name: platform-team
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: flux-web-view
  apiGroup: rbac.authorization.k8s.io
```

Assuming there is a group named `dev-team`, you can create a `RoleBinding` to grant read-only access
to resources in the `apps` namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: flux-web-dev-team
  namespace: apps
subjects:
  - kind: Group
    name: dev-team
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: flux-web-view
  apiGroup: rbac.authorization.k8s.io
```

When a user from the `dev-team` group logs in, they can only search and view
Flux resources in the `apps` namespace. Attempting to access resources in other
namespaces will result in a "403 Forbidden" error.

Users who are not part of any group with assigned permissions will only see the main dashboard,
without access to any specific resources. The dashboard displays cluster-wide statistics
such as the number of deployed apps and the readiness status of the Flux controllers.

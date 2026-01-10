---
title: Flux Web UI User Management
description: Flux Status Web UI user management guide.
---

# Flux Web UI User Management

By default, the Flux Operator exposes the Web UI without authentication,
providing read-only access to all Flux resources in the cluster.
Users can view the current state of Flux reconcilers
and their managed resources, but cannot access sensitive data such as Kubernetes Secrets and ConfigMaps.

We recommend restricting access to the Web UI using [Single Sign-On](#single-sign-on)
and Kubernetes Role-Based Access Control (RBAC) policies.

## Anonymous Access

By default, the Web UI runs under the `flux-operator` Kubernetes service account.
Cluster admins can assign a different identity to the Web UI using the configuration options.

To assign the `flux` identity to the Web UI, set the following values in the Flux Operator
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator):

```yaml
web:
  config:
    authentication:
      type: Anonymous
      anonymous:
        username: flux
        groups:
          - flux-admin
```

To grant full permissions to the anonymous user, create the necessary RBAC resources for the `flux-admin` group:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: flux-admin
subjects:
  - kind: Group
    name: flux-admin
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: flux-web-admin
  apiGroup: rbac.authorization.k8s.io
```

Note that the `flux-web-admin` is a predefined role included with the Flux Operator
that grants full access to Flux resources including the ability to perform actions.
See the [Role-Based Access Control (RBAC)](#role-based-access-control) section
for more information about predefined roles.

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

For a complete guide on configuring SSO, see the [Flux SSO with Dex](web-sso-dex.md)
or the [SSO with Keycloak](web-sso-keycloak.md) documentation.

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
a `ClusterRoleBinding` to grant full access to members of the `platform-team` group:

```yaml
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
  name: flux-web-admin
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
  name: flux-web-user
  apiGroup: rbac.authorization.k8s.io
```

When a user from the `dev-team` group logs in, they can only search and view
Flux resources in the `apps` namespace. Attempting to access resources in other
namespaces will result in a "403 Forbidden" error. To allow the dev team to
perform actions on their resources, you can bind them to the `flux-web-admin` role instead.

Users who are not part of any group with assigned permissions will only see the main dashboard,
without access to any specific resources. The main dashboard will only display
the readiness status of the Flux controllers.

## Role-Based Access Control

The Flux Web UI relies on Kubernetes Role-Based Access Control (RBAC)
to manage user permissions and access to resources.

The Web UI impersonates the authenticated user when making requests to the Kubernetes API server.
This means that the permissions granted to a user in the Web UI are determined by
the Kubernetes RBAC policies assigned to that user or to the groups they belong to.

### Predefined Roles

The Flux Operator includes several predefined `ClusterRole` resources
that can be used to grant specific permissions to users or groups.

- `flux-web-user`: Grants read-only access to Flux resources and workloads.
- `flux-web-admin`: Grants full access to Flux resources and workloads, including the ability to trigger actions.

If you prefer to define custom permissions, you can create your own roles using the predefined roles as a reference.
To disable the creation of the standard roles, set the following values in the Flux Operator
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator):

```yaml
web:
  rbac:
    createRoles: false
```

### Flux Web User Role

The `flux-web-user` role grants read-only access to Flux resources
and is suitable for users who only need to view the state of their managed resources and workloads.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: flux-web-user
  labels:
    app.kubernetes.io/part-of: flux-operator
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs:
      - get
      - list
      - watch
```

### Flux Web Admin Role

The `flux-web-admin` role grants full access to Flux resources,
including the ability to perform actions such as triggering reconciliations
and suspending/resuming resources.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: flux-web-admin
  labels:
    app.kubernetes.io/part-of: flux-operator
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - fluxcd.controlplane.io
      - source.toolkit.fluxcd.io
      - source.extensions.fluxcd.io
      - kustomize.toolkit.fluxcd.io
      - helm.toolkit.fluxcd.io
      - image.toolkit.fluxcd.io
      - notification.toolkit.fluxcd.io
    resources: ["*"]
    verbs:
      - patch
      - reconcile
      - suspend
      - resume
```

Note that the `patch` verb is not enough to allow a user to perform actions in the Web UI.
The user also needs the `reconcile`, `suspend`, and `resume` verbs
for the respective resources. These verbs are specially defined in Flux Operator
to assign action permissions.

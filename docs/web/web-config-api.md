---
title: Flux Web UI Config API
description: Config API reference for the Flux Web UI
---

# Flux Web UI Config API

This document describes the declarative API for configuring the Flux Web UI server.

## Overview

The `Config` API supports:

- **Base URL Configuration** - The public URL of the web UI
- **Security Settings** - Control over secure/insecure cookie behavior
- **Authentication** - User authentication and authorization
    - **Anonymous** - All users share a single identity for Kubernetes RBAC
    - **OAuth2/OIDC** - Users authenticate via an OpenID Connect provider

A YAML configuration file defines the settings to be used by the web UI server.
This file must be specified with `--web-config=<path to file>` when starting the server.

Example:

```yaml
# /etc/flux-status-page/config.yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: <OIDC-CLIENT-ID>
      clientSecret: <OIDC-CLIENT-SECRET>
      issuerURL: https://dex.example.com
```

## Config API

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:

  # Base URL for constructing URLs for the web UI. Required when using OAuth2 authentication.
  baseURL: https://flux-web.example.com

  # If true, sets insecure behaviors such as HTTP cookie 'secure' field to false.
  # Use only for local development or testing.
  insecure: false # Optional, default is false.

  # User actions (optional)
  userActions:
    # Send audit events to Kubernetes and Flux's notification-controller.
    # Optional. Disabled by default. Special value ["*"] enables all actions.
    audit:
      - reconcile
      - suspend
      - resume

  # Authentication configuration (optional)
  authentication:
    type: OAuth2 # Anonymous | OAuth2

    # Duration of user sessions. Default: One week.
    sessionDuration: 24h # Optional

    # Size of the user cache in number of users. Default: 100.
    userCacheSize: 200 # Optional

    # Anonymous authentication settings (when type=Anonymous)
    anonymous:
      username: some-user
      groups:
        - some-group

    # OAuth2 authentication settings (when type=OAuth2 and oauth2.provider=OIDC)
    oauth2:
      provider: OIDC
      clientID: flux-web-client-id
      clientSecret: flux-web-client-secret
      issuerURL: https://dex.example.com
```

## User Actions

To enable user actions, you need to configure [Authentication](#authentication).

To enable audit notifications integrated with both Kubernetes Events and
Flux's `notification-controller`, set `.spec.userActions.audit` to a list
of actions to audit. When set, user actions performed in the web UI will
generate audit events that are sent to both the Kubernetes API server and
Flux's `notification-controller`. This allows administrators to track user
activities for security and compliance purposes. The special value `["*"]`
can be used to enable auditing for all supported actions.

## Authentication

Authentication controls how users are identified and what Kubernetes RBAC permissions
they have when accessing Flux resources through the web UI.

### Anonymous

Setting the authentication to `Anonymous` assigns a fixed identity to all users accessing the web UI.
All users share the same Kubernetes RBAC permissions based on the configured
username and/or groups.

At least one of `username` or `groups` must be specified.

```yaml
spec:
  authentication:
    type: Anonymous
    anonymous:
      username: flux-readonly-user
      groups:
        - flux-viewers
```

Note that anyone who can access the web UI will have the same permissions, so this
mode is only suitable for environments where the UI is accessible to trusted users
in a secure network.

### OAuth2 Authentication

OAuth2 authentication integrates with external identity providers to authenticate
users. Currently, only the OIDC (OpenID Connect) provider is supported.

When OAuth2 is configured, `spec.baseURL` must be set to enable proper redirect
handling during the OAuth2 flow.

#### OIDC Provider

The OIDC provider authenticates users via an OpenID Connect identity provider
(such as Dex, Keycloak, Okta, Auth0, or any OIDC-compliant provider). For this
provider, RBAC permissions are derived from claims in the ID token issued by
the identity provider. This means that even if groups/roles are revoked from
the user, their access to the web UI will persist until the ID token expires.
For production environments where immediate revocation is required, consider
using very short-lived tokens if supported by your identity provider. Dex,
for example, allows configuring the ID token lifetime.

```yaml
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: flux-web                          # Required: OAuth2 client ID
      clientSecret: flux-web-secret               # Required: OAuth2 client secret
      issuerURL: https://auth.example.com        # Required: OIDC issuer URL
      scopes:                                    # Optional: custom scopes to request instead of defaults
        - groups
        - email
```

The default scopes requested are `openid`, `offline_access`, `profile`, `email` and `groups`.

After successful authentication, the OIDC provider extracts user information from
the ID token claims to create a user session. This session includes:

- **Profile information** - Display name shown in the UI
- **Impersonation credentials** - Username and groups used for Kubernetes RBAC

#### Claims Processing with CEL

The OIDC provider uses Common Expression Language (CEL) expressions to extract
and validate information from ID token claims. This provides flexibility in
mapping identity provider claims to Kubernetes RBAC identities.

References for writing CEL expressions:
- [CEL Language Reference](https://cel.dev/overview/cel-overview)
- [CEL Playground](https://playcel.undistro.io)

##### Variables

Variables allow you to extract and transform claim values for reuse in other
expressions. Variables can reference previously declared variables, enabling
complex data transformations.

```yaml
spec:
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      # ... omitted for brevity ...
      variables:
        - name: username
          expression: "claims.sub"
        - name: domain
          expression: "claims.email.split('@')[1]"
        - name: departments
          expression: "claims.groups.filter(g, g.startsWith('dept:')).map(g, g.substring(5))"
```

##### Validations

Validations enforce rules on claims and variables. Each validation is a CEL
expression that must return `true` for authentication to succeed. If a validation
fails, its message is returned as an error.

```yaml
spec:
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      # ... omitted for brevity ...
      variables:
        - name: domain
          expression: "claims.email.split('@')[1]"
      validations:
        - expression: "variables.domain == 'example.com'"
          message: "email domain not allowed"
        - expression: "size(claims.groups) > 0"
          message: "user must belong to at least one group"
```

##### Profile

The profile configuration extracts display information for the UI.

```yaml
spec:
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      # ... omitted for brevity ...
      profile:
        name: "claims.name"
```

Default: `"has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')"`

##### Impersonation

Impersonation configures how the authenticated user's identity maps to Kubernetes
RBAC. The username and groups extracted here are used for Kubernetes API
impersonation when the user accesses Flux resources.

```yaml
spec:
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      # ... omitted for brevity ...
      impersonation:
        username: "claims.email"
        groups: "claims.groups"
```

Defaults:
- `username`: `"has(claims.email) ? claims.email : ''"`
- `groups`: `"has(claims.groups) ? claims.groups : []"`

At least one of `username` or `groups` must result in a non-empty value.

## Configuration Examples

### Anonymous Read-Only Access

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    type: Anonymous
    anonymous:
      username: flux-viewer
      groups:
        - flux-readonly
```

### Basic OIDC Authentication

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: flux-web
      clientSecret: my-client-secret
      issuerURL: https://dex.example.com
```

### OIDC with Custom Session Duration

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    sessionDuration: 8h
    userCacheSize: 500
    oauth2:
      provider: OIDC
      clientID: flux-web
      clientSecret: my-client-secret
      issuerURL: https://dex.example.com
```

### OIDC with Domain Validation

This example restricts access to users with emails from a specific domain:

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: flux-web
      clientSecret: my-client-secret
      issuerURL: https://dex.example.com
      variables:
        - name: domain
          expression: "claims.email.split('@')[1]"
      validations:
        - expression: "variables.domain == 'example.com'"
          message: "Only example.com emails are allowed"
```

### OIDC with Group Transformation

This example extracts department groups from prefixed claim values and uses
them for Kubernetes RBAC:

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: flux-web
      clientSecret: my-client-secret
      issuerURL: https://dex.example.com
      scopes:
        - groups
        - email
      variables:
        - name: username
          expression: "claims.sub"
        - name: departments
          expression: "claims.groups.filter(g, g.startsWith('dept:')).map(g, g.substring(5))"
      impersonation:
        username: "variables.username"
        groups: "variables.departments"
```

### OIDC with Multiple Validations

```yaml
apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://flux-web.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: flux-web
      clientSecret: my-client-secret
      issuerURL: https://dex.example.com
      variables:
        - name: email
          expression: "claims.email"
        - name: domain
          expression: "variables.email.split('@')[1]"
      validations:
        - expression: "variables.domain in ['example.com', 'corp.example.com']"
          message: "Email domain not allowed"
        - expression: "size(claims.groups) > 0"
          message: "User must belong to at least one group"
        - expression: "claims.email_verified == true"
          message: "Email must be verified"
      profile:
        name: "has(claims.name) ? claims.name : variables.email"
      impersonation:
        username: "variables.email"
        groups: "claims.groups"
```

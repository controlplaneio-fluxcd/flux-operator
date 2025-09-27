---
title: Flux MCP Server Config API
description: Config API reference for the Flux MCP Server
---

# Flux MCP Server Config API

This document describes a declarative API for configuring a subset of features
of the Flux MCP Server.

## Overview

The `Config` API supports:

- **MCP Transport Modes** - How the MCP server receives and sends messages
    - **Streamable HTTP** - HTTP transport with support for streaming responses and authentication
    - **Standard Input/Output (stdio)** - Simple transport using standard input and output streams
- **Authentication** - Secure access control for MCP server operations when running with Streamable HTTP
    - **Credentials** - How credentials are extracted from incoming requests
    - **Providers** - How extracted credentials are validated and converted to user sessions

A YAML configuration file defines the features and settings to be used by the MCP server.
This file must be specified with `--config=<path to file>` when starting the MCP server.
If this flag is not specified, the MCP server will start with default settings.

The legacy `sse` transport mode can only be specified using the `--transport=sse` flag.

Example:

```yaml
# /etc/flux-mcp/config.yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  transport: http # or stdio
  authentication:
    credentials:
      - type: BearerToken
    providers:
      - name: external
        type: OIDC
        issuerURL: "https://auth.example.com"
        audience: "https://flux-mcp.example.com"
        impersonation:
          username: "claims.sub"
        scopes:
          expression: "claims.scopes"
```

```bash
flux-operator-mcp serve --config=/etc/flux-mcp/config.yaml
```

## Config API

```yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:

  # Transport mode for MCP communication. Supported values: "http", "stdio". Default: "stdio"
  transport: http

  # If true, the MCP server will operate in read-only mode. The MCP server will run in read-only
  # mode if at least one between this field or the CLI flag --read-only is set to true.
  readonly: false # Optional, default is false.

  # Authentication configuration (optional)
  authentication:

    # Methods for extracting credentials. At least one method must be defined.
    credentials:
      - type: BearerToken
      - type: BasicAuth
      - type: CustomHTTPHeader
        headers:
          token: "X-Auth-Token"

    # Authentication providers. At least one provider must be defined.
    providers:
      - name: external
        type: OIDC
        issuerURL: "https://auth.example.com"
        audience: "https://flux-mcp.example.com"
```

## Authentication

Authentication is only supported in the Streamable HTTP MCP transport mode. When authentication is
configured, it provides secure access control with support for multiple credential extraction methods
and multiple authentication providers.

### Credentials

The field `spec.authentication.credentials` defines how credentials are extracted from
incoming HTTP requests. The first credential that successfully extracts the information
from a request is used.

#### BearerToken

Extracts a token from the `Authorization: Bearer <token>` header.

```yaml
spec:
  authentication:
    credentials:
      - type: BearerToken
```

#### BasicAuth

Extracts username and password from the `Authorization: Basic base64(<username>+":"+<password>)` header.

```yaml
spec:
  authentication:
    credentials:
      - type: BasicAuth
```

#### CustomHTTPHeader

Extracts credentials from custom HTTP headers.

```yaml
spec:
  authentication:
    credentials:
      - type: CustomHTTPHeader
        headers:
          username: "X-Username" # Header containing username (optional)
          password: "X-Password" # Header containing password (optional)
          token: "X-Auth-Token"  # Header containing token (optional)
```

### Providers

Authentication providers validate extracted credentials and extract user information
from these credentials to create a user session. Multiple providers can be configured
under `spec.authentication.providers` to support different authentication systems.
The first provider that successfully validates the extracted credentials and
successfully extracts a user session is used.

A user session consists of a username, a list of groups, and a list of scopes.

The username and groups in a user session are used for Kubernetes
impersonation. RBAC permissions are expected to be properly granted
to this username and groups separately, the MCP server is not
responsible for managing these permissions.

#### OIDC Provider

The OIDC provider validates JSON Web Tokens (JWT) against an OpenID
Connect provider. An HTTP call is made to the provider's
`/.well-known/openid-configuration` endpoint to fetch the provider's
public keys and other metadata. The public keys are used to validate
the token's signature. If the signature is valid, and the standard
claims (`iss`, `aud`, `exp`, etc.) are valid, the token is considered
valid.

If a token is considered valid, the provider proceeds to extract
the user session information from the token's claims, and then finally
to validate custom properties defined in the configuration. The extraction
and validation rules are defined using Common Expression Language (CEL)
expressions. A map with the claims of the JWT is passed to the CEL
expressions. References for writing CEL expressions:

- [CEL Language Reference](https://cel.dev/overview/cel-overview)
- [CEL Playground](https://playcel.undistro.io)

Multiple OIDC providers can be defined to support multiple OIDC providers.

Each OIDC provider under `spec.authentication.providers` supports the following configuration:

```yaml
spec:
  authentication:
    providers:
      - # Required fields
        name: external                                      # Provider name (must be unique)
        type: OIDC                                          # Provider type
        issuerURL: "https://auth.example.com"               # OIDC issuer URL for fetching public keys
        audience: "https://flux-mcp.example.com"            # Expected "aud" claim in the JWT

        # Optional fields
        variables:                                          # Named variables for reuse in other expressions
          - name: username
            expression: "claims.sub"
          - name: domain
            expression: "claims.email.split('@')[1]"
        validations:                                        # Custom claim and variables validations
          - expression: "variables.domain == 'example.com'" # CEL expression returning bool
            message: "email domain not allowed"
        impersonation:                                      # Kubernetes impersonation configuration
          username: "variables.username"                    # CEL expression returning string
          groups: "claims.groups"                           # CEL expression returning []string
        scopes:                                             # Scopes extraction
          expression: "claims.scopes"                       # CEL expression returning []string
```

##### Scopes

Scopes provide fine-grained access control for MCP operations. When configured,
the scopes extracted from authentication credentials are used by the MCP tools
to limit access to certain operations.

The scopes in a user session are used by the MCP tools individually
to limit access to certain operations. Even though a user or group
may have RBAC permissions to perform an operation in the Kubernetes
cluster, the MCP tools will deny the operation if the required
scope is not present in the user session. The opposite is
not true, i.e. having the required scope does not guarantee that
the operation will be allowed by Kubernetes RBAC.

The scopes required by each MCP tool are documented in the
[tools](tools.md#scopes-and-the-toolslist-request) reference.

## Configuration Examples

### Basic OIDC Authentication

```yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  transport: http
  authentication:
    credentials:
      - type: BearerToken
    providers:
      - name: external
        type: OIDC
        issuerURL: "https://auth.example.com"
        audience: "https://flux-mcp.example.com"
```

### Advanced OIDC with Variables and Validations

```yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  transport: http
  authentication:
    credentials:
      - type: BearerToken
      - type: BasicAuth
    providers:
      - name: external
        type: OIDC
        issuerURL: "https://auth.example.com"
        audience: "https://flux-mcp.example.com"
        variables:
          - name: email
            expression: "claims.email"
          - name: department
            expression: "claims.department"
        validations:
          - expression: "variables.email.endsWith('@example.com')"
            message: "Only example.com emails allowed"
          - expression: "variables.department in ['engineering', 'devops']"
            message: "Access restricted to engineering and devops"
        impersonation:
          username: "claims.preferred_username"
          groups: "claims.groups + ['authenticated']"
        scopes:
          expression: "claims.scopes"
```

### OIDC with Variable Referencing

This example demonstrates how variables can reference previously declared variables,
enabling complex data transformations and validations:

```yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  transport: http
  authentication:
    credentials:
      - type: BearerToken
    providers:
      - name: external
        type: OIDC
        issuerURL: "https://auth.example.com"
        audience: "https://flux-mcp.example.com"
        variables:
          - name: email
            expression: "claims.email"                  # Extract email from claims
          - name: domain
            expression: "variables.email.split('@')[1]" # Extract domain from email variable
          - name: normalized_domain
            expression: "variables.domain.lowerAscii()" # Normalize domain using previous variable
          - name: username_prefix
            expression: "variables.email.split('@')[0]" # Extract username part from email
        validations:
          - expression: "variables.normalized_domain in ['example.com', 'corp.example.com']"
            message: "Email domain not allowed"
          - expression: "size(variables.username_prefix) >= 3"
            message: "Username must be at least 3 characters"
        impersonation:
          username: "variables.email"
          groups: "['users', 'domain:' + variables.normalized_domain]"
        scopes:
          expression: "['read', 'write:' + variables.normalized_domain]"
```

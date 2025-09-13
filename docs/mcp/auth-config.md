# MCP Authentication Configuration

This document describes the authentication configuration system for the Model Context Protocol (MCP) server. The authentication system provides secure access control with support for multiple transport methods and authentication providers.

## Overview

The MCP authentication system consists of two main components:

1. **Transport Configuration** - How credentials are extracted from incoming requests
2. **Authenticator Configuration** - How extracted credentials are validated and converted to user sessions

## API Reference

The authentication configuration uses the `mcp.fluxcd.controlplane.io/v1` API group.

### AuthenticationConfiguration

The main configuration object that defines transports and authenticators.

```yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: auth-config
spec:
  # Transport methods for extracting credentials
  transports:
    - type: BearerToken
    - type: BasicAuth
    - type: CustomHTTPHeader
      headers:
        token: "X-Auth-Token"
  
  # References to authenticator configurations
  authenticators:
    - kind: OIDCAuthenticator
      name: oidc-provider
  
  # Whether to validate scopes for MCP tools (optional)
  validateScopes: true
```

#### ConfigSpec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transports` | `[]TransportSpec` | Yes | List of transport configurations for extracting credentials |
| `authenticators` | `[]ConfigObjectReference` | Yes | List of authenticator references |
| `validateScopes` | `bool` | No | Enable scope validation for MCP tools (default: false) |

## Transport Configuration

Transports define how credentials are extracted from HTTP requests. Multiple transports can be configured to support different authentication methods.

### Supported Transport Types

#### BearerToken

Extracts token from the `Authorization: Bearer <token>` header.

```yaml
transports:
  - type: BearerToken
```

#### BasicAuth

Extracts username and password from the `Authorization: Basic <credentials>` header.

```yaml
transports:
  - type: BasicAuth
```

#### CustomHTTPHeader

Extracts credentials from custom HTTP headers.

```yaml
transports:
  - type: CustomHTTPHeader
    headers:
      username: "X-Username"      # Header containing username (optional)
      password: "X-Password"      # Header containing password (optional) 
      token: "X-Auth-Token"       # Header containing token (optional)
```

### TransportSpec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | `string` | Yes | Transport type: `BearerToken`, `BasicAuth`, or `CustomHTTPHeader` |
| `headers` | `CustomHTTPHeaderSpec` | No | Custom header configuration (only for `CustomHTTPHeader` type) |

### CustomHTTPHeaderSpec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `username` | `string` | No | Name of HTTP header containing the username |
| `password` | `string` | No | Name of HTTP header containing the password |
| `token` | `string` | No | Name of HTTP header containing the token |

## Authenticator Configuration

Authenticators validate extracted credentials and create user sessions for Kubernetes impersonation.

### OIDC Authenticator

The OIDC authenticator validates JWT tokens against an OpenID Connect provider.

```yaml
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: oidc-provider
spec:
  # OIDC provider configuration
  issuerURL: "https://auth.example.com"
  clientID: "mcp-client-id"
  
  # CEL expressions for extracting user information
  username: "sub"                              # Extract username (default: "sub")
  groups: "groups"                             # Extract groups (default: "[]")
  scopes: "scopes"                             # Extract scopes (default: "[]")
  
  # CEL expressions for token validation
  assertions:
    - "'mcp-client-id' in aud"                 # Validate audience
    - "email.endsWith('@example.com')"         # Validate email domain
```

#### OIDCAuthenticatorSpec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `issuerURL` | `string` | Yes | URL of the OIDC issuer (must use HTTPS) |
| `clientID` | `string` | Yes | Client ID of the OIDC application |
| `username` | `string` | No | CEL expression to extract username (default: `"sub"`) |
| `groups` | `string` | No | CEL expression to extract groups (default: `"[]"`) |
| `scopes` | `string` | No | CEL expression to extract scopes (default: `"[]"`) |
| `assertions` | `[]string` | No | List of CEL expressions for token validation |

## CEL Expressions

The OIDC authenticator uses Common Expression Language (CEL) to extract and validate claims from JWT tokens.

### Available Variables

- `sub` - Subject claim (user identifier)
- `aud` - Audience claim (client identifier)
- `iss` - Issuer claim
- `exp` - Expiration time
- `iat` - Issued at time
- `email` - Email claim (if present)
- `groups` - Groups claim (if present)
- `scopes` - Scopes claim (if present)
- Custom claims from the JWT token

### Expression Examples

#### Username Extraction

```yaml
# Use subject claim
username: "sub"

# Use email if available, fallback to subject
username: "has(email) ? email : sub"

# Use preferred username with fallback
username: "preferred_username != '' ? preferred_username : sub"
```

#### Groups Extraction

```yaml
# Extract groups from claim
groups: "groups"

# Provide default groups if none present
groups: "groups != null ? groups : ['default']"

# Transform group names
groups: "groups.map(g, 'mcp-' + g)"
```

#### Scopes Extraction

```yaml
# Extract scopes from claim
scopes: "scopes"

# Filter specific scopes
scopes: "scopes.filter(s, s.startsWith('mcp:'))"
```

#### Token Assertions

```yaml
assertions:
  # Validate audience contains client ID
  - "'mcp-client-id' in aud"
  
  # Validate email domain
  - "email.endsWith('@example.com')"
  
  # Validate token expiration
  - "exp > now().getSeconds()"
  
  # Validate required groups
  - "size(groups.filter(g, g == 'admin')) > 0"
  
  # Validate custom claims
  - "has(department) && department == 'engineering'"
```

## Configuration Reference

### ConfigObjectReference

References to other authentication configuration objects.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `kind` | `string` | Yes | Kind of the configuration object (e.g., `OIDCAuthenticator`) |
| `name` | `string` | Yes | Name of the configuration object |

## Complete Example

```yaml
# Main authentication configuration
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: mcp-auth
spec:
  transports:
    - type: BearerToken
    - type: CustomHTTPHeader
      headers:
        token: "X-API-Token"
  authenticators:
    - kind: OIDCAuthenticator
      name: company-oidc
  validateScopes: true

---
# OIDC authenticator configuration
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: company-oidc
spec:
  issuerURL: "https://auth.company.com"
  clientID: "mcp-production"
  username: "preferred_username != '' ? preferred_username : email"
  groups: "groups != null ? groups : ['users']"
  scopes: "scopes.filter(s, s.startsWith('mcp:'))"
  assertions:
    - "'mcp-production' in aud"
    - "email.endsWith('@company.com')"
    - "'mcp:access' in scopes"
```

## Security Considerations

1. **HTTPS Only**: OIDC issuer URLs must use HTTPS to ensure secure token validation
2. **Token Validation**: Always validate audience claims to prevent token reuse across different applications
3. **Scope Validation**: Enable scope validation when using fine-grained access control
4. **Expression Security**: CEL expressions are sandboxed but should still be reviewed for correctness
5. **Token Expiration**: Assertions should validate token expiration times when required

## Troubleshooting

### Common Issues

1. **Invalid CEL Expression**: Check expression syntax and available claims in JWT tokens
2. **Authentication Failed**: Verify OIDC configuration and token claims
3. **Scope Validation Errors**: Ensure required scopes are present in user tokens
4. **Transport Errors**: Check HTTP headers and credential extraction configuration

### Debugging

Enable debug logging to see detailed authentication flow information including:
- Credential extraction results
- CEL expression evaluation
- Token validation steps
- User session creation
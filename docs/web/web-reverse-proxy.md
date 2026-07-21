---
title: Flux Web UI Reverse Proxy Authentication
description: Flux Status Web UI guide for authenticating users via an upstream reverse proxy, with Nginx and Tailscale examples.
---

# Flux Web UI Reverse Proxy Authentication

Reverse Proxy authentication lets an upstream proxy authenticate users and pass their
identity to the Flux Web UI via HTTP headers. This is useful when authentication is
already handled outside the cluster (e.g. by an Nginx reverse proxy with LDAP/basic auth,
or by a Tailscale tailnet) and you don't want to run a separate OIDC provider.

See the [Web Config API](web-config-api.md#reverse-proxy-authentication) documentation
for the full list of `reverseProxy` fields.

!!! warning "Security requirement"

    The Flux Web UI only trusts the direct TCP peer IP of the incoming connection, never
    `X-Forwarded-For` or `Forwarded` headers. This means the reverse proxy must connect
    directly to the Web UI (no untrusted hop in between), and `trustedProxies` must be
    scoped to exactly the proxy's address. Any client that can reach the Web UI directly
    and set the configured identity headers can impersonate any user, so the Web UI must
    never be reachable except through the trusted proxy.

## Nginx Reverse Proxy

This setup uses Nginx as an authenticating reverse proxy in front of the Flux Web UI.
Nginx authenticates the request (for example via `auth_request` against an LDAP/basic-auth
gateway or an external authentication service) and then injects the user's identity into
headers before proxying to the Web UI.

### Nginx configuration

```nginx
server {
    listen 443 ssl;
    server_name flux.example.com;

    # Authenticate the request against an external auth service.
    # This subrequest can implement basic auth, LDAP, or any custom login flow.
    auth_request /auth;
    auth_request_set $remote_user $upstream_http_x_auth_user;
    auth_request_set $remote_groups $upstream_http_x_auth_groups;

    location /auth {
        internal;
        proxy_pass http://auth-service.internal/verify;
        proxy_pass_request_body off;
    }

    location / {
        proxy_pass http://flux-operator.flux-system.svc.cluster.local:9080;

        # Strip any identity headers a client may have sent, then set our own
        # from the values produced by the auth_request above.
        proxy_set_header X-Remote-User $remote_user;
        proxy_set_header X-Remote-Groups $remote_groups;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

Always clear any inbound `X-Remote-User`/`X-Remote-Groups` headers before setting them
from the auth subrequest, so a malicious client cannot inject its own identity.

### Flux Web UI configuration

Set `trustedProxies` to the IP address (or CIDR) that Nginx uses to connect to the Web UI.
In Kubernetes, this is typically the pod IP/CIDR of the Nginx deployment or ingress
controller namespace:

```yaml
spec:
  authentication:
    type: ReverseProxy
    reverseProxy:
      headers:
        username: X-Remote-User
        groups: X-Remote-Groups
      trustedProxies:
        - 10.244.0.0/16 # cluster pod CIDR, or the Nginx pod's specific IP
```

If you're running Nginx as the `ingress-nginx` controller, find its pod IPs with
`kubectl get pods -n ingress-nginx -o wide` and scope `trustedProxies` as tightly as
possible (ideally to the controller's individual pod IPs, or a `NetworkPolicy`-enforced
CIDR that only the controller can reach the Web UI from).

---
title: Flux Web UI SSO with OpenShift
description: Flux Status Web UI SSO guide using OpenShift as the identity provider.
---

# Flux Web UI with OpenShift SSO

When deploying Flux Operator in OpenShift clusters through the Operator Lifecycle Manager (OLM),
the configuration for the Flux Web UI can be passed as a Kubernetes Secret through an environment
variable called `WEB_CONFIG_SECRET_NAME`.

For example:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  channel: stable
  name: flux-operator
  source: operatorhubio-catalog
  sourceNamespace: olm
  config:
    env:
      - name: WEB_CONFIG_SECRET_NAME
        value: "flux-web-config"
```

Flux Operator will watch for changes in this Secret and automatically
reconfigure the Web UI accordingly without downtime.
The Kubernetes Secret should contain a key named `config.yaml` holding the configuration
for the Flux Web UI in YAML format.

For example, to configure the Web UI with OAuth2 authentication using Dex as the OIDC provider:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: flux-web-config
  namespace: flux-system # same namespace as Flux Operator
type: Opaque
stringData:
  config.yaml: |
    apiVersion: web.fluxcd.controlplane.io/v1
    kind: Config
    spec:
      baseURL: https://flux.example.com
      authentication:
        type: OAuth2
        oauth2:
          provider: OIDC
          clientID: flux-web
          clientSecret: flux-web-secret
          issuerURL: https://dex.example.com
```

For more information on the Web UI configuration options,
refer to the [Web Config API](web-config-api.md) documentation.

## Authentication using OpenShift

If you want to use OpenShift users and groups for authentication in the Flux Web UI,
you can configure Dex with the OpenShift connector. This allows users to log in to the Web UI
using their OpenShift credentials, and their group memberships will be reflected in the RBAC policies.

For more information on setting up Dex with OpenShift,
refer to the [Dex documentation](https://dexidp.io/docs/connectors/openshift/).

## Further Reading

- [Flux Web UI SSO with Dex](./web-sso-dex.md)
- [Flux Web UI Ingress Configuration](./web-ingress.md)

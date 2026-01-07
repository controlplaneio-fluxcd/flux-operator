---
title: Flux Web UI with OpenShift OLM
description: Flux Status Web UI guide using OpenShift Operator Lifecycle Manager (OLM)
---

# Flux Web UI with OpenShift OLM

When deploying Flux Operator in OpenShift clusters through the Operator Lifecycle Manager (OLM),
the configuration for the Flux Web UI can be passed as a Kubernetes Secret through an environment
variable.

To specify the name of a Secret in the same namespace where the Flux Operator is installed,
you can use the environment variable `WEB_CONFIG_SECRET_NAME`. Flux Operator will watch for
changes in this Secret and automatically reconfigure the Web UI accordingly without downtime.

The Kubernetes Secret should contain a key named `config.yaml` holding the configuration
for the Flux Web UI in YAML format. The Web UI Config API is documented [here](./web-config-api.md).

For example:

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
      baseURL: https://flux-status.${CLUSTER_DOMAIN}
      authentication:
        type: OAuth2
        oauth2:
          provider: OIDC
          clientID: flux-web
          clientSecret: flux-web-secret
          issuerURL: https://example-oidc-provider.${CLUSTER_DOMAIN}
```

## Further Reading

If you are using Dex as your OIDC provider, you can find more information on setting up
Dex with OpenShift in the [Dex documentation](https://dexidp.io/docs/connectors/openshift/).

See also:

- [Flux Web UI SSO with Dex](./web-sso-dex.md)
- [Flux Web UI Ingress Configuration](./web-ingress.md)

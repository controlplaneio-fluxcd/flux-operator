---
title: Flux Web UI Ingress
description: Flux Status Web UI Ingress configuration guide.
---

# Flux Web UI Ingress Configuration

Flux Operator serves the Web UI on port `9080` with the name `http-web`.
This port is exposed inside the cluster by the `flux-operator` Kubernetes Service of type `ClusterIP`.

To access the Web UI from outside the cluster, you can use Ingress or Gateway API configurations.
It is recommended to secure the Web UI with TLS and [Single Sign-On](web-user-management.md#single-sign-on).

## Ingress Configuration

If the Flux Operator is deployed using its [Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator),
you can create an Ingress resource by setting the following values:

```yaml
web:
  ingress:
    enabled: true
    className: nginx
    annotations:
      cert-manager.io/cluster-issuer: letsencrypt-prod
    hosts:
      - host: flux.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - hosts:
          - flux.example.com
        secretName: flux-web-tls
```

When using other deployment methods, you can create an Ingress resource like this:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: flux-web
  namespace: flux-system
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: "nginx"
  rules:
    - host: "flux.example.com"
      http:
        paths:
          - backend:
              service:
                name: flux-operator
                port:
                  name: http-web
            path: /
            pathType: Prefix
  tls:
    - hosts:
        - "flux.example.com"
      secretName: flux-web-tls
```

Make sure to replace `nginx` with your Ingress class name
and `flux.example.com` with your actual domain name.
It is recommended to configure redirection from HTTP to HTTPS.

## Gateway API Configuration

If you are using Gateway API, you can create a `Gateway` definition
with TLS termination and a corresponding `HTTPRoute`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: flux-web
  namespace: flux-system
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: internet-gateway
      namespace: gateway-namespace
  hostnames:
    - "flux.example.com"
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: flux-operator
          namespace: flux-system
          port: 9080
```

Note the `parentRefs` section must be updated to match your `Gateway` name
and the `hostname` should be set to your own domain name.

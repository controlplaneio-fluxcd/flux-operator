---
title: Flux Web UI Ingress
description: Flux Status Web UI Ingress configuration guide.
---

# Flux Web UI Ingress Configuration

Flux Operator serves the Web UI on port `9080` with the name `http-web`.
This port is exposed inside the cluster by the `flux-operator` Kubernetes Service of type `ClusterIP`.

To access the Web UI from outside the cluster, you can use Ingress or Gateway API configurations.
It is recommended to secure the Web UI with TLS and Single Sign-On (SSO).

## Ingress Configuration

The Flux Operator [Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator)
can create an Ingress resource for the Web UI.

To enable the Ingress, set the following values:

```yaml
web:
  ingress:
    enabled: true
    className: nginx
    hosts:
      - host: flux-operator.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: flux-operator-tls
        hosts:
          - flux-operator.example.com
```

When using other deployment methods, you can create an Ingress resource like this:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  ingressClassName: nginx
  rules:
  - host: "flux-operator.example.com"
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
    - "flux-operator.example.com"
```

Make sure to replace `nginx` with your Ingress class name
and `flux-operator.example.com` with your actual domain name.
It is recommended to configure redirection from HTTP to HTTPS.

## Gateway API Configuration

If you are using Gateway API, you can create a `Gateway` definition
with TLS termination and a corresponding `HTTPRoute`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: internet-gateway
      namespace: gateway-namespace
  hostnames:
    - "flux-operator.example.com"
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

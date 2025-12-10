---
title: Flux Instance Customization
description: Flux Operator customization guide for FluxCD controllers and sources
---

# Flux Instance Customization

The [FluxInstance](fluxinstance.md) allows for the customization of the
Flux controller deployments and the Flux sync custom resources using Kustomize patches.

## Kustomize patches usage

You can make changes to all controllers using a single patch
or target a specific controller:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      # target all controller deployments
      - patch: |
          # strategic merge or JSON patch
        target:
          kind: Deployment
      # target multiple controller deployments by name
      - patch: |
          # strategic merge or JSON patch      
        target:
          kind: Deployment
          name: "(kustomize-controller|helm-controller)"
      # target a single controller service account by name
      - patch: |
          # strategic merge or JSON patch     
        target:
          kind: ServiceAccount
          name: "source-controller"
```

!!! warning "Target namespace"

    Note that the `patch.target` **must not contain** a `namespace` field, all patches are
    applied to the instance namespace.

### Verifying patches

To verify the patches, you can use The Flux Operator CLI to build the `FluxInstance`
locally and print the generated manifests.

```bash
flux-operator build instance -f flux.yaml
```

See the [Flux Operator CLI documentation](cli.md) for more details on how to use the CLI.

## Examples

The following examples demonstrate how to customize the Flux manifests.

### Increase concurrency and resources limits

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          - op: add
            path: /spec/template/spec/containers/0/args/-
            value: --concurrent=10
          - op: add
            path: /spec/template/spec/containers/0/args/-
            value: --requeue-dependency=5s 
          - op: replace
            path: /spec/template/spec/containers/0/resources/limits
            value:
              cpu: 2000m
              memory: 2048Mi
        target:
          kind: Deployment
          name: "(kustomize-controller|helm-controller|source-controller)"
```

### Node affinity and tolerations

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: all
          spec:
            template:
              metadata:
                annotations:
                  cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
              spec:
                affinity:
                  nodeAffinity:
                    requiredDuringSchedulingIgnoredDuringExecution:
                      nodeSelectorTerms:
                        - matchExpressions:
                            - key: role
                              operator: In
                              values:
                                - flux
                tolerations:
                  - effect: NoSchedule
                    key: role
                    operator: Equal
                    value: flux      
        target:
          kind: Deployment
```

### HTTP/S Proxy

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: all
          spec:
            template:
              spec:
                containers:
                  - name: manager
                    env:
                      - name: "HTTPS_PROXY"
                        value: "https://proxy.example.com"
                      - name: "NO_PROXY"
                        value: ".cluster.local.,.cluster.local,.svc"      
        target:
          kind: Deployment
```

### Cluster sync semver range

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          - op: replace
            path: /spec/ref
            value:
              semver: ">=1.0.0-0"
        target:
          kind: (GitRepository|OCIRepository)
```

### Cluster sync SOPS decryption

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          - op: add
            path: /spec/decryption
            value:
              provider: sops
              secretRef:
                name: flux-sops
        target:
          kind: Kustomization
```

### Cluster sync GitRepository verification

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          - op: add
            path: /spec/verify
            value:
              mode: HEAD
              secretRef:
                name: pgp-public-keys
        target:
          kind: GitRepository
```

### Cluster sync OCIRepository keyless verification

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
spec:
  kustomize:
    patches:
      - patch: |
          - op: add
            path: /spec/verify
            value:
              provider: cosign
              matchOIDCIdentity:
              - issuer: ^https://token\.actions\.githubusercontent\.com$
                subject: ^https://github\.com/<owner>/<repo>/\.github/workflows/push-flux-system\.yml@refs/heads/main$
        target:
          kind: OCIRepository
```

For more examples, refer to the [Flux bootstrap documentation](https://fluxcd.io/flux/installation/configuration/).

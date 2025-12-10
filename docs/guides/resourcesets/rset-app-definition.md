---
title: Using ResourceSets for Application Definitions
description: Flux Operator ResourceSets guide for application definitions
---

# Using ResourceSets for Application Definitions

The ResourceSet API allows bundling a set of Kubernetes resources
(Flux HelmRelease, Kustomization, OCIRepository, Alert, Provider, Receiver, Kubernetes Namespace, ServiceAccount, etc)
into a single deployable unit that can be templated and parameterized.

Use cases of ResourceSet include:

**Multi-instance provisioning** - Generate multiple instances of the same application
in a cluster with different configurations.

**Multi-cluster provisioning** - Generate multiple instances of the same application for
each target cluster that are deployed by Flux from a management cluster.

**Multi-tenancy provisioning** - Generate a set of resources
(Namespace, ServiceAccount, RoleBinding, Flux GitRepository, Kustomization) for each tenant
with specific roles and permissions to simplify the onboarding of new tenants
and their applications on a shared cluster.

**Dependency management** - Define dependencies between apps to ensure that the resources
are applied in the correct order. The dependencies are more flexible  than in Flux,
they can be for other ResourceSets, CRDs, or any other Kubernetes object.
When defining dependencies, these can be for checking the existence of a resource
or for waiting for a resource to be ready. To evaluate the readiness of a dependent resource,
users can specify a CEL expression that is evaluated against the resource status.

## Multi-instance example

While bundling resources is possible with Flux HelmReleases and Kustomize overlays, the ResourceSet API
can drastically reduce the amount of files and overlays needed to manage multiple instances of the same application.

With Kustomize overlays the following structure is needed to deploy an app instance
per tenant with different Helm values:

```text
apps/
└── app1
    ├── base
    │   ├── flux-helm-release.yaml
    │   ├── flux-oci-repository.yaml
    │   └── kustomization.yaml
    ├── overlays
    │   ├── tenant1
    │   │   ├── kustomization.yaml
    │   │   ├── values-patch.yaml
    │   │   └── version-patch.yaml
    │   └── tenant2
    │       ├── kustomization.yaml
    │       ├── values-patch.yaml
    │       └── version-patch.yaml
    └── bundle
        └── kustomization.yaml
```

Using a ResourceSet, the same can be achieved with a single file:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app1
  namespace: apps
spec:
  inputs:
    - tenant: "tenant1"
      app:
       version: "6.7.x"
       replicas: 2
    - tenant: "tenant2"
      app:
       version: "6.6.x"
       replicas: 3
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: app1-<< inputs.tenant >>
        namespace: apps
      spec:
        interval: 10m
        url: oci://my.registry/org/charts/app1
        ref:
          semver: << inputs.app.version | quote >>
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: app1-<< inputs.tenant >>
        namespace: apps
      spec:
        interval: 1h
        releaseName: app1-<< inputs.tenant >>
        chartRef:
          kind: OCIRepository
          name: app1-<< inputs.tenant >>
        values:
          replicaCount: << inputs.app.replicas | int >>
```

### Decoupled example

You may want to separate the inputs from the `ResourceSet` manifest to allow,
for example, different teams to manage the inputs independently, and also
without requiring every instance to be listed in the `ResourceSet` manifest.
This can be done by declaring the inputs in separate `ResourceSetInputProvider`
resources with the `spec.type` field set to `Static` and referencing them
dynamically through
[Label Selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors)
in the `ResourceSet`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app1
  namespace: apps
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      selector:
        matchLabels:
          some: label
        matchExpressions:
          - key: anotherLabel
            operator: In
            values:
              - value1
              - value2
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: app1-<< inputs.tenant >>
        namespace: apps
      spec:
        interval: 10m
        url: oci://my.registry/org/charts/app1
        ref:
          semver: << inputs.app.version | quote >>
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: app1-<< inputs.tenant >>
        namespace: apps
      spec:
        interval: 1h
        releaseName: app1-<< inputs.tenant >>
        chartRef:
          kind: OCIRepository
          name: app1-<< inputs.tenant >>
        values:
          replicaCount: << inputs.app.replicas | int >>
```

Then you can create the `ResourceSetInputProvider` resources with the
`Static` input provider type and labels matching the `ResourceSet` selector:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app1-tenant1
  namespace: apps
  labels:
    some: label
    anotherLabel: value1
spec:
  type: Static
  defaultValues:
    tenant: "tenant1"
    app:
      version: "6.7.x"
      replicas: 2
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app1-tenant2
  namespace: apps
  labels:
    some: label
    anotherLabel: value2
spec:
  type: Static
  defaultValues:
    tenant: "tenant2"
    app:
      version: "6.6.x"
      replicas: 3
```

> **Note:** The `ResourceSet` and `ResourceSetInputProvider` resources must be in the same namespace.

## Multi-cluster example

When deploying applications across multiple environments from a management cluster, the ResourceSet API
can simplify the definition of the application and its customization for each target cluster.

With Kustomize overlays the following structure is needed to deploy an app instance
per environment:

```text
apps/
└── app1
    ├── base
    │   ├── flux-kustomization.yaml
    │   ├── flux-git-repository.yaml
    │   └── kustomization.yaml
    ├── overlays
    │   ├── dev
    │   │   ├── kustomization.yaml
    │   │   ├── vars-patch.yaml
    │   │   └── kubeconfig-patch.yaml
    │   └── production
    │       ├── kustomization.yaml
    │       ├── vars-patch.yaml
    │       └── kubeconfig-patch.yaml
    └── bundle
        └── kustomization.yaml
```

Using a ResourceSet, the same can be achieved with a single file:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app1
  namespace: apps
spec:
  inputs:
    - cluster: "dev"
      branch: "main"
      ingress: "app1.dev.example.com"
    - cluster: "production"
      branch: "prod"
      ingress: "app1.example.com"
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: app1-<< inputs.cluster >>
        namespace: apps
      spec:
        interval: 5m
        url: https://my.git/org/app1-deploy
        ref:
          branch: << inputs.branch >>
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: app1-<< inputs.cluster >>
        namespace: apps
      spec:
        interval: 10m
        prune: true
        path: "./deploy"
        sourceRef:
          kind: GitRepository
          name: app1-<< inputs.cluster >>
        postBuild:
          substitute:
            domain: << inputs.ingress >>
        kubeConfig:
          secretRef:
            name: << inputs.cluster >>-kubeconfig
```

## Monorepo example

When an application is composed of multiple microservices, the ResourceSet API can be used
to define the deployment of each component and the rollout order based on dependencies.

Assuming the following directory structure in a monorepo where the Kubernetes resources
are templated using Flux variables:

```text
deploy/
├── frontend
│   ├── deployment.yaml
│   ├── ingress.yaml
│   └── service.yaml
├── backend
│   ├── deployment.yaml
│   └── service.yaml
└── database
    ├── deployment.yaml
    ├── pvc.yaml
    └── service.yaml
```

Using a ResourceSet, we can generate one GitRepository that points to the monorepo,
and a set of Flux Kustomizations one for each component that depends on the previous one:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app1
  namespace: apps
spec:
  inputs:
    - service: "frontend"
      dependsOn: "backend"
    - service: "backend"
      dependsOn: "database"
    - service: "database"
      dependsOn: ""
  resourcesTemplate: |
    ---
    apiVersion: source.toolkit.fluxcd.io/v1
    kind: GitRepository
    metadata:
      name: app1
      namespace: apps
    spec:
      interval: 5m
      url: https://my.git/org/app1-deploy
      ref:
        branch: main
    ---
    apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    metadata:
      name: app1-<< inputs.service >>
      namespace: apps
    spec:
      << if inputs.dependsOn >>
      dependsOn:
        - name: app1-<< inputs.dependsOn >>
      << end >>
      path: "./deploy/<< inputs.service >>"
      interval: 30m
      retryInterval: 5m
      prune: true
      wait: true
      timeout: 5m
      sourceRef:
        kind: GitRepository
        name: app1
      postBuild:
        substituteFrom:
          - kind: ConfigMap
            name: app1-vars
```

## Working with ResourceSets

When working with ResourceSets, you can use the Flux Operator CLI for building ResourceSet
templates locally and for listing, reconciling, suspending and resuming ResourceSets in-cluster.

The following commands are available:

```shell
# Build the given ResourceSet and print the generated objects
flux-operator build rset -f my-resourceset.yaml

# List all ResourceSets in the cluster
flux-operator get rset --all-namespaces

# Reconcile a ResourceSet 
flux-operator -n apps reconcile rset podinfo

# Suspend a ResourceSet 
flux-operator -n apps suspend rset podinfo

# Resume a ResourceSet 
flux-operator -n apps resume rset podinfo
```

See the [Flux Operator CLI documentation](cli.md) for more details on how to use the CLI.

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)


---
title: Using ResourceSets for Monorepo Deployments
description: Flux Operator ResourceSets guide for auto-generating the app delivery pipelines from a monorepo directory structure
---

# Using ResourceSets for Monorepo Deployments

This guide demonstrates how to use the Flux Operator APIs to automatically generate
the delivery pipeline for every application directory found in a monorepo.

For platform teams managing hundreds of apps and environments in a single repository,
hand-writing a Flux Kustomization for each directory generates constant churn:
every app that is added, renamed or retired means touching the Flux configuration
as well. With the approach described here, the deployment pipelines are kept in sync
with the monorepo directory structure automatically: adding a directory deploys
the app, and removing it tears the deployment down.

## How it works

Before diving into the configuration, it's important to understand how the Flux APIs
work together to enable directory-based generation of deployment pipelines:

1. A Flux [GitRepository](https://fluxoperator.dev/docs/crd/gitrepository/)
   pulls the monorepo contents into the cluster.
2. A Flux [ArtifactGenerator](https://fluxoperator.dev/docs/crd/artifactgenerator/)
   scans the repository artifact for directories matching a path pattern
   (e.g. `apps/{app}/envs/{env}`) and generates one `ExternalArtifact` resource
   per matched directory, labeled with the values captured from the path.
3. A [ResourceSetInputProvider](resourcesetinputprovider.md) of type `ExternalArtifact`
   discovers the generated artifacts using label selectors and exports an input set
   for each artifact, containing its name, namespace, revision and labels.
4. A [ResourceSet](resourceset.md) consumes the exported inputs and templates
   a Flux Kustomization per input set, which applies the manifests from the
   matched directory onto the cluster.

The pipeline is fully event-driven: the Flux Operator watches the `ExternalArtifact`
resources in the cluster and reacts instantly when artifacts are created, updated or
deleted.

## Prerequisites

The cluster must be provisioned with a [FluxInstance](fluxinstance.md) running
Flux v2.9.0 or later, with the `source-watcher` component enabled:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.9.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - source-watcher
```

The source-watcher component provides the `ArtifactGenerator` API used to scan
the monorepo and generate the `ExternalArtifact` resources.

## GitOps Workflow

To demonstrate the workflow, we'll use a monorepo with the following structure,
where each application contains a Kustomize base and an overlay per environment:

```text
platform-monorepo/
└── apps/
    ├── auth/
    │   ├── base/
    │   │   ├── kustomization.yaml
    │   │   └── deployment.yaml
    │   └── envs/
    │       ├── dev/
    │       │   └── kustomization.yaml
    │       └── prod/
    │           └── kustomization.yaml
    └── payments/
        ├── base/
        │   ├── kustomization.yaml
        │   └── deployment.yaml
        └── envs/
            ├── dev/
            │   └── kustomization.yaml
            └── prod/
                └── kustomization.yaml
```

Each environment overlay references the app base with a relative path:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patches:
  # environment-specific patches e.g.:
  - target:
      kind: Deployment
    patch: |
      - op: replace
        path: /spec/replicas
        value: 2
```

### Configure the Git source

First, we'll create a Flux `GitRepository` that pulls the monorepo contents
into the cluster:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: platform-monorepo
  namespace: apps
spec:
  interval: 5m
  url: https://github.com/org/platform-monorepo
  ref:
    branch: main
```

Note that for private repositories, you can provide credentials by referencing
a Kubernetes Secret in the `spec.secretRef` field.

On production clusters, instead of tracking a branch, you can follow semver tags
by setting `spec.ref.semver` (e.g. `semver: ">=1.0.0"`). This way, changes are
rolled out to production only when a new version of the monorepo is tagged in Git.

### Generate an artifact per app environment

Next, we'll create an `ArtifactGenerator` that scans the repository artifact for
directories matching the `apps/{app}/envs/{env}` pattern and generates an
`ExternalArtifact` for each match:

```yaml
apiVersion: source.extensions.fluxcd.io/v1beta1
kind: ArtifactGenerator
metadata:
  name: platform-apps
  namespace: apps
spec:
  sources:
    - alias: monorepo
      kind: GitRepository
      name: platform-monorepo
  commonMetadata:
    labels:
      team: platform
  pathPattern: "@monorepo/apps/{app}/envs/{env}"
  artifacts:
    - name: "{app}-{env}"
      copy:
        - from: "@monorepo/apps/{app}/base/**"
          to: "@artifact/base/"
        - from: "@monorepo/apps/{app}/envs/{env}/**"
          to: "@artifact/envs/{env}/"
```

The `{app}` and `{env}` named captures act as wildcards when matching directories,
and their captured values are used to template the artifact names and are set as
labels on the generated `ExternalArtifact` resources.

Note that the copy operations replicate the app directory structure inside the
artifact: the app base is copied along with the environment overlay, so that the
relative path reference from the overlay to the base resolves inside the artifact.

For the example monorepo, the generator produces four artifacts named
`auth-dev`, `auth-prod`, `payments-dev` and `payments-prod`. For example:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: ExternalArtifact
metadata:
  name: auth-dev
  namespace: apps
  labels:
    app: auth                # from the {app} path capture
    env: dev                 # from the {env} path capture
    team: platform           # from commonMetadata
    app.kubernetes.io/managed-by: source-watcher
    source.extensions.fluxcd.io/generator: 32251b40-a2ec-4a1c-8d40-a2f38e42ac6f
status:
  artifact:
    revision: latest@sha256:6e7dcb5a0e14be6c3ee3ba00c0be00921ba1f6a72b9b6e26aab3d0e29e9c313e
    url: http://source-watcher.flux-system.svc.cluster.local./externalartifact/apps/auth-dev/3120514532.tar.gz
```

Each artifact contains only the base and overlay of its app environment, and its
revision changes only when the contents of those directories change in Git.

### Discover the generated artifacts

With the artifacts in place, we'll create a `ResourceSetInputProvider` of type
`ExternalArtifact` that discovers them by their labels. Since each environment
runs on its own cluster, the provider selects only the artifacts belonging to
the cluster's environment. For example, on the dev cluster:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: platform-apps
  namespace: apps
spec:
  type: ExternalArtifact
  selectors:
    - matchLabels:
        team: platform
        env: dev
```

The provider lists the `ExternalArtifact` resources in its own namespace matching
the label selector, and exports an input set for each discovered artifact.
Besides the built-in `id`, `name`, `namespace` and `revision` fields, all the labels
found on the artifact are exported as input values, which makes the `app` and `env`
path captures available as template variables:

```yaml
status:
  exportedInputs:
    - id: "592053506"
      name: auth-dev
      namespace: apps
      revision: latest@sha256:6e7dcb5a0e14be6c3ee3ba00c0be00921ba1f6a72b9b6e26aab3d0e29e9c313e
      app: auth
      env: dev
      team: platform
    - id: "1023018689"
      name: payments-dev
      namespace: apps
      revision: latest@sha256:9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b
      app: payments
      env: dev
      team: platform
```

Instead of matching on labels, the selectors can also target artifacts by name,
using `matchExpressions`, or discover artifacts across namespaces. For all the
selector options, see the
[ResourceSetInputProvider API reference](resourcesetinputprovider.md#selectors).

Note that by default the provider exports at most 100 input sets. If your monorepo
contains more matching directories, increase the limit with the `spec.filter.limit` field.

### Configure the app deployments

Finally, we'll create a `ResourceSet` that generates a Flux Kustomization for each
input set exported by the provider:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: platform-apps
  namespace: apps
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: platform-apps
  resources:
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: << inputs.app >>
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 30m
        retryInterval: 5m
        timeout: 5m
        prune: true
        wait: true
        sourceRef:
          kind: ExternalArtifact
          name: << inputs.name >>
        path: ./envs/<< inputs.env >>
        targetNamespace: << inputs.provider.namespace >>
```

Each generated Kustomization points to its corresponding `ExternalArtifact` and
builds the environment overlay from the artifact contents, applying the resulting
resources in the namespace set with `targetNamespace`.

For the example monorepo, the `ResourceSet` running on the dev cluster generates
the `auth` and `payments` Kustomizations in the `apps` namespace, pointing to the
`auth-dev` and `payments-dev` artifacts.

### Day-2 operations

Once the pipeline is in place, the entire lifecycle of the deployments is driven
by the monorepo directory structure:

- **Adding an app or environment**: when a new directory matching the path pattern
  is pushed to the monorepo, an `ExternalArtifact` is generated for it, and on the
  cluster whose provider selector matches the artifact labels, the `ResourceSet`
  creates a new Kustomization that deploys the app.
- **Changing the app manifests**: when the contents of a directory change,
  the artifact revision is updated and only the affected Kustomization
  reconciles the changes.
- **Removing an app or environment**: when a directory is deleted from the monorepo,
  its `ExternalArtifact` is removed, the corresponding input disappears, and the
  `ResourceSet` deletes the Kustomization, which in turn prunes the app workloads
  from the cluster.

### Environment segregation

The same `GitRepository`, `ArtifactGenerator` and `ResourceSet` definitions can be
applied to all clusters, with the only difference being the `env` value in the input
provider selector: `env: dev` on the dev cluster, `env: prod` on the prod cluster.
To avoid maintaining per-cluster variants of the provider definition, the `env` value
can be set with Flux
[variable substitution](https://fluxoperator.dev/docs/crd/kustomization/#post-build-variable-substitution)
from a per-cluster ConfigMap.

On the production cluster, you can also restrict when changes are rolled out by
attaching deployment windows to the input provider using the `spec.schedule` field.
For more details, see the [Time-Based Delivery](rset-time-based-delivery.md) guide.

### Multi-tenancy considerations

By default, the Flux Operator discovers `ExternalArtifact` resources using its own
service account, which has read access to these resources cluster-wide.

On multi-tenant clusters, you can restrict the discovery to the permissions of
a tenant service account by setting the `spec.serviceAccountName` field on the
`ResourceSetInputProvider`. When the operator is running with the
`--default-service-account` flag, the impersonation is enforced for all providers.

Note that when using a selector with `namespace: "*"` (discovery across all
namespaces), the impersonated service account must have cluster-wide `list`
permissions on `ExternalArtifact` resources, otherwise the reconciliation
will fail with a forbidden error.

## Working with ResourceSets

Using the [Flux Operator CLI](cli.md), you can interact with the ResourceSet
and its input provider.

To view the status of the `ResourceSet`, its input provider and the generated
Kustomizations:

```shell
flux-operator -n apps get all
```

To inspect the inputs exported by the provider:

```shell
kubectl -n apps get rsip platform-apps -o yaml
```

To pause the generation of deployments, you can suspend the `ResourceSet` with:

```shell
flux-operator -n apps suspend rset platform-apps
```

To resume:

```shell
flux-operator -n apps resume rset platform-apps
```

To trigger an immediate discovery of the `ExternalArtifact` resources:

```shell
flux-operator -n apps reconcile rsip platform-apps
```

In addition, you can use the CLI to build the `ResourceSet` locally and verify
that the templates are valid with mock input data:

```shell
flux-operator build rset -f platform-apps-resourceset.yaml \
  --inputs-from-provider static-inputs.yaml
```

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)
- [Flux ArtifactGenerator API reference](https://fluxoperator.dev/docs/crd/artifactgenerator/)

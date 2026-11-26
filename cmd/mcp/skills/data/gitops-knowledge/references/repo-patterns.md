# Repository Patterns

All patterns use Flux Operator with FluxInstance for cluster bootstrap. Each cluster has a
FluxInstance that syncs from a source and applies manifests via Kustomizations or ResourceSets.

**Contents:** [Monorepo Pattern](#monorepo-pattern) | [Directory-Driven Monorepo (Gitless)](#directory-driven-monorepo-gitless) | [Multi-Repo Git-Based Pattern](#multi-repo-git-based-pattern) | [Multi-Repo OCI-Based Pattern (Gitless GitOps)](#multi-repo-oci-based-pattern-gitless-gitops) | [Per-Cluster Configuration](#per-cluster-configuration)

## Monorepo Pattern

Single Git repository containing all infrastructure and application manifests with
per-cluster directories.

### Directory Structure

```
fleet/
├── clusters/
│   ├── production/
│   │   ├── flux-system/
│   │   │   ├── flux-instance.yaml     # FluxInstance with Git sync
│   │   │   └── runtime-info.yaml      # Cluster-specific variables
│   │   └── tenants.yaml               # Root Kustomization
│   └── staging/
│       ├── flux-system/
│       │   ├── flux-instance.yaml
│       │   └── runtime-info.yaml
│       └── tenants.yaml
├── infrastructure/
│   ├── controllers/
│   │   ├── base/                      # HelmReleases for CRDs and operators
│   │   ├── production/
│   │   └── staging/
│   └── configs/
│       ├── base/                      # Cluster configs (issuers, monitoring)
│       ├── production/
│       └── staging/
├── apps/
│   ├── base/                          # App deployments
│   ├── production/
│   └── staging/
└── tenants/
    ├── infra.yaml                     # Kustomization for infrastructure
    └── apps.yaml                      # Kustomization for applications
```

### FluxInstance with Git Sync

Configure `FluxInstance.spec.sync` to apply the cluster directory from the fleet repo
(full FluxInstance spec in `references/flux-operator.md`):

```yaml
spec:
  sync:
    kind: GitRepository
    url: "https://github.com/org/fleet.git"
    ref: "refs/heads/main"
    path: "clusters/production"
    pullSecret: "git-credentials"
```

### Root Kustomization

The root Kustomization (`tenants.yaml`) applies manifests from the `tenants/` directory:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: tenants
  namespace: flux-system
spec:
  interval: 12h
  retryInterval: 3m
  path: ./tenants
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: flux-runtime-info
```

### Monorepo Decomposition with ArtifactGenerator

For large monorepos, use ArtifactGenerator to split into independent artifacts:

```yaml
apiVersion: source.extensions.fluxcd.io/v1beta1
kind: ArtifactGenerator
metadata:
  name: monorepo
  namespace: flux-system
spec:
  sources:
    - alias: repo
      kind: GitRepository
      name: fleet
  artifacts:
    - name: infrastructure
      revision: "@repo"
      copy:
        - from: "@repo/infrastructure/**"
          to: "@artifact/"
    - name: apps
      revision: "@repo"
      copy:
        - from: "@repo/apps/**"
          to: "@artifact/"
```

Each ExternalArtifact can be referenced by a separate Kustomization, enabling
independent reconciliation and failure isolation.

When the monorepo holds many apps with per-environment overlays (`apps/<app>/envs/<env>`),
skip the hand-written artifact list and Kustomizations entirely: `pathPattern` discovers the
directories, a `ResourceSetInputProvider` of `type: ExternalArtifact` exports one input per
generated artifact, and a `ResourceSet` templates the Kustomizations — the next section.

## Directory-Driven Monorepo (Gitless)

The recommended monorepo layout for Flux Operator fleets. The whole
`kubernetes/` tree is pushed as one OCI artifact per cluster; the cluster never reads Git.
Flux Operator generates the delivery pipeline from the directory structure: a component or app
is a directory with a fixed shape, environments are overlays with fixed names, and no Flux
Kustomization is ever written by hand for them.

```text
kubernetes/
├── clusters/<cluster>/                  # the ONLY cluster-specific directory
│   ├── flux-instance.yaml               #   OCI sync of clusters/<cluster>/, source-watcher enabled
│   ├── flux-vars.yaml                   #   ConfigMap: env, cluster_registry, app_registry, cluster_name
│   ├── infra-reconcilers.yaml           #   Kustomization → ./infra/reconcilers
│   └── apps-<app_env>-reconcilers.yaml  #   one per hosted app env → ./apps/reconcilers
├── infra/
│   ├── reconcilers/                     # ArtifactGenerator + RSIP + ResourceSet per layer
│   │   ├── core.yaml                    #   → Kustomization infra-core
│   │   ├── controllers.yaml             #   → Kustomization <component>   (depends on core)
│   │   └── configs.yaml                 #   → Kustomization <component>-configs (depends on <component>)
│   ├── core/{base,envs/<env>}/          # Namespaces + NetworkPolicies for all components
│   └── components/<name>/
│       ├── controllers/{base,envs/<env>}/   # OCIRepository (chart) + HelmRelease
│       └── configs/{base,envs/<env>}/       # CRs that need the controller's CRDs
└── apps/
    ├── reconcilers/                     # generic; applied once per hosted app env
    │   ├── artifacts.yaml               #   ArtifactGenerator apps/{app}/envs/{env}
    │   ├── env.yaml                     #   Namespace apps-<app_env> + plumbing ResourceSet
    │   └── <app>.yaml                   #   image RSIP + ResourceSet per app
    └── <app>/{base,envs/<app_env>}/     # plain manifests + app.env → ConfigMap
```

**Two kinds of environment.** The *cluster env* (`dev`, `prod`; one per cluster, from
`flux-vars`) selects `infra/**/envs/<env>` overlays. The *app env* (`test`, `staging`, `prod`)
is a namespace `apps-<app_env>` hosting one instance of every app, selecting
`apps/<app>/envs/<app_env>` overlays; a cluster hosts any number of them (`dev` hosts `test` +
`staging`, `prod` hosts `prod`), one small reconciler file each, carrying that app env's image
policy (`app_semver`).

**Delivery.** Infra components and apps are split into per-directory `ExternalArtifact`s
in-cluster, so a change to one app reconciles only that app's Kustomization. Apps additionally
get gitless image automation: an `OCIArtifactTag` provider per app and app env scans the
cluster's registry, and the generated Kustomization pins `tag@digest` via `spec.images`.
Promotion to prod is copying the image into the prod registry (`flux mirror`).

Full manifests, ordering chain, and the rules that must hold (plain-name images, YAML-safe
semver ranges, namespace as plain manifest, unique names) are in
`references/monorepo-delivery.md`.

## Multi-Repo Git-Based Pattern

Separate repositories for fleet management, infrastructure, and applications.
A fleet repo orchestrates by creating GitRepository + Kustomization resources
pointing to component repos.

### Directory Structure

**Fleet repo:**
```
fleet/
├── clusters/
│   ├── production/
│   │   ├── flux-system/
│   │   │   ├── flux-instance.yaml
│   │   │   └── runtime-info.yaml
│   │   └── tenants.yaml
│   └── staging/
│       └── ...
└── tenants/
    ├── infra.yaml                     # GitRepository + Kustomization for infra repo
    └── apps.yaml                      # GitRepository + Kustomization for apps repo
```

**Infrastructure repo:**
```
infra/
├── controllers/
│   ├── base/
│   ├── production/
│   └── staging/
└── configs/
    ├── base/
    ├── production/
    └── staging/
```

**Applications repo:**
```
apps/
├── frontend/
│   ├── base/
│   ├── production/
│   └── staging/
└── backend/
    ├── base/
    ├── production/
    └── staging/
```

### Orchestration from Fleet Repo

```yaml
# tenants/infra.yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: infra
  namespace: flux-system
spec:
  interval: 5m
  url: https://github.com/org/infra.git
  ref:
    branch: main
  secretRef:
    name: git-credentials
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: infra-controllers
  namespace: flux-system
spec:
  interval: 30m
  sourceRef:
    kind: GitRepository
    name: infra
  path: ./controllers/${ENVIRONMENT}
  prune: true
  wait: true
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: flux-runtime-info
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: infra-configs
  namespace: flux-system
spec:
  dependsOn:
    - name: infra-controllers
  interval: 30m
  sourceRef:
    kind: GitRepository
    name: infra
  path: ./configs/${ENVIRONMENT}
  prune: true
  wait: true
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: flux-runtime-info
```

## Multi-Repo OCI-Based Pattern (Gitless GitOps)

The recommended pattern for production. Uses OCI artifacts instead of Git for manifest
distribution. Based on the D2 reference architecture.

### Architecture

```
CI Pipeline:
  infra repo → build → push OCI artifact per component
  apps repo  → build → push OCI artifact per component

Fleet repo:
  Defines FluxInstance + ResourceSets
  CI pushes fleet OCI artifact

Clusters:
  FluxInstance syncs fleet OCI artifact
  ResourceSets create per-tenant OCIRepository + Kustomization
  Each tenant pulls component OCI artifacts
```

### Directory Structure

**Fleet repo:**
```
fleet/
├── clusters/
│   ├── prod-eu/
│   │   ├── flux-system/
│   │   │   ├── flux-instance.yaml     # OCI sync + Cosign verification
│   │   │   ├── flux-operator.yaml     # Self-managed operator ResourceSet
│   │   │   └── runtime-info.yaml      # Cluster variables
│   │   └── tenants.yaml               # Bootstrap Kustomization
│   ├── staging/
│   │   └── ...
│   └── update/                        # Dedicated cluster for image automation
│       ├── flux-system/
│       └── automation.yaml            # Image update ResourceSet
└── tenants/
    ├── policies.yaml                  # Security policies ResourceSet
    ├── infra.yaml                     # Infrastructure ResourceSet
    └── apps.yaml                      # Applications ResourceSet
```

**Infrastructure repo:**
```
infra/
├── components/
│   ├── cert-manager/
│   │   ├── controllers/
│   │   │   ├── base/                  # OCIRepository + HelmRelease
│   │   │   ├── production/
│   │   │   └── staging/
│   │   └── configs/
│   │       ├── base/                  # ClusterIssuers, etc.
│   │       ├── production/
│   │       └── staging/
│   └── monitoring/
│       ├── controllers/
│       └── configs/
└── update-policies/                   # ImageRepository + ImagePolicy per component
    ├── cert-manager.yaml
    └── kube-prometheus-stack.yaml
```

**Applications repo:**
```
apps/
├── components/
│   ├── frontend/
│   │   ├── base/                      # OCIRepository + HelmRelease
│   │   ├── production/                # Environment overrides
│   │   └── staging/
│   └── backend/
│       ├── base/
│       ├── production/
│       └── staging/
└── update-policies/
    ├── frontend-podinfo.yaml
    └── backend-redis.yaml
```

### Key Characteristics

**OCI artifact flow:**
- CI builds and pushes OCI artifacts per component (e.g., `oci://ghcr.io/org/apps/frontend:v1.0.0`)
- Artifacts are signed with Cosign in CI
- FluxInstance pulls the fleet artifact and applies cluster configuration
- ResourceSets create per-tenant OCIRepository + Kustomization
- Each tenant pulls its own component artifacts from the registry

**Tag promotion:**
- `latest` — pushed from main branch, used by staging
- `latest-stable` — pushed from release tags, used by production
- Cosign verification subjects differ per environment (main branch vs release tags)

**Dependency chain:**
1. FluxInstance bootstraps the cluster
2. Root Kustomization applies `tenants/` directory
3. `policies` ResourceSet creates security policies
4. `infra` ResourceSet (depends on policies) creates infrastructure
5. `apps` ResourceSet (depends on infra) creates applications

**Runtime configuration:**
- Each cluster has a `flux-runtime-info` ConfigMap with environment variables
- Variables are substituted in ResourceSet inputs (`${VAR}`) and Kustomization postBuild
- ConfigMap is copied to tenant namespaces via `fluxcd.controlplane.io/copyFrom`

### Benefits of OCI-Based Pattern

- **No Git credentials on clusters** — only registry pull secrets needed
- **Immutable artifacts** — OCI artifacts are content-addressable
- **Cryptographic verification** — Cosign signatures verify artifact provenance
- **Decoupled delivery** — Git push triggers CI, CI pushes artifact, Flux pulls artifact
- **Registry-native** — leverages existing container registry infrastructure
- **Faster sync** — OCI pulls are faster than Git clones for large repos

## Per-Cluster Configuration

All patterns use the same approach for cluster-specific configuration:

### Runtime Info ConfigMap

Each cluster carries a `flux-runtime-info` ConfigMap in `flux-system` with
cluster-specific variables, e.g. `ENVIRONMENT: production`, `CLUSTER_NAME: prod-eu-1`,
`CLUSTER_DOMAIN: prodeu1.example.com`, `ARTIFACT_TAG: latest-stable`. The canonical
manifest — including the `reconcile.fluxcd.io/watch: Enabled` label and the
`kustomize.toolkit.fluxcd.io/ssa: "Merge"` annotation — is in `references/flux-operator.md`.

Variables are consumed by:
- `postBuild.substituteFrom` in Kustomizations → `${ENVIRONMENT}`, `${CLUSTER_DOMAIN}`
- Input values in ResourceSets → `"${ARTIFACT_TAG}"` (resolved during Kustomization apply)

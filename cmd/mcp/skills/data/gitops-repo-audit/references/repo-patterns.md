# GitOps Repository Patterns

Common patterns for organizing Flux-managed GitOps repositories.

## Monorepo Pattern

A single Git repository contains everything: applications, infrastructure, and cluster configurations.
Best suited for small-to-medium teams or organizations starting with GitOps.

```
repo/
├── apps/
│   ├── base/                    # Shared app definitions
│   │   └── <app-name>/
│   │       ├── kustomization.yaml
│   │       ├── namespace.yaml
│   │       ├── repository.yaml  # HelmRepository or OCIRepository
│   │       └── release.yaml     # HelmRelease
│   ├── staging/                 # Staging overlay (patches base)
│   │   ├── kustomization.yaml
│   │   └── <app>-values.yaml
│   └── production/              # Production overlay
│       ├── kustomization.yaml
│       └── <app>-values.yaml
├── infrastructure/
│   ├── controllers/             # CRDs and operators (cert-manager, gateway, etc.)
│   │   └── kustomization.yaml
│   └── configs/                 # Custom resources (ClusterIssuers, etc.)
│       └── kustomization.yaml
└── clusters/
    ├── staging/
    │   ├── flux-system/         # Flux bootstrap (gotk-components, gotk-sync) or FluxInstance
    │   ├── artifacts.yaml       # ArtifactGenerator (optional, for monorepo decomposition)
    │   ├── infrastructure.yaml  # Kustomization for infra
    │   └── apps.yaml            # Kustomization for apps
    └── production/
        ├── flux-system/
        ├── artifacts.yaml
        ├── infrastructure.yaml
        └── apps.yaml
```

**Key characteristics:**
- ArtifactGenerator splits the repo into independent ExternalArtifacts so infra and app changes reconcile independently
- Kustomize base+overlay pattern avoids duplicating manifests across environments
- Dependency chain: `infra-controllers` → `infra-configs` → `apps`
- Each cluster directory contains its own Flux bootstrap and reconciliation config

**Flux CRDs typically used:** GitRepository (via bootstrap or FluxInstance), Kustomization, HelmRelease, HelmRepository/OCIRepository, ArtifactGenerator, ExternalArtifact

## Directory-Driven Monorepo (Flux Operator, Gitless)

A monorepo where Flux Operator generates the delivery pipeline from the directory structure:
an `ArtifactGenerator` with `pathPattern` produces one labeled `ExternalArtifact` per
`<thing>/envs/<env>` directory, a `ResourceSetInputProvider` of `type: ExternalArtifact` exports
one input per artifact, and a `ResourceSet` templates a Flux `Kustomization` per input. No Flux
Kustomization is written by hand for components or apps — adding a directory deploys it, removing
it prunes the deployment. Usually pushed as one OCI artifact per cluster (`FluxInstance` with
`sync.kind: OCIRepository`), so the cluster never reads Git.

```
kubernetes/
├── clusters/<cluster>/                  # the ONLY cluster-specific directory
│   ├── flux-instance.yaml               #   OCI sync, source-watcher component enabled
│   ├── flux-vars.yaml                   #   ConfigMap: env, cluster_registry, app_registry, ...
│   ├── infra-reconcilers.yaml           #   Kustomization → ./infra/reconcilers (substituteFrom flux-vars)
│   └── apps-<app_env>-reconcilers.yaml  #   one per hosted app env → ./apps/reconcilers
├── infra/
│   ├── reconcilers/                     # ArtifactGenerator + RSIP + ResourceSet per layer
│   │   ├── core.yaml                    #   → Kustomization infra-core
│   │   ├── controllers.yaml             #   → Kustomization <component>   (dependsOn core)
│   │   └── configs.yaml                 #   → Kustomization <component>-configs
│   ├── core/{base,envs/<env>}/          # Namespaces + NetworkPolicies for all components
│   └── components/<name>/
│       ├── controllers/{base,envs/<env>}/   # OCIRepository (chart) + HelmRelease
│       └── configs/{base,envs/<env>}/       # CRs needing the controller's CRDs
└── apps/
    ├── reconcilers/                     # generic; applied once per hosted app env
    │   ├── artifacts.yaml               #   ArtifactGenerator apps/{app}/envs/{env}
    │   ├── env.yaml                     #   Namespace apps-<app_env> (plain manifest) + plumbing ResourceSet
    │   └── <app>.yaml                   #   OCIArtifactTag RSIP + ResourceSet per app
    └── <app>/{base,envs/<app_env>}/     # plain manifests, image: <app> (bare name)
```

**Key characteristics:**
- Two kinds of environment: the *cluster env* (`dev`/`prod`, from `flux-vars`) selects `infra/**/envs/<env>`; the *app env* (`test`/`staging`/`prod`) is a namespace `apps-<app_env>` hosting one instance of every app
- Ordering is entirely at ResourceSet level: `infra-core` → `infra-controllers` → `apps-<app_env>` → per-app ResourceSets, all with `wait: true` and `dependsOn` entries that set `namespace`
- Apps combine both input types: manifests from the generated `ExternalArtifact` (referenced by its deterministic name `<app>-<app_env>`), image version from an `OCIArtifactTag` provider pinned via the Kustomization `spec.images` (`newTag` + `digest`)
- Generated Kustomizations in `apps-<app_env>` reference sources in `flux-system` — this is intentional and relies on `multitenant: false`
- Only `clusters/<cluster>/` differs between clusters; everything else is parameterised by `${env}`, `${cluster_registry}`, `${app_registry}`, `${app_env}`, `${app_semver}`

**What to audit:** the checks under *ResourceSet Pipelines and Jobs* and *Post-Build Substitution* in
[best-practices.md](best-practices.md) — `source-watcher` enabled, base + overlay copied by every
`pathPattern` generator, YAML-safe substituted values, plain image names in `spec.images[].name`,
`namespace` on ResourceSet `dependsOn`, `wait: true` and `reconcileTimeout` on every ResourceSet.

**Flux CRDs typically used:** FluxInstance, OCIRepository, ArtifactGenerator, ExternalArtifact, ResourceSetInputProvider (`ExternalArtifact`, `OCIArtifactTag`), ResourceSet, Kustomization, HelmRelease

## Multi-Repo Pattern (Git-based)

Separate repositories for fleet management, infrastructure, and applications.
Best for organizations with distinct platform and application teams.

```
fleet-repo/                      # Platform team owns this
├── clusters/
│   ├── staging/
│   │   ├── flux-system/         # Flux bootstrap (gotk-components, gotk-sync) or FluxInstance
│   │   ├── runtime-info.yaml    # ConfigMap with cluster variables
│   │   ├── infra-tenant.yaml    # Kustomization → tenants/infra
│   │   └── apps-tenant.yaml     # Kustomization → tenants/apps (dependsOn infra)
│   └── production/
├── tenants/
│   ├── infra/
│   │   └── components/
│   │       ├── source.yaml      # GitRepository pointing to infra-repo
│   │       └── sync.yaml        # Kustomization reconciling infra components
│   └── apps/
│       └── components/
│           ├── <app>/
│           │   ├── namespace.yaml
│           │   ├── rbac.yaml    # ServiceAccount + RoleBinding per tenant
│           │   └── sync.yaml    # GitRepository + Kustomization for app-repo

infra-repo/                      # Platform team owns this
├── components/
│   ├── <component>/             # e.g., cert-manager, monitoring
│   │   ├── controllers/
│   │   │   ├── base/
│   │   │   ├── staging/
│   │   │   └── production/
│   │   └── configs/
│   │       ├── base/
│   │       ├── staging/
│   │       └── production/
└── update/                      # ImageRepository + ImagePolicy for auto-updates

apps-repo/                       # Dev teams own this
├── components/
│   ├── <app>/                   # e.g., frontend, backend
│   │   ├── base/
│   │   ├── staging/
│   │   └── production/
└── update/                      # ImageRepository + ImagePolicy
```

**Key characteristics:**
- Fleet repo orchestrates: creates GitRepositories pointing to infra and app repos
- RBAC isolation: each tenant gets its own ServiceAccount with scoped RoleBinding
- `postBuild.substituteFrom` injects cluster variables (environment, domain) from ConfigMaps
- Image automation (ImageUpdateAutomation) runs in fleet repo, pushes to infra/app repos

**Flux CRDs typically used:** GitRepository, Kustomization, HelmRelease, HelmRepository, ImageRepository, ImagePolicy, ImageUpdateAutomation

## Multi-Repo Pattern (OCI-based)

Similar to Git-based multi-repo but uses OCI artifacts instead of Git for manifest distribution.
Most advanced pattern, suited for enterprise deployments with strict security requirements.

```
fleet-repo/                      # Platform team owns this
├── clusters/
│   ├── staging/
│   │   └── flux-system/
│   │       └── flux-instance.yaml  # FluxInstance with OCI sync
│   └── production/
├── tenants/
│   ├── apps.yaml                # ResourceSet template for app tenants
│   ├── infra.yaml               # ResourceSet template for infra tenants
│   └── policies.yaml            # ValidatingAdmissionPolicy for source allowlist
├── deploy/                      # Sample app manifests
└── terraform/                   # Bootstrap infrastructure

infra-repo/                      # Published as OCI artifacts per component
├── components/
│   └── <component>/
│       ├── controllers/
│       └── configs/
└── update-policies/             # ImageRepository + ImagePolicy

apps-repo/                       # Published as OCI artifacts per component
├── components/
│   └── <app>/
│       ├── base/
│       ├── staging/
│       └── production/
└── update-policies/
```

**Key characteristics:**
- FluxInstance manages Flux installation declaratively (replaces manual bootstrap)
- ResourceSet templates generate per-tenant resources (namespaces, RBAC, OCIRepositories, Kustomizations)
- OCI artifacts are immutable, signed with Cosign, and tagged with semver
- Artifact tag promotion: `latest` for staging → `latest-stable` for production
- ValidatingAdmissionPolicy restricts allowed source URLs

**Flux CRDs typically used:** FluxInstance, ResourceSet, ResourceSetInputProvider, OCIRepository, Kustomization, HelmRelease

## Identification Heuristics

Use these signals to classify an unknown repo:

| Signal                                                     | Likely Pattern                                     |
|------------------------------------------------------------|----------------------------------------------------|
| Has `clusters/*/` directories                              | Multi-cluster setup (monorepo or fleet repo)       |
| Has `apps/base/` and `apps/<env>/`                         | Monorepo with Kustomize overlays                   |
| Has `ArtifactGenerator` resources                          | Monorepo with source decomposition                 |
| `ArtifactGenerator` with `pathPattern` + `ResourceSetInputProvider` `type: ExternalArtifact` | Directory-driven monorepo (Flux Operator) |
| Has `apps/<app>/envs/<env>/` or `infra/components/<name>/{controllers,configs}/envs/<env>/` and a `reconcilers/` dir | Directory-driven monorepo |
| Has `ResourceSet` with `spec.steps` generating `Job` resources | Staged deployments (migration/smoke-test Jobs around a rollout) |
| Has `FluxInstance` with `sync.kind.GitRepository` resource | Git-based fleet repo (Flux Operator)               |
| Has `FluxInstance` with `sync.kind.OCIRepository` resource | OCI-based fleet repo (Flux Operator)               |
| Has `ResourceSet` resources                                | Flux Operator templating — fleet repo (tenants) or directory-driven monorepo (per-directory Kustomizations) |
| Has `tenants/` directory                                   | Fleet repo (Git or OCI multi-repo)                 |
| Has `components/` with `base/staging/production`           | Infra or apps repo in multi-repo setup             |
| Has `update/` or `update-policies/` directory              | Repo with image automation                         |
| References external Git repos via GitRepository            | Fleet repo in Git-based multi-repo                 |
| References OCI artifacts via OCIRepository                 | OCI-based setup                                    |
| Has `postBuild.substituteFrom`                             | Multi-cluster setup with per-cluster variables     |

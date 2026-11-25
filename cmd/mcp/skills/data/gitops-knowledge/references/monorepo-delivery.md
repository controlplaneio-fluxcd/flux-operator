# Monorepo Delivery Reference

Generate the delivery pipeline for every component and application directory in a monorepo
automatically. For platform teams with many apps and environments in one repository,
hand-writing a Flux `Kustomization` per directory is constant churn: every app added, renamed
or retired means editing the Flux configuration too. With this pattern the pipelines follow
the directory structure — adding a directory deploys it, removing it tears the deployment down.

**Contents:** [How It Works](#how-it-works) | [Prerequisites](#prerequisites) | [Minimal Pipeline](#minimal-pipeline) | [Production Layout](#production-layout) (two environments, layered infra, apps with image automation, ordering, rules) | [Validation and Publishing](#validation-and-publishing) | [Day-2 Operations](#day-2-operations) | [Multi-Tenancy](#multi-tenancy) | [Operations](#operations)

## How It Works

Four objects, each reacting to the previous one:

```
Source (GitRepository, or the FluxInstance OCIRepository for Gitless)
  │
  ▼ artifact of the whole repo
ArtifactGenerator (pathPattern: "@monorepo/apps/{app}/envs/{env}")
  │
  ▼ one ExternalArtifact per matched directory, labeled app=…, env=…
ResourceSetInputProvider (type: ExternalArtifact, selectors by label)
  │
  ▼ one input set per artifact: id, name, namespace, revision + all labels
ResourceSet
  │
  ▼ one Flux Kustomization per input (sourceRef.kind: ExternalArtifact)
Workloads
```

1. A source pulls the monorepo into the cluster.
2. An `ArtifactGenerator` (from the `source-watcher` component) scans the source artifact for
   directories matching `spec.pathPattern` and generates one `ExternalArtifact` per match. The
   named captures (`{app}`, `{env}`) become labels on the generated artifact, alongside
   `commonMetadata` labels. Each artifact contains only its directory's `base/` and
   `envs/<env>/`, so its revision changes only when those files change — a commit touching one
   app never triggers a reconciliation storm across every other app.
3. A `ResourceSetInputProvider` of `type: ExternalArtifact` discovers the artifacts with label
   selectors and exports one input set per artifact — `id`, `name`, `namespace`, `revision`
   plus every label, so `app` and `env` are available as template variables.
4. A `ResourceSet` templates a Flux `Kustomization` per input, pointing at the artifact.

The pipeline is event-driven: the operator watches `ExternalArtifact` objects and reacts as soon
as they are created, updated or deleted.

## Prerequisites

Flux **v2.9.0 or later** (v2.10.0+ for cross-namespace artifacts and managed tenant namespaces) with
the `source-watcher` component enabled on the `FluxInstance` (see `references/flux-operator.md`):

```yaml
spec:
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - source-watcher
```

Cross-namespace `sourceRef`/`dependsOn` references between the generated objects rely on
`spec.cluster.multitenant: false` (the default).

## Minimal Pipeline

The smallest working shape: one layer of apps, one environment per cluster, all four objects
in the `apps` namespace. Each app has a Kustomize base and one overlay per environment that
references the base with a relative path (`resources: [../../base]`):

```text
apps/
├── auth/
│   ├── base/                # kustomization.yaml, deployment.yaml, ...
│   └── envs/
│       ├── dev/             # kustomization.yaml: resources: [../../base] + patches
│       └── prod/
└── payments/
    ├── base/
    └── envs/{dev,prod}/
```

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
    branch: main          # prod: ref.semver: ">=1.0.0" to roll out only on tags
---
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
      team: platform                       # extra label the provider can select on
  pathPattern: "@monorepo/apps/{app}/envs/{env}"
  artifacts:
    - name: "{app}-{env}"                  # auth-dev, auth-prod, payments-dev, payments-prod
      copy:
        - from: "@monorepo/apps/{app}/base/**"
          to: "@artifact/base/"
        - from: "@monorepo/apps/{app}/envs/{env}/**"
          to: "@artifact/envs/{env}/"
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: platform-apps
  namespace: apps
spec:
  type: ExternalArtifact                   # no url for this type
  selectors:
    - matchLabels:                         # artifacts in the provider's own namespace
        team: platform
        env: dev                           # prod cluster: env: prod
---
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
        name: << inputs.app >>             # stable across environments
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

Key details:

- The two `copy` operations **replicate the app directory structure inside the artifact**
  (`base/` + `envs/<env>/`) so the overlay's `../../base` reference resolves. Copying only the
  overlay directory is the most common mistake — the Kustomization build then fails on the
  missing base.
- `{app}` and `{env}` act as wildcards when matching directories; their captured values
  template the artifact name and are set as **labels** on each `ExternalArtifact`
  (`app: auth`, `env: dev`).
- The provider exports at most `spec.filter.limit` inputs (default 100); raise it for larger
  monorepos. Selector variants: `name:` picks one artifact, `matchExpressions` for set logic,
  `namespace: <ns>` or `namespace: "*"` discovers outside the provider's namespace.
- The same three shared objects apply to every cluster; only the provider's `env` value
  differs — substitute it from a per-cluster ConfigMap (`env: ${env}` with
  `postBuild.substituteFrom` on the Flux Kustomization that applies these manifests).

Exported inputs for the example above:

```yaml
status:
  exportedInputs:
    - id: "592053506"        # Adler-32 of "<namespace>/<name>"
      name: auth-dev
      namespace: apps
      revision: latest@sha256:6e7d...
      app: auth              # labels exported at the root of the input
      env: dev
      team: platform
```

## Production Layout

A Gitless GitOps monorepo where infra components and apps are driven by folders, apps get
per-environment image automation from the registry, and only one directory differs between
clusters. Use this layout when generating a fleet monorepo from scratch.

```text
kubernetes/                              # pushed as ONE OCI artifact: oci://<registry>/flux-cluster:<cluster>
├── clusters/<cluster>/                  # the ONLY cluster-specific directory
│   ├── flux-instance.yaml               #   FluxInstance: OCI sync, path clusters/<cluster>/, source-watcher
│   ├── flux-vars.yaml                   #   ConfigMap flux-vars: env, cluster_registry, app_registry, cluster_name
│   ├── infra-reconcilers.yaml           #   Kustomization → ./infra/reconcilers (substituteFrom flux-vars)
│   └── apps-<app_env>-reconcilers.yaml  #   one per hosted app env → ./apps/reconcilers (substitute app_env, app_semver)
├── infra/
│   ├── reconcilers/                     # ArtifactGenerator + RSIP + ResourceSet per layer, in flux-system
│   │   ├── core.yaml                    #   infra/core/envs/{env}                         → Kustomization infra-core
│   │   ├── controllers.yaml             #   infra/components/{app}/controllers/envs/{env} → Kustomization <app>
│   │   └── configs.yaml                 #   infra/components/{app}/configs/envs/{env}     → Kustomization <app>-configs
│   ├── core/{base,envs/<env>}/          # Namespaces + NetworkPolicies for ALL infra components
│   └── components/<name>/
│       ├── controllers/{base,envs/<env>}/   # OCIRepository (chart) + HelmRelease
│       └── configs/{base,envs/<env>}/       # CRs needing the controller's CRDs (optional)
└── apps/
    ├── reconcilers/                     # generic; applied once per hosted app env
    │   ├── artifacts.yaml               #   ArtifactGenerator apps: apps/{app}/envs/{env} → ExternalArtifact {app}-{env}
    │   ├── env.yaml                     #   Namespace apps-${app_env} + ResourceSet (flux-vars copy, NetworkPolicy)
    │   └── <app>.yaml                   #   RSIP (OCIArtifactTag) + ResourceSet per app, in apps-${app_env}
    └── <app>/{base,envs/<app_env>}/     # plain manifests: Deployment, Service, HPA, app.env → ConfigMap
```

### Two Kinds of Environment

| | Cluster env `${env}` | App env `${app_env}` |
|---|---|---|
| Values | `dev`, `prod` | `test`, `staging`, `prod` |
| Scope | One per cluster, from `flux-vars` | A namespace `apps-<app_env>` hosting one instance of every app |
| Selects | `infra/**/envs/<env>` overlays | `apps/<app>/envs/<app_env>` overlays |
| Defined by | `clusters/<cluster>/flux-vars.yaml` | `clusters/<cluster>/apps-<app_env>-reconcilers.yaml` |

A cluster hosts any number of app envs: `dev` hosts `test` + `staging`, `prod` hosts `prod`.
Hosting another app env on a cluster is one small file.

### Layered Infra Reconcilers

Each layer file holds one `ArtifactGenerator` + `ResourceSetInputProvider` + `ResourceSet`
triple in `flux-system`. The generators all read the same root `OCIRepository`; a `role` label
from `commonMetadata` keeps each provider scoped to its own layer's artifacts:

| Layer | `pathPattern` | Artifact | Generated Kustomization | Ordering |
|-------|---------------|----------|-------------------------|----------|
| core | `infra/core/envs/{env}` | `core-{env}` | `infra-core` | — (RSET `wait: true`) |
| controllers | `infra/components/{app}/controllers/envs/{env}` | `{app}-{env}` | `<app>` | RSET `dependsOn` RSET `infra-core` |
| configs | `infra/components/{app}/configs/envs/{env}` | `{app}-{env}-configs` | `<app>-configs` | Kustomization `dependsOn: <app>` (CRDs before CRs) |

The controllers layer, in full:

```yaml
apiVersion: source.extensions.fluxcd.io/v1beta1
kind: ArtifactGenerator
metadata:
  name: infra-controllers
  namespace: flux-system
spec:
  sources:
    - alias: monorepo
      kind: OCIRepository
      name: flux-system                    # the FluxInstance sync source
  commonMetadata:
    labels:
      role: infra-controllers
  pathPattern: "@monorepo/infra/components/{app}/controllers/envs/{env}"
  artifacts:
    - name: "{app}-{env}"
      copy:
        - from: "@monorepo/infra/components/{app}/controllers/base/**"
          to: "@artifact/base/"
        - from: "@monorepo/infra/components/{app}/controllers/envs/{env}/**"
          to: "@artifact/envs/{env}/"
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: infra-controllers
  namespace: flux-system
spec:
  type: ExternalArtifact
  selectors:
    - matchLabels:
        role: infra-controllers
        env: ${env}                        # from flux-vars via the infra-reconcilers Kustomization
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: infra-controllers
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: "10m"
spec:
  wait: true
  dependsOn:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: infra-core
      namespace: flux-system               # always set namespace on RSET dependsOn
      ready: true
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: infra-controllers
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
        postBuild:
          substituteFrom:
            - kind: ConfigMap
              name: flux-vars              # components may use ${cluster_registry}, ${env}, ...
```

The configs layer is identical except the artifact is `{app}-{env}-configs`, the Kustomization
is `<< inputs.app >>-configs` and carries `dependsOn: [{name: << inputs.app >>}]`. Components
never create their own namespace — namespaces and network policies for all components live in
`infra/core`, applied first. Chart sources use `oci://${cluster_registry}/charts/<name>` with
`ref.semver: '*'`; cross-component ordering goes on `HelmRelease.spec.dependsOn`.

### Apps: Manifests from the Artifact, Images from the Registry

Apps combine the two ResourceSet input types: the app manifests come from the generated
`ExternalArtifact`, the image version from an `OCIArtifactTag` provider scanning the registry
(gitless image automation, see `references/gitless-image-automation.md`). Because the artifact
name is deterministic (`<app>-<app_env>`), the app ResourceSet does not need to *discover* it —
its input provider is the image scanner, and the artifact is referenced by name.

`apps/reconcilers/` is cluster- and app-env-agnostic. The cluster applies it once per hosted
app env with the app env and its image policy substituted:

```yaml
# clusters/dev/apps-test-reconcilers.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps-test-reconcilers
  namespace: flux-system
spec:
  interval: 30m
  retryInterval: 5m
  timeout: 5m
  prune: true
  wait: false                              # ordering is enforced at ResourceSet level
  sourceRef:
    kind: OCIRepository
    name: flux-system
  path: ./apps/reconcilers
  postBuild:
    substitute:
      app_env: "test"
      app_semver: "x || >=0.0.0-0"         # test: any version incl. release candidates
    substituteFrom:                        # staging/prod: "x" (stable only)
      - kind: ConfigMap
        name: flux-vars
```

`env.yaml` creates the app-env namespace as a **plain manifest** (it must exist before the
namespaced provider/ResourceSet below are applied, so it cannot be generated by a ResourceSet)
and a ResourceSet owning the namespace plumbing. The generated Kustomizations use
`substituteFrom: flux-vars`, which only resolves ConfigMaps in their own namespace, so the
ResourceSet copies `flux-system/flux-vars` there:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: apps-${app_env}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: apps-${app_env}
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: "10m"
spec:
  wait: true
  dependsOn:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: infra-controllers                # apps only after infra (e.g. metrics-server for HPAs)
      namespace: flux-system
      ready: true
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: flux-vars
        namespace: apps-${app_env}
        annotations:
          fluxcd.controlplane.io/copyFrom: "flux-system/flux-vars"
        labels:
          reconcile.fluxcd.io/watch: Enabled
    - apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      metadata:
        name: allow-egress
        namespace: apps-${app_env}
      spec:
        podSelector: {}
        policyTypes: [Egress]
        egress:
          - {}
```

`<app>.yaml` — one file per app, body identical across apps (only the name and registry path
change), so onboarding an app is copying the file:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: frontend
  namespace: apps-${app_env}
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "5m"
spec:
  type: OCIArtifactTag
  url: oci://${app_registry}/frontend
  insecure: ${app_registry_insecure}
  defaultValues:
    app: frontend                          # lets the template use << inputs.app >>
  filter:
    semver: "${app_semver}"                # must not start with a YAML indicator, see rules
    limit: 1
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: frontend
  namespace: apps-${app_env}
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: "10m"
spec:
  wait: true
  dependsOn:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: apps-${app_env}
      namespace: flux-system
      ready: true
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: frontend
  resources:
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: << inputs.app >>
        namespace: apps-${app_env}
      spec:
        interval: 30m
        retryInterval: 5m
        timeout: 5m
        prune: true
        wait: true
        targetNamespace: apps-${app_env}
        sourceRef:
          kind: ExternalArtifact
          name: << inputs.app >>-${app_env}  # produced by the apps ArtifactGenerator
          namespace: flux-system
        path: ./envs/${app_env}
        images:
          - name: << inputs.app >>           # base manifests use the bare app name: image: frontend
            newName: ${app_registry}/<< inputs.app >>
            newTag: << inputs.tag | quote >>
            digest: << inputs.digest | quote >>
        postBuild:
          substitute:
            app_env: ${app_env}
          substituteFrom:
            - kind: ConfigMap
              name: flux-vars                # the copy in apps-${app_env}
```

When a new tag is published the provider exports it, the ResourceSet re-renders, the
Kustomization `spec.images` changes and Flux rolls out `${app_registry}/<app>:<tag>@<digest>` —
no Git commit. Each app env has its own image policy (`app_semver`), so `test` can run release
candidates while `staging`/`prod` follow stable releases. Promotion to a prod cluster with its
own registry is copying the image there (`flux mirror`, see `references/gitless-gitops.md`).

App bases are plain manifests with **no `namespace:`** (placed via `targetNamespace`), the
container image is the bare app name, and config lives in dotenv files rendered by
`configMapGenerator` (`base/app.env` + `envs/<app_env>/app.env` with `behavior: merge`) and
consumed with `envFrom`, so the generated name hash rolls the Deployment on config changes.

### Ordering

```
infra-core RSET ──► infra-controllers RSET ──► apps-<app_env> RSET ──► <app> RSETs
(namespaces,        (HelmReleases;              (flux-vars copy,        (Kustomizations
 network policies)   <app>-configs after <app>)  network policy)         frontend, backend)
```

All ordering is at ResourceSet level with `dependsOn` (`ready: true`, `namespace` set) and
`wait: true` on every ResourceSet; the per-app-env reconciler Kustomizations run with
`wait: false` and no `dependsOn`.

### Rules That Must Hold

- **Only `clusters/<cluster>/` differs between clusters.** Everything under `infra/` and
  `apps/` is parameterised by `flux-vars` (`${env}`, `${cluster_registry}`, `${app_registry}`,
  `${app_registry_insecure}`, `${cluster_name}`) and the app-env variables (`${app_env}`,
  `${app_semver}`). Never hardcode registry hosts or environment names in shared manifests.
- **A directory that doesn't match the pattern is silently ignored**, and a missing
  `envs/<env>` overlay silently skips that component/app on that cluster. Every infra layer
  needs an overlay per cluster env; every app needs an overlay per app env.
- **Names must be unique across `infra/components` and `apps`**: both produce `<name>-<env>`
  artifacts and a `<name>` Kustomization.
- **`spec.images[].name` must be a plain image reference** in the base manifests. Kustomize
  image rewriting runs at build time, before post-build substitution, so a `${var}` there is
  never matched.
- **Substituted values must not start with a YAML indicator** (`>`, `|`, `*`, `&`, `%`, `@`,
  `[`, `{`). Kustomize drops the quotes around `"${app_semver}"` and substitution then yields
  invalid YAML. Use `x` (any), `6.14.x`, `~6.14.0`, `^6.14.0` (same major — `^` is not `>=`) or
  `x || >=0.0.0-0` to include prereleases; never `>=6.14.0`.
- The app-env **Namespace is a plain manifest**, never templated by a ResourceSet — the
  namespaced provider/ResourceSet pair must land in an existing namespace.
- Every ResourceSet carries `wait: true` and `fluxcd.controlplane.io/reconcileTimeout: "10m"`;
  the default 5m is too short for a layer of HelmReleases.

## Validation and Publishing

Validate the whole tree before pushing: build every kustomize overlay (catches broken
`../../base` references and bad patches), substitute variables from a dotenv mirror of
`flux-vars` (`env=dev`, `app_env=test`, `app_semver='x || >=0.0.0-0'` — single-quote values
with spaces, `>` or `|` since the file is shell-sourced), and schema-validate the output. The
reference repo wraps this in `scripts/validate.sh -d ./kubernetes -E ./flux/flux-vars.env`
(`make vet`); the equivalent per overlay is:

```shell
kustomize build --load-restrictor=LoadRestrictionsNone apps/frontend/envs/test \
  | flux envsubst | flux schema validate --schema-location ecosystem --skip-missing-schemas
```

Publish with `flux push artifact oci://<registry>/flux-cluster:<cluster> --path ./kubernetes`
(diff first with `flux diff artifact` to skip unchanged trees) and reconcile the root source —
see `references/gitless-gitops.md`. Mirror charts and app images into the cluster's registry
with `flux mirror sync` so reconciliation has no external dependencies.

## Day-2 Operations

- **Adding a component or app** — create the directory (and for apps the `<app>.yaml`
  reconciler + registry mirror entry); the artifact appears, the provider exports a new input,
  the ResourceSet creates the Kustomization.
- **Changing manifests** — only the affected artifact gets a new revision; only that
  Kustomization reconciles.
- **Removing** — delete the directory; the artifact is removed, the input disappears, the
  ResourceSet deletes the Kustomization, which prunes the workloads.
- **Hosting a new app env** on a cluster — add `apps/<app>/envs/<app_env>/` to every app and
  one `clusters/<cluster>/apps-<app_env>-reconcilers.yaml`.
- **Pinning or rolling back an app env** — change its `app_semver` range (e.g. `6.13.x`) and
  push; the providers re-scan and roll out the newest matching tag. Copying an older image to
  the registry does not roll back, because the policy selects the newest match.
- **Production rollout windows** — attach `spec.schedule` to the providers on prod.

## Multi-Tenancy

By default the operator lists `ExternalArtifact` objects with its own service account, which
has cluster-wide read access. To restrict discovery to a tenant's permissions, set
`spec.serviceAccountName` on the provider; when the operator runs with
`--default-service-account`, impersonation is enforced for all providers. With
`namespace: "*"` selectors, the impersonated account must hold cluster-wide `list` on
ExternalArtifacts or reconciliation fails with a forbidden error. Pair this with
`spec.serviceAccountName` on the ResourceSet so the generated Kustomizations run under the
same tenant identity. Note the production layout above uses `multitenant: false` for its
cross-namespace references; a locked-down variant keeps each tenant's artifacts, provider and
ResourceSet in the tenant namespace.

### Cross-Namespace Artifacts and Tenant Namespaces (Flux 2.10+)

source-watcher v2.3 (Flux 2.10) lets one ArtifactGenerator in a shared namespace publish each
tenant's `ExternalArtifact` into the tenant's namespace, and optionally create the namespaces:

```yaml
apiVersion: source.extensions.fluxcd.io/v1beta1
kind: ArtifactGenerator
metadata:
  name: tenants
  namespace: flux-system
spec:
  serviceAccountName: tenant-artifacts      # authorizes the work via SubjectAccessReview
  managedNamespaces:                        # opt-in namespace provisioning
    allowedNames:
      - "ns.startsWith('tenant-')"          # CEL over `ns`; default/kube-*/flux-system always rejected
    prune: true                             # deletes the WHOLE namespace when the dir is removed
  sources:
    - alias: repo
      kind: GitRepository
      name: my-monorepo
  pathPattern: "@repo/tenants/{tenant}"
  artifacts:
    - name: "{tenant}"
      namespace: "{tenant}"                 # captures are lowercased; must be a valid namespace
      copy:
        - from: "@repo/tenants/{tenant}/**"
          to: "@artifact/"
```

- `artifacts[].namespace` targets another namespace. The controller checks with
  SubjectAccessReview that `spec.serviceAccountName` (resolved in the generator's namespace) may
  `get`/`list`/`create`/`patch`/`delete` `externalartifacts` (and `patch` `externalartifacts/status`)
  in every target namespace, then acts with its own credentials — no impersonation RBAC.
- Without `serviceAccountName`, in-namespace artifacts use the controller's credentials;
  cross-namespace ones use the `--default-service-account` flag's SA (or the controller's when
  unset). On multi-tenant clusters set `--default-service-account` on source-watcher.
- `managedNamespaces` requires `serviceAccountName` or `--default-service-account`. Both that
  authorizing ServiceAccount (checked via SubjectAccessReview) and the `flux-system/source-watcher`
  ServiceAccount need a ClusterRole with `get`/`create`/`patch`/`delete` on `namespaces` (not granted
  by default; namespace verbs are cluster-scoped). Existing namespaces are adopted. Namespace metadata
  comes from `commonMetadata`, then `managedNamespaces.metadata.fromSource` (a `NamespaceMetadata`
  file in the repo, filtered by `allowedLabels`/`allowedAnnotations` CEL over `value`), then
  `metadata.from[]` (`strategy: Reset|Override|Merge`, `namespace: <name>|*`). Protect a namespace
  from pruning with the `source.extensions.fluxcd.io/prune: Disabled` annotation.
- Only enable `prune` when the generator fully owns the namespaces; it wipes anything else in them.

Other source-watcher v2.3 additions: `copy[].optional: true` skips copy operations that match no
files instead of failing, and the `source.extensions.fluxcd.io/reconcileEvery: 10m` annotation
sets the generator's reconcile interval.

## Operations

```shell
flux operator -n flux-system tree ks flux-system        # everything generated from the root
kubectl -n flux-system get externalartifacts           # did the generator match the directory?
kubectl get rset,rsip -A                               # ResourceSets and providers everywhere
flux operator get all -A                               # ResourceSets + generated Kustomizations
kubectl -n apps get rsip platform-apps -o yaml         # inspect exported inputs
flux operator -n flux-system reconcile rsip infra-controllers   # force artifact discovery now
flux operator -n apps-test reconcile rsip frontend     # force an image scan
flux operator -n apps suspend rset platform-apps       # pause generation / resume

# Render locally before applying: mock the exported inputs (one map per artifact)
flux operator build rset -f platform-apps-resourceset.yaml --inputs-from inputs.yaml
# inputs.yaml: [{name: auth-dev, namespace: apps, app: auth, env: dev}, ...]
# (--inputs-from-provider only accepts Static providers, i.e. a single input set)
```

Common failure causes: the directory doesn't match the `pathPattern`, the chart or image was
not mirrored to the cluster's registry, the manifests were not pushed, or a substituted value
produced invalid YAML in a reconciler Kustomization.

For ArtifactGenerator copy semantics and strategies see `references/sources.md`; for
ResourceSet templating, `Permute`, `dependsOn` and steps see `references/resourcesets.md`.

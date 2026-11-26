---
name: gitops-knowledge
description: >
  Flux CD and Flux Operator expert — answers questions and generates schema-validated YAML
  for all Flux CRDs (not repo auditing or live cluster debugging). Use when users ask about
  Flux concepts, want manifests for HelmRelease, Kustomization, GitRepository, OCIRepository,
  ResourceSet, FluxInstance, or any Flux resource. When user needs guidance on GitOps repository
  structure, bootstrap Flux with Terraform, multi-tenancy, OCI-based delivery, image tag automation,
  drift detection, preview environments, monorepo app delivery, running migration Jobs before
  or after a deployment, notifications, or the Flux Web UI and MCP Server.
license: Apache-2.0
---

# Flux CD Knowledge Base

You are an expert on Flux CD, the GitOps toolkit for Kubernetes. Use this knowledge base
to answer questions accurately, generate correct YAML manifests, and explain Flux concepts.

**Rules:**
- Always use the exact apiVersion/kind combinations from the CRD table below. Never invent API versions.
- Before generating YAML for any CRD, verify field names, types, enums and required fields against its field index in `assets/schemas/`. Each line is `<dotted.path> <type> [(required)] [enum=a|b] [default=x] [pattern="..."] [min=N max=N] # description` (arrays as `path[]`, maps as `<map[string]T>`). Grep a path prefix to list a subtree (`grep '^spec\.chart\.' assets/schemas/helmrelease-helm-v2.fields.txt`) or a field name to find where it lives. Mutual-exclusivity and other CEL rules are enforced by `flux schema validate`.
- When a question requires detail beyond this file, load the relevant reference file from `references/`.
- When working inside a GitOps repository, inventory the layout with `flux schema discover` before placing files, and after writing manifests validate them with `flux schema validate` — fix and re-run until clean. Load `references/flux-cli.md` for the full CLI workflow, local rendering, and overlay debugging. If the tools aren't installed, skip validation and say so.
- Prefer Flux Operator (FluxInstance) for cluster setup. Do not reference `flux bootstrap` or legacy `gotk-*` files.

## What is Flux

Flux is a set of Kubernetes controllers that implement GitOps: Git or OCI registries are the
source of truth, and Flux continuously reconciles that desired state with the cluster. Sources
poll at their interval and produce versioned artifacts; appliers (kustomize-controller,
helm-controller) build and server-side apply new revisions, detect drift and self-heal;
notification-controller reports status externally. **Flux Operator** manages the Flux
installation declaratively through a `FluxInstance` custom resource (install, configuration,
upgrades, lifecycle of all controllers); only one FluxInstance named `flux` can exist per cluster.

```
Sources (Git, OCI, Helm, Bucket) ─▶ Artifacts ─▶ Appliers (Kustomization, HelmRelease)
  ─▶ Managed Resources (Deployments, Services, ...) ─▶ Notifications (Provider + Alert)

ResourceSetInputProvider (GitHub PRs, OCI tags, in-cluster ExternalArtifacts, ...)
  ─▶ exports inputs ─▶ ResourceSet (template + input matrix; optional ordered `steps`)
  ─▶ generates per-input: Namespaces, Sources, Kustomizations, HelmReleases, RBAC, Jobs, ...
```

**Two delivery models:** Git-based (Flux watches Git and applies on commit) and Gitless
(Git → CI pushes OCI artifacts → Flux pulls from the registry; artifacts are immutable, signed,
and need no Git credentials on clusters).

## Controllers, CRDs and References

The field index for each CRD is `assets/schemas/<kind>-<group>-<version>.fields.txt` in lowercase
(e.g. `helmrelease-helm-v2.fields.txt`, `fluxinstance-fluxcd-v1.fields.txt`).

| Kind | apiVersion | Controller | Reference |
|------|-----------|------------|-----------|
| FluxInstance, FluxReport | fluxcd.controlplane.io/v1 | flux-operator | `references/flux-operator.md` |
| ResourceSet, ResourceSetInputProvider | fluxcd.controlplane.io/v1 | flux-operator | `references/resourcesets.md` |
| GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket | source.toolkit.fluxcd.io/v1 | source-controller | `references/sources.md` |
| ExternalArtifact | source.toolkit.fluxcd.io/v1 | (3rd-party controllers) | `references/sources.md` |
| ArtifactGenerator | source.extensions.fluxcd.io/v1beta1 | source-watcher | `references/sources.md` |
| Kustomization | kustomize.toolkit.fluxcd.io/v1 | kustomize-controller | `references/kustomization.md` |
| HelmRelease | helm.toolkit.fluxcd.io/v2 | helm-controller | `references/helmrelease.md` |
| Provider, Alert | notification.toolkit.fluxcd.io/v1 | notification-controller | `references/notifications.md` |
| Receiver | notification.toolkit.fluxcd.io/v1 | notification-controller | `references/notifications.md` |
| ImageRepository, ImagePolicy | image.toolkit.fluxcd.io/v1 | image-reflector-controller | `references/image-automation.md` |
| ImageUpdateAutomation | image.toolkit.fluxcd.io/v1 | image-automation-controller | `references/image-automation.md` |

| Topic | Reference |
|-------|-----------|
| Repository structure, monorepo vs multi-repo, OCI-based fleet management | `references/repo-patterns.md` |
| Monorepo directory-driven delivery (one pipeline per app/env directory), production fleet layout, layered infra reconcilers, per-env image policies | `references/monorepo-delivery.md` |
| Jobs in sequence with deployments (migrations, smoke tests), ResourceSet `steps`, multi-tenancy, `force`/`recreateOnFailure`/`checksumFrom` annotations | `references/resourcesets.md` |
| Best practices, dependency management, remediation, versioning, SPIFFE mTLS, object identity | `references/best-practices.md` |
| Gitless GitOps, Flux OCI artifacts, `flux push artifact`, registry-based delivery | `references/gitless-gitops.md` |
| Gitless image automation (ResourceSet + OCIArtifactTag) | `references/gitless-image-automation.md` |
| Flux CLI and plugins: `flux schema` discover/validate/extract, local rendering with `flux build` and `flux operator build`, overlay debugging | `references/flux-cli.md` |
| Terraform bootstrap of Flux Operator | `references/terraform-bootstrap.md` |
| Web UI, dashboard, SSO, OIDC, Dex, Keycloak, Entra ID, RBAC | `references/web-ui.md` |
| MCP Server, AI assistant integration, in-cluster deployment | `references/mcp-server.md` |

## Ordering and Reactivity

Use `dependsOn` to control reconciliation order (CRDs before CRs, infrastructure before apps):

```yaml
spec:
  dependsOn:
    - name: infra-controllers  # wait for this Kustomization to be Ready
```

ResourceSet `dependsOn` entries take `apiVersion`/`kind`/`name`/`namespace` of any resource,
plus `ready: true` or a `readyExpr` CEL expression for custom readiness. For ordering *within*
a single ResourceSet, use `spec.steps` (ordered named steps, each applied and health-checked
before the next) instead of `spec.resources` — see `references/resourcesets.md`.

Controllers poll sources at their interval. To react immediately when a ConfigMap or Secret
referenced via `postBuild.substituteFrom` or `valuesFrom` changes, label it
`reconcile.fluxcd.io/watch: Enabled`.

## Decision Trees

### Which Source Type?

- **Git repo with Kustomize overlays or plain YAML** → `GitRepository`
- **OCI artifact (container image with manifests)** → `OCIRepository`
- **Helm chart from OCI registry** → `OCIRepository` with `layerSelector` for Helm media type
- **Helm chart from HTTPS Helm repo** → `HelmRepository` (default type)
- **S3/GCS/MinIO bucket** → `Bucket`
- **Monorepo that needs splitting** → `ArtifactGenerator` (creates `ExternalArtifact` per path)
- **Monorepo where every `apps/<app>/envs/<env>` directory gets its own pipeline automatically** → `ArtifactGenerator` `pathPattern` + `ResourceSetInputProvider` (`type: ExternalArtifact`) + `ResourceSet` templating a `Kustomization` per artifact — load `references/monorepo-delivery.md`
- **Helm chart + env-specific values from Git** → `ArtifactGenerator` (composes chart with values overlay)

### Kustomization vs HelmRelease vs ResourceSet?

- **Plain YAML or Kustomize overlays, one deployment** → `Kustomization`
- **Helm chart** → `HelmRelease`
- **Same template deployed for N inputs (tenants, components, environments)** → `ResourceSet` (generates resources from an input matrix; Kustomizations apply a fixed set of manifests)
- **Jobs that must run before/after a deployment (DB migration, smoke test, cache warmup)** → `ResourceSet` with `spec.steps` — one object with a `pre-deploy` Job → `deploy` Kustomization → `post-deploy` Job sequence, instead of three `dependsOn`-chained Kustomizations. See `references/resourcesets.md` (Step-Based Reconciliation).
- Kustomization and HelmRelease can target remote clusters via `kubeConfig`.

### How to Set Up GitOps from Scratch

Install Flux Operator (Helm chart or Terraform) → create a `FluxInstance` named `flux` in
`flux-system` with `.spec.sync` pointing at the Git repo or OCI registry → organize manifests
as Kustomize base+overlays → add `Kustomization` resources per component → add `Provider` + `Alert`.

## Canonical YAML Patterns

### 1. GitOps Pipeline (GitRepository + Kustomization)

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: my-app
  namespace: flux-system
spec:
  interval: 5m
  url: https://github.com/org/my-app.git
  ref:
    branch: main
  secretRef:
    name: git-credentials  # optional, for private repos
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-app
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: my-app
  path: ./deploy/production
  prune: true
  wait: true
  timeout: 5m
```

### 2. Helm from OCI Registry (Recommended)

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: cert-manager-chart
  namespace: cert-manager
spec:
  interval: 1h
  url: oci://quay.io/jetstack/charts/cert-manager
  layerSelector:
    mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
    operation: copy
  ref:
    semver: "1.x"
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  interval: 1h
  chartRef:
    kind: OCIRepository
    name: cert-manager-chart
  install:
    strategy:
      name: RetryOnFailure
      retryInterval: 5m
  upgrade:
    strategy:
      name: RetryOnFailure
      retryInterval: 5m
  values:
    crds:
      enabled: true
```

For HTTPS Helm repositories use `HelmRepository` + `HelmRelease` with `spec.chart.spec`
(`chart`, `version: "3.x"`, `sourceRef`) instead of `chartRef`. For an OCI chart owned by one HelmRelease,
2.10+ also allows `spec.chart.kind: OCIRepository` — see `references/helmrelease.md`.

### 3. FluxInstance with OCI Sync (Gitless GitOps)

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - source-watcher
    - kustomize-controller
    - helm-controller
    - notification-controller
  cluster:
    type: kubernetes
    size: medium
    multitenant: true
    tenantDefaultServiceAccount: flux
    networkPolicy: true
  sync:
    kind: OCIRepository
    url: "oci://ghcr.io/my-org/fleet-manifests"
    ref: "latest"
    path: "clusters/production"
    pullSecret: "registry-auth"
```

### 4. ResourceSet for Multi-Component Orchestration

A `ResourceSet` lists `spec.inputs` (e.g. `tenant`/`environment` pairs) and `spec.resources`
templates that reference them with `<< inputs.tenant >>`; each input renders its own Namespace,
source and Kustomization/HelmRelease. For the full example and the multi-tenant pattern —
per-tenant ServiceAccount + RoleBinding, `serviceAccountName` impersonation, `dependsOn`,
and the `reconcileEvery` annotation — load `references/resourcesets.md`.

### 5. Image Automation

- **Git-based** — `ImageRepository` + `ImagePolicy` + `ImageUpdateAutomation` commit tag
  bumps to Git via `$imagepolicy` YAML markers; requires `image-reflector-controller` and
  `image-automation-controller`. Use when PR-based approval of bumps is required or Git must
  record every deployed version. Load `references/image-automation.md`.
- **Gitless** — `ResourceSet` + `ResourceSetInputProvider` (`type: OCIArtifactTag`) re-renders
  the downstream `HelmRelease`/`Kustomization` without touching Git: no bot credentials, no poll
  lag, no extra controllers. Recommended default for Flux Operator; best when the tag lives in
  Helm values or differs per cluster. Load `references/gitless-image-automation.md`.

### 6. Notifications

`Provider` + `Alert` (`v1`) for outgoing notifications, `Receiver` (`v1`) for incoming
webhooks. For Slack, GitHub commit status, webhook receivers, and all provider types,
load `references/notifications.md`.

## Common Mistakes

**Wrong template delimiters:**
- ResourceSet uses `<< inputs.field >>` — NOT `{{ .inputs.field }}` or `{{ inputs.field }}`
- Go templates `{{ }}` are only used in ImageUpdateAutomation `.spec.git.commit.messageTemplate`

**Mutual exclusivity:**
- HelmRelease: `spec.chart` and `spec.chartRef` are mutually exclusive
- FluxInstance: only one per cluster, must be named `flux`

**HelmRelease strategy fields:**
- Install/upgrade strategy is at `spec.install.strategy.name` and `spec.upgrade.strategy.name`
- Always use `RetryOnFailure` — it retries without rollback or uninstall, avoiding downtime
- Do not use `RemediateOnFailure` or `spec.install.remediation` / `spec.upgrade.remediation`

**OCIRepository for Helm charts:**
- Set `layerSelector` (`mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"`, `operation: copy`) to extract the chart, as in pattern 2.

**Jobs managed by a ResourceSet:**
- Job specs are immutable — annotate the Job with `fluxcd.controlplane.io/force: enabled` so a changed spec (new image tag) recreates it instead of failing the apply; add `fluxcd.controlplane.io/recreateOnFailure: enabled` only for idempotent Jobs.
- Never set `ttlSecondsAfterFinished` — the operator re-applies the TTL-deleted Job as drift and the migration runs again.
- Set `spec.wait: true` on a stepped ResourceSet, otherwise the final step is not health-checked.

**Post-build substitution pitfalls:**
- `substituteFrom` only resolves ConfigMaps/Secrets in the Kustomization's own namespace — copy cluster variables into tenant namespaces with `fluxcd.controlplane.io/copyFrom` (ResourceSet) rather than referencing `flux-system` from elsewhere.
- Kustomize `spec.images[].name` must match a plain image reference in the manifests (`image: frontend`); image rewriting runs at build time, before `${var}` substitution.
- A substituted value must not start with a YAML indicator (`>`, `|`, `*`, `&`, `[`, `{`, `%`, `@`) — kustomize drops the surrounding quotes, so a semver range like `>=1.0.0` yields invalid YAML. Use `x`, `1.x`, `~1.2.0`, `^1.2.0` or `x || >=0.0.0-0`.

**Drift control — pick the right knob:**
- Kustomization `spec.ignore` — exclude specific JSON-pointer fields from drift detection/apply (e.g. HPA `replicas`). Distinct from the `kustomize.toolkit.fluxcd.io/ssa: Ignore` annotation, which skips a whole object.
- HelmRelease `spec.driftDetection.ignore` — the HelmRelease equivalent, only active when `driftDetection.mode` is `warn`/`enabled`.

# Flux Operator API Summary

Condensed reference for the Flux Operator CRDs.

## FluxInstance (`fluxcd.controlplane.io/v1`)

Field index: assets/schemas/fluxinstance-fluxcd-v1.fields.txt

Manages the Flux controllers installation and configuration.

**Key fields:**
- `.spec.distribution.version` — Flux version semver range (e.g., `"2.x"`)
- `.spec.distribution.registry` — Container registry for Flux images
- `.spec.distribution.artifact` — OCI artifact URL for automated updates
- `.spec.components[]` — List of controllers to install (source-controller, source-watcher, kustomize-controller, helm-controller, notification-controller, image-reflector-controller, image-automation-controller)
- `.spec.cluster.type` — `kubernetes`, `openshift`, `aws`, `gcp`, `azure` (enables platform-specific optimizations)
- `.spec.cluster.size` — Vertical scaling profile: `small` (5 concurrency, 512Mi), `medium` (10 concurrency, 1Gi), `large` (20 concurrency, 3Gi). Sets CPU/memory limits and concurrency for kustomize-controller and helm-controller. Recommend `medium` or `large` for up to a thousand. For thousands of apps, use sharding instead.
- `.spec.cluster.multitenant` — Enable multi-tenancy lockdown: sets default service account via `tenantDefaultServiceAccount` (defaults to `"default"`), enables `--no-cross-namespace-refs=true` and `--default-service-account` on all controllers. When enabled, individual Kustomizations/HelmReleases don't need `serviceAccountName`
- `.spec.cluster.objectIdentityRBAC` / `.spec.cluster.multitenantObjectIdentity` — Grant controllers RBAC for object identity (ServiceAccount tokens) and enable the object identity lockdown (tenants default to `tenantDefaultServiceAccount`, `tenantDefaultDecryptionServiceAccount`, `tenantDefaultKubeConfigServiceAccount`). `multitenantObjectIdentity` requires `objectIdentityRBAC: true`. Legacy names `objectLevelWorkloadIdentity` / `multitenantWorkloadIdentity` are still accepted but must not be combined with the new ones
- `.spec.cluster.networkPolicy` — Deploy network policies for controller pods (default: `true`)
- `.spec.sync` — Configure Git/OCI sync for cluster reconciliation (kind, url, ref, path, pullSecret, provider)
- `.spec.sync.provider` — OIDC-based auth provider. For `GitRepository`: `github` (GitHub App auth) or `azure`. For `OCIRepository`/`Bucket`: `aws`, `azure`, `gcp`. When the sync URL points to GitHub (`github.com`), recommend `provider: github` with GitHub App authentication to avoid reliance on personal access tokens.
- `.spec.kustomize.patches[]` — Strategic merge patches for controller deployments, used for custom configuration (affinity, tolerations, extra env vars)

**Patterns:**
- Should use `.spec.sync` to point at the cluster's config directory in the fleet repo.

**Gotchas:**
- Only one FluxInstance allowed per cluster. Name must be `flux`.
- Network policies are enabled by default — omitting `networkPolicy` means policies ARE deployed.

## ResourceSet (`fluxcd.controlplane.io/v1`)

Field index: assets/schemas/resourceset-fluxcd-v1.fields.txt

Generates groups of Kubernetes resources from a matrix of input values with templated resources.

**Key fields:**
- `.spec.inputs[]` — Array of inline input objects (each becomes a template iteration)
- `.spec.inputsFrom[]` — References to ResourceSetInputProvider objects that supply dynamic inputs
- `.spec.resources[]` — Templated Kubernetes resources using `<< inputs.field >>` syntax
- `.spec.resourcesTemplate` — Alternative to `resources[]`: a single multi-document YAML string, useful for complex templating with conditionals and range loops
- `.spec.steps[]` — Ordered, named reconciliation steps (max 20), each with its own `resources`/`resourcesTemplate` and an optional `timeout`. Mutually exclusive with `resources`/`resourcesTemplate`. Each step is applied and health-checked before the next one starts; the **final** step is health-checked only when `.spec.wait: true`. Used to run Jobs before/after a deployment (`pre-deploy` Job → `deploy` Kustomization → `post-deploy` Job) instead of three `dependsOn`-chained Kustomizations
- `.spec.wait` — Health-check all applied resources (default `false`)
- `.spec.commonMetadata` — Labels/annotations applied to all generated resources
- `.spec.serviceAccountName` — Service account for impersonation when applying the generated resources
- `.spec.dependsOn[]` — Kubernetes objects that must be ready first (any kind: ResourceSet, Kustomization, HelmRelease, CRDs, etc.). Entries should always set `namespace` so cross-namespace dependencies resolve unambiguously; `ready: true` waits for the Ready condition, `readyExpr` evaluates a CEL expression against the dependency's status

**Reconciliation annotations** (`fluxcd.controlplane.io/*`) — set on the ResourceSet object; `force`, `recreateOnFailure` and `prune` also work on individual generated resources:

| Annotation | Default | Meaning |
|---|---|---|
| `reconcile` | `enabled` | `disabled` pauses reconciliation |
| `reconcileEvery` | `1h` | Drift detection/correction interval |
| `reconcileTimeout` | `5m` | Timeout including health checks; also the default per-step `timeout` |
| `force` | — | `enabled`: delete and recreate resources whose immutable fields changed (Job specs, some Service/PVC fields) instead of failing the apply |
| `recreateOnFailure` | — | `enabled` on a Job: delete it when `Failed=True` and recreate it on the next reconciliation — only safe for idempotent Jobs |
| `prune` | `enabled` | `disabled` excludes a generated resource from garbage collection |
| `copyFrom` | — | On a generated ConfigMap/Secret: `namespace/name` of a source object whose data is copied in and kept in sync (Secrets also need `type`). Resolved before the first step runs — not step-aware |
| `checksumFrom` | — | On a **pod template**: comma-separated `Kind/namespace/name` list of ConfigMaps/Secrets; the operator writes a SHA256 of their data to `fluxcd.controlplane.io/checksum` so the workload rolls when the data changes |

**Patterns:**
- Application definitions — group Flux and Kubernetes resources into a single templated unit deployed across environments.
- Multi-tenant app deployment — one input per tenant generates namespace, RBAC, source, and Kustomization.
- Namespace-as-a-Service — auto-provision namespaces for feature/long-lived branches, giving developers self-service infrastructure in a GitOps manner.
- Time-based delivery — define deployment windows based on time intervals or specific dates for controlled rollouts.
- Staged deployments with Jobs — `steps` run a migration Job, then the app Kustomization/HelmRelease, then a smoke-test Job, in one object with one status and inventory.
- Directory-driven monorepo delivery — `inputsFrom` an `ExternalArtifact` provider and template one `Kustomization` (`sourceRef.kind: ExternalArtifact`) per generated artifact; see [repo-patterns.md](repo-patterns.md).

**Gotchas:**
- Template syntax uses `<< >>` delimiters (not `{{ }}`).
- `<< inputs.provider.name >>` and `<< inputs.provider.namespace >>` resolve to the ResourceSet's metadata when using inline `inputs`, or to the ResourceSetInputProvider's metadata when using `inputsFrom`.
- Supports slim-sprig functions (`quote`, `int`, `toYaml`, `nindent`, `get`, `default`, `sha256sum`, etc.) plus a custom `slugify` function.
- **Jobs are immutable**: a Job whose rendered spec changes (new image tag from an input) fails the apply unless it carries `fluxcd.controlplane.io/force: enabled`. Never set `ttlSecondsAfterFinished` on a Job managed by a ResourceSet — once the TTL controller deletes it, the operator sees drift and re-applies it, so the Job runs again. `recreateOnFailure: enabled` re-runs a failed Job on every reconciliation until it succeeds, which repeats partial changes of a non-idempotent migration.
- **Step timeouts**: a step wrapping a Kustomization or HelmRelease must have a `timeout` longer than that applier's own `spec.timeout`, otherwise the step times out before the applier reports a result. Without an explicit step `timeout` the `reconcileTimeout` annotation (default 5m) applies.
- **Steps fail fast**: a failing step blocks later steps, and garbage collection runs only after all steps succeed. A resource may be defined in only one step.
- The namespace that hosts a namespaced ResourceSet or ResourceSetInputProvider must exist before they are applied, so it is a plain manifest in the parent Kustomization, not a resource templated by that ResourceSet.

## ResourceSetInputProvider (`fluxcd.controlplane.io/v1`)

Field index: assets/schemas/resourcesetinputprovider-fluxcd-v1.fields.txt

Fetches input values from external services (or from in-cluster objects) for ResourceSet consumption.

**Key fields:**
- `.spec.type` — Provider type. Full list: `Static` (inline inputs), `GitHubBranch`/`GitHubTag`/`GitHubPullRequest`, `GitLabBranch`/`GitLabTag`/`GitLabMergeRequest`/`GitLabEnvironment`, `AzureDevOpsBranch`/`AzureDevOpsTag`/`AzureDevOpsPullRequest`, `AWSCodeCommitBranch`/`AWSCodeCommitTag`/`AWSCodeCommitPullRequest`, `GiteaBranch`/`GiteaTag`/`GiteaPullRequest`, `OCIArtifactTag`/`ACRArtifactTag`/`ECRArtifactTag`/`GARArtifactTag`, `ExternalArtifact` (in-cluster `ExternalArtifact` objects matched by label selectors), `ExternalService` — do not flag a type as invalid without checking this list or the schema
- `.spec.url` — Repository or registry URL. Required for external types; must **not** be set for `Static` or `ExternalArtifact`
- `.spec.selectors[]` — `ExternalArtifact` only (required for it, forbidden otherwise). Each entry: `matchLabels`/`matchExpressions`, or `name` (mutually exclusive with the label selectors), plus optional `namespace` — empty = the provider's namespace, `"*"` = all namespaces, else that namespace. Entries are OR-ed. Exported inputs per artifact: `id`, `name`, `namespace`, `revision` plus **every label** on the artifact (e.g. the `{app}`/`{env}` captures from an ArtifactGenerator `pathPattern`)
- `.spec.filter.labels[]` — Label filter for PRs/MRs
- `.spec.filter.semver` — Semver range to filter and sort tags (`*ArtifactTag` types); combined with `filter.limit: 1` it selects the newest matching version
- `.spec.filter.limit` — Maximum number of inputs to export (default 100) — raise it for monorepos with more matching directories
- `.spec.defaultValues` — Default key-value pairs merged into every exported input (e.g. `app: frontend` so the ResourceSet body can template on `<< inputs.app >>`)
- `.spec.schedule[]` — Cron windows (`cron`, `timeZone`, `window`) restricting when inputs are refreshed (rollout windows)
- `.spec.secretRef` — Authentication secret reference
- `.spec.serviceAccountName` — Service account for workload identity (AzureDevOps*, AWSCodeCommit*, *ArtifactTag types) and for impersonation when listing `ExternalArtifact` objects; with `namespace: "*"` selectors that account needs cluster-wide `list` on ExternalArtifacts

**Patterns:**
- Change request preview environments — Provider (`GitHubPullRequest`, `GitLabMergeRequest`, `AzureDevOpsPullRequest`, `GiteaPullRequest`) fetches open PRs, ResourceSet creates ephemeral environments per PR.
- Gitless image automation — Instead of pushing image tags to Git (Flux image-automation-controller), use `OCIArtifactTag`/`ACRArtifactTag`/`ECRArtifactTag`/`GARArtifactTag` providers to scan registries for new versions. The provider exports `tag` and `digest` per image, and a ResourceSet injects them into HelmRelease values or Kustomization image overrides using `<< inputs.name.tag >>@<< inputs.name.digest >>` (or `spec.images[].newTag` + `digest`). Updates apply directly to the cluster without Git commits.
- Directory-driven monorepo delivery — `ArtifactGenerator` with `pathPattern` labels one `ExternalArtifact` per matching directory, an `ExternalArtifact` provider exports one input per artifact, and a ResourceSet templates a `Kustomization` per input. Requires the `source-watcher` component on the FluxInstance and Flux >= 2.9.

**Gotchas:**
- Reconciliation interval is the `fluxcd.controlplane.io/reconcileEvery` annotation (default 10m), not a spec field.
- When `filter.semver` is fed through Flux post-build substitution (`semver: "${app_semver}"`) the value must not start with a YAML indicator (`>`, `|`, `*`, `&`, `%`, `@`, `[`, `{`) — kustomize drops the quotes and `>=1.0.0` yields invalid YAML. Use `x`, `1.x`, `~1.2.0`, `^1.2.0`, or `x || >=0.0.0-0` for prereleases.

## Deep Dive API Specs

Do NOT fetch these URLs unless you need to look up a specific field or behavior not covered above.

- FluxInstance: https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/fluxinstance.md
- ResourceSet: https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/resourceset.md
- ResourceSetInputProvider: https://raw.githubusercontent.com/controlplaneio-fluxcd/flux-operator/refs/heads/main/docs/api/v1/resourcesetinputprovider.md

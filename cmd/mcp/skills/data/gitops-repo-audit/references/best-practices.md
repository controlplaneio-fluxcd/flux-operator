# Flux GitOps Best Practices

Assessment checklist for GitOps repository analysis. Each item is a recommendation —
not all apply to every repo. Judge based on the repo's complexity and maturity.

## Repository Structure

- [ ] **Clear top-level separation**: `apps/`, `infrastructure/`, `clusters/` (monorepo) or dedicated repos for fleet/infra/apps (multi-repo)
- [ ] **Base + overlay pattern**: Shared base configurations with per-environment overlays using Kustomize patches — avoids duplicating entire manifests across environments
- [ ] **One directory per cluster** under `clusters/` with cluster-specific Flux bootstrap or FluxInstance and reconciliation config
- [ ] **Separate controllers and configs** under `infrastructure/`: controllers install CRDs and operators, configs create custom resources that depend on those CRDs
- [ ] **ArtifactGenerator for monorepos**: When using a monorepo, split the source into independent ExternalArtifacts so infra changes don't trigger app reconciliation and vice versa
- [ ] **Components match the CRDs in use**: compare the FluxInstance `.spec.components` (or the bootstrap components) with the kinds the repo declares and state the result either way. Only source, kustomize, helm and notification controllers are installed by default:
  - `ArtifactGenerator` needs `source-watcher`. Without it nothing generates the ExternalArtifacts and every Kustomization pointing at them stays stuck — flag as Critical
  - `ImageRepository`/`ImagePolicy` need `image-reflector-controller` and `ImageUpdateAutomation` needs `image-automation-controller`. Without them image automation silently does nothing — flag as Critical; when they are listed, note it as a positive
- [ ] **Directory-driven pipelines copy base + overlay**: An ArtifactGenerator with `pathPattern` whose overlays use `resources: [../../base]` must copy both `base/**` and `envs/{env}/**` into the artifact preserving the layout. Read every `copy` entry — an artifact holding only the overlay directory fails to build in-cluster even though `kustomize build` of the repo succeeds locally
- [ ] **No Flux bootstrap manifests in app/infra dirs**: `flux-system/` (gotk-components, gotk-sync or FluxInstance) belongs under `clusters/`, not mixed with app resources
- [ ] **Runtime info substitution**: Use ConfigMaps with `postBuild.substituteFrom` for cluster-specific variables (environment, domain, branch) rather than hardcoding values

## Dependency Management

- [ ] **Explicit dependency chains**: Use `dependsOn` on Kustomizations or ResourceSets to enforce ordering — typically `infra-controllers` → `infra-configs` → `apps`
- [ ] **CRDs before custom resources**: Infrastructure controllers (that install CRDs) must be ready before configs that create CRs using those CRDs
- [ ] **`wait: true` on dependencies**: Kustomizations and ResourceSets that other resources depend on should set `wait: true` so dependents only start after all resources are healthy
- [ ] **`namespace` on ResourceSet `dependsOn`**: Every `spec.dependsOn[]` entry on a ResourceSet should set `namespace` (and `ready: true` or a `readyExpr`) so the dependency resolves unambiguously, especially when the dependent lives in another namespace
- [ ] **HelmRelease dependencies**: Use `dependsOn` when one Helm release requires another (e.g., ingress-nginx depends on cert-manager for TLS)
- [ ] **No circular dependencies**: Verify `dependsOn` chains form a DAG (directed acyclic graph)
- [ ] **No overlapping Kustomization paths**: Kustomizations sharing the same source must not have paths where one is a prefix of the other — the parent path includes the child's resources, causing apply conflicts and unpredictable pruning

## Remediation and Reliability

- [ ] **Install/upgrade strategies on HelmReleases**: Use the modern `install.strategy.name: RetryOnFailure` and `upgrade.strategy.name: RetryOnFailure`.
  The legacy `install.remediation.retries` / `upgrade.remediation.retries` pattern still works but should be flagged for migration
- [ ] **Retry intervals**: Set `retryInterval` on Kustomizations and HelmReleases to avoid overwhelming the API server on failures
- [ ] **Timeouts**: Set `timeout` on Kustomizations and HelmReleases
- [ ] **Drift detection**: Enable `driftDetection.mode: enabled` on production HelmReleases to detect and correct out-of-band changes
- [ ] **Drift detection ignores**: Use `driftDetection.ignore` for fields that are expected to change (e.g., `/spec/replicas` for HPA-managed deployments, annotation fields set by operators)
- [ ] **storageNamespace on HelmReleases**: If `targetNamespace` is set and `storageNamespace` is not, flag it and recommend setting `storageNamespace` to match `targetNamespace` to avoid Helm storage being in a different namespace than the deployed resources
- [ ] **CRD handling**: Set `install.crds: Create` and `upgrade.crds: CreateReplace` on HelmReleases that manage CRDs
- [ ] **Reactivity**: ConfigMaps and Secrets used in `valuesFrom` and `postBuild.substituteFrom` should be labeled with `reconcile.fluxcd.io/watch: Enabled` to trigger immediate reconciliation on changes instead of waiting for the next interval

## Versioning

- [ ] **Semver ranges on charts**: Use semver constraints (e.g., `>=1.0.0`, `6.5.*`) instead of `*` or `latest`
- [ ] **Environment-differentiated versions**: Staging uses broader ranges (e.g., `>=1.0.0-alpha`) for early adoption; production uses stable-only ranges (e.g., `>=1.0.0`)
- [ ] **Pinned source refs**: Use specific branches, tags, or semver for GitRepository/OCIRepository — avoid `latest` in production
- [ ] **OCI artifact tagging**: Use immutable tags (semver or digest) for production; `latest` or `*` only for staging/development

## Namespace Isolation

- [ ] **Per-app namespaces**: Each application and set of microservices deployed in dedicated namespace to limit blast radius
- [ ] **Tenant labels**: Use `toolkit.fluxcd.io/tenant` labels on namespaces for multi-tenancy grouping
- [ ] **`targetNamespace` on Kustomizations**: Set when the source manifests don't include namespace metadata for apps
- [ ] **Namespace created by Kustomization or ResourceSet**: Verify that target namespaces are created as part of the Kustomization or ResourceSet that deploys the component. Flag usage of `targetNamespace` or `createNamespace` in HelmRelease — these bypass proper namespace lifecycle management. The namespace should exist before the HelmRelease runs, created by the parent Kustomization or ResourceSet template. Exception: the namespace that *hosts* a namespaced ResourceSet or ResourceSetInputProvider must be a plain manifest applied by the parent Kustomization (it has to exist before those objects are applied) — do not flag that as bypassing the ResourceSet
- [ ] **No workloads in the default namespace**: HelmReleases and Kustomizations should deploy to a dedicated namespace, not the `default` namespace

## ResourceSet Pipelines and Jobs

Applies to repos with `ResourceSet` resources — especially ones using `spec.steps` or generating `Job` objects.
Verify annotation names against `assets/schemas/resourceset-fluxcd-v1.fields.txt` and [flux-operator-api-summary.md](flux-operator-api-summary.md).

- [ ] **`wait: true` on stepped ResourceSets**: With `spec.steps`, each step is health-checked before the next regardless of `wait`, but the **final** step is only health-checked when `spec.wait: true`. A ResourceSet with steps and no `wait: true` reports Ready before its last stage (post-deploy Job, smoke test) has finished — flag it
- [ ] **`force` on Jobs**: Job specs are immutable. A Job templated by a ResourceSet whose spec changes between reconciliations (an image tag from an input, a templated command) must carry `fluxcd.controlplane.io/force: enabled` or the apply fails on the first change. Flag Jobs under a ResourceSet without it
- [ ] **No `ttlSecondsAfterFinished` on ResourceSet-managed Jobs**: When the TTL controller deletes a completed Job, the operator sees a missing resource as drift on the next reconciliation and re-applies it — the migration runs again unexpectedly. Recommend removing the TTL; to keep history per version, template the Job name from the revision instead (`db-migration-<< inputs.tag >>`)
- [ ] **`recreateOnFailure` only on idempotent Jobs**: `fluxcd.controlplane.io/recreateOnFailure: enabled` deletes a failed Job and re-runs it on every reconciliation until it succeeds. On a schema migration or any non-idempotent Job this repeats partial changes — flag as Warning and ask whether the Job is safe to re-run
- [ ] **Step timeouts exceed wrapped applier timeouts**: A step containing a Kustomization or HelmRelease must have a `timeout` longer than that object's own `spec.timeout` (otherwise the step gives up before the applier reports). Without a step `timeout`, the ResourceSet `fluxcd.controlplane.io/reconcileTimeout` annotation applies (default 5m) — compare against it
- [ ] **Version changes reach the applier spec**: For a `deploy` step to actually wait for a rollout, the new version must change the Kustomization/HelmRelease spec (`spec.images[].newTag`, Helm `values`). If only a ConfigMap changes, add `fluxcd.controlplane.io/checksumFrom` on the pod template (workload defined in the ResourceSet) or a checksum annotation patch (workload behind a Kustomization) so the workload rolls
- [ ] **Digest pinning with registry providers**: Kustomizations or HelmReleases fed by an `OCIArtifactTag`/`*ArtifactTag` provider should pin `digest` as well as `tag` (`spec.images[].digest: << inputs.digest >>` or `tag@digest` in values) so the deployed image is immutable
- [ ] **`reconcileTimeout` sized for the layer**: A ResourceSet whose generated Kustomizations install a layer of HelmReleases usually needs `fluxcd.controlplane.io/reconcileTimeout` above the 5m default (e.g. `10m`) when `wait: true`

## Post-Build Substitution

Applies to Kustomizations with `postBuild.substitute`/`substituteFrom` and the ConfigMaps/Secrets they read.

- [ ] **Substituted values are YAML-safe**: Kustomize drops the quotes around `"${var}"`, so a value starting with a YAML indicator (`>`, `|`, `*`, `&`, `%`, `@`, `[`, `{`) produces invalid YAML after substitution. Check `postBuild.substitute` literals and the `data` of referenced ConfigMaps — a semver range like `>=1.0.0` is the classic case; recommend `1.x`, `x`, `~1.2.0`, `^1.2.0` or `x || >=0.0.0-0`
- [ ] **`spec.images[].name` matches the manifest image exactly**: Kustomize matches `images[].name` against the image reference *as written in the manifests* — if the base says `image: frontend`, the name must be `frontend`, not `registry/frontend` or `${app_registry}/frontend`. Compare every `Kustomization.spec.images[].name` (and `kustomization.yaml` `images[].name`) with the rendered base manifests; a mismatch is a silent no-op and the workload runs the unpinned base image. Note that a `${var}` inside the same Kustomization's `images[].name` can never work (image rewriting runs at build time, before that Kustomization's own post-build substitution); a variable resolved by an *outer* Kustomization does expand, but the expanded value still has to equal the manifest image. `newName`/`newTag`/`digest` may use variables freely
- [ ] **`substituteFrom` targets the same namespace**: `substituteFrom` only resolves ConfigMaps/Secrets in the Kustomization's own namespace. If the referenced object exists only in another namespace (e.g. `flux-system/flux-vars` read from `apps-test`), the Kustomization fails — recommend copying it into the tenant namespace with a ResourceSet-generated ConfigMap carrying `fluxcd.controlplane.io/copyFrom: flux-system/flux-vars` (with the `reconcile.fluxcd.io/watch: Enabled` label)

## Security & Multi-Tenancy

See [security-audit.md](security-audit.md) for the full security audit checklist covering
secrets management, source authentication, OCI supply chain, RBAC, multi-tenancy,
network policies, and image automation security.

## Flux Operator

- [ ] **Cluster size**: If a FluxInstance is present, ensure `.spec.cluster.size` is set (`small`, `medium`, or `large`). This configures vertical scaling (CPU/memory limits, concurrency) for kustomize-controller and helm-controller appropriate to the cluster's workload. Without it, controllers use default resource limits which may be insufficient for larger deployments.
- [ ] **GitHub App auth for sync**: If a FluxInstance uses `sync.kind: GitRepository` with a GitHub URL, recommend setting `sync.provider: github` with GitHub App authentication to avoid reliance on personal access tokens. The secret should contain `githubAppPrivateKey`, `githubAppInstallationID` (or `githubAppInstallationOwner`), and exactly one of `githubAppID` or `githubAppClientID` (Flux 2.10+).
- [ ] **Supported Flux version**: `.spec.distribution.version` should track a supported minor (`2.x` or `2.10.x`). Flux 2.7.x and older are end of life; Flux 2.10 needs Kubernetes 1.35+.
- [ ] **Migrate from bootstrap to Flux Operator**: If `gotk-sync.yaml` is found in the repo (generated by `flux bootstrap`),
  recommend migrating to the Flux Operator with a `FluxInstance` resource for declarative Flux lifecycle management.
  Recommend https://fluxoperator.dev/docs/guides/migration/ for zero-downtime migration steps.

## Operational Excellence

- [ ] **Alerts configured**: At minimum, error-severity Alerts with a Provider (Slack, Teams, etc.) for production clusters
- [ ] **Receivers for webhooks**: Configure Receivers to trigger immediate reconciliation on Git push instead of waiting for polling interval
- [ ] **Appropriate intervals**: Sources polled frequently (5m-15m), reconciliation intervals longer (30m-1h), drift detection at reconciliation interval
- [ ] **CI validation pipeline**: Run `validate.sh` (flux-schema strict + CEL validation + kustomize build) in CI before merging
- [ ] **`prune: true` on all Kustomizations**: Enables garbage collection of resources removed from source
- [ ] **Image automation**: For container images that need automatic updates, configure ImageRepository + ImagePolicy + ImageUpdateAutomation
- [ ] **Monitoring**: Deploy kube-prometheus-stack or similar with ServiceMonitors/PodMonitors for Flux controllers

## Up-to-date API versions

- [ ] **Current API versions**: All Flux resources use the latest stable API versions (see CRD version table in SKILL.md)
- [ ] **No deprecated fields**: HelmRelease uses `install.strategy`/`upgrade.strategy` instead of legacy `install.remediation`/`upgrade.remediation` pattern
- [ ] **`chartRef` for OCI**: HelmReleases referencing OCI charts use `.spec.chartRef` (pointing to OCIRepository) instead of inline `.spec.chart.spec` with HelmRepository
- [ ] **No `HelmRepository` with `type: oci`**: `HelmRepository` with `.spec.type: oci` is a legacy pattern. Migrate to `OCIRepository` with `.spec.chartRef` on the HelmRelease instead — it supports signature verification, semver policies, and layer selection that `HelmRepository` OCI mode does not
- [ ] **Notification v1**: Alert and Provider use `notification.toolkit.fluxcd.io/v1` (`v1beta3` is deprecated in 2.10, still served). No Alert `spec.summary`, Provider `spec.proxy` or `spec.interval` — `v1` drops them silently (see [api-migration.md](api-migration.md))
- [ ] **Run `flux migrate`**: Use `flux migrate -f . --dry-run` (Flux CLI 2.10+) to detect and automatically fix deprecated API versions and removed notification fields

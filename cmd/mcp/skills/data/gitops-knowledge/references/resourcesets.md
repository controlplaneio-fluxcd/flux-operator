# ResourceSet Reference

`apiVersion: fluxcd.controlplane.io/v1` — managed by flux-operator.

ResourceSet generates Kubernetes resources from a matrix of input values using a template-based
approach. It is the primary mechanism for multi-tenant orchestration, fleet management, and
self-service platforms.

For monorepos where each tenant is a directory, Flux 2.10 also lets an ArtifactGenerator publish
per-tenant `ExternalArtifact`s into tenant namespaces and provision those namespaces
(`spec.managedNamespaces`); an `ExternalArtifact` provider then feeds the ResourceSet. See
`references/monorepo-delivery.md` (Cross-Namespace Artifacts and Tenant Namespaces).

**Contents:** [Canonical YAML](#canonical-yaml) | [Key Spec Fields](#key-spec-fields) | [Template Syntax](#template-syntax) | [Testing ResourceSets Locally](#testing-resourcesets-locally) | [Input Strategies](#input-strategies) | [Dependencies](#dependencies) | [Step-Based Reconciliation](#step-based-reconciliation) (Jobs before/after a deploy) | [Advanced Features](#advanced-features) (annotations, `copyFrom`, `checksumFrom`) | [Built-in Input Fields](#built-in-input-fields) | [ResourceSetInputProvider](#resourcesetinputprovider) | [Use Cases](#use-cases)

## Canonical YAML

Based on the Gitless reference architecture fleet pattern — deploys per-tenant namespaces with
OCIRepository sources and Kustomizations:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: apps
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "5m"
spec:
  dependsOn:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: infra
      ready: true
  inputs:
    - tenant: "frontend"
      tag: "${ARTIFACT_TAG}"
      environment: "${ENVIRONMENT}"
    - tenant: "backend"
      tag: "${ARTIFACT_TAG}"
      environment: "${ENVIRONMENT}"
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: << inputs.tenant >>
        labels:
          toolkit.fluxcd.io/role: "tenant"
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: flux-runtime-info
        namespace: << inputs.tenant >>
        annotations:
          fluxcd.controlplane.io/copyFrom: "flux-system/flux-runtime-info"
        labels:
          reconcile.fluxcd.io/watch: Enabled
    - apiVersion: v1
      kind: Secret
      metadata:
        name: registry-auth
        namespace: << inputs.tenant >>
        annotations:
          fluxcd.controlplane.io/copyFrom: "flux-system/registry-auth"
      type: kubernetes.io/dockerconfigjson
    - apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: flux
        namespace: << inputs.tenant >>
      imagePullSecrets:
        - name: registry-auth
    - apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: flux
        namespace: << inputs.tenant >>
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: admin
      subjects:
        - kind: ServiceAccount
          name: flux
          namespace: << inputs.tenant >>
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: apps
        namespace: << inputs.tenant >>
      spec:
        interval: 5m
        serviceAccountName: flux
        url: "oci://ghcr.io/my-org/apps/<< inputs.tenant >>"
        ref:
          tag: << inputs.tag >>
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: apps
        namespace: << inputs.tenant >>
      spec:
        targetNamespace: << inputs.tenant >>
        serviceAccountName: flux
        interval: 30m
        retryInterval: 5m
        wait: true
        timeout: 5m
        sourceRef:
          kind: OCIRepository
          name: apps
        path: "./<< inputs.environment >>"
        prune: true
        postBuild:
          substituteFrom:
            - kind: ConfigMap
              name: flux-runtime-info
```

## Key Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| `inputs` | array | List of input value maps — each entry generates one set of resources |
| `inputsFrom` | array | References to ResourceSetInputProvider resources |
| `inputStrategy.name` | string | `Flatten` (default) or `Permute` (Cartesian product) |
| `resources` | array | Templated Kubernetes resource definitions |
| `resourcesTemplate` | string | Go-template string rendered as multi-document YAML (alternative to `resources`) |
| `steps` | array | Ordered, named reconciliation steps — each applied and health-checked before the next (mutually exclusive with `resources`/`resourcesTemplate`) |
| `wait` | bool | Health-check all applied resources (default `false`). With `steps`, gates the health check of the **final** step |
| `commonMetadata` | object | Labels/annotations applied to all generated resources |
| `dependsOn` | array | Prerequisites with optional readiness checks |
| `serviceAccountName` | string | Impersonate this SA (in the ResourceSet namespace) when applying — scopes what the templates may create |

## Template Syntax

**Delimiters:** `<< >>` — NOT `{{ }}`

| Expression | Description |
|-----------|-------------|
| `<< inputs.name >>` | Simple field substitution |
| `<< inputs.name \| quote >>` | Quote the value (wraps in double quotes) |
| `<< inputs.replicas \| int >>` | Convert to integer (for numeric YAML fields) |
| `<< inputs.config \| toYaml \| nindent 4 >>` | Render as YAML with indentation |
| `<< inputs.config \| toJson >>` | Render as JSON string |
| `<< inputs.name \| slugify >>` | Convert to DNS-safe slug |
| `<< inputs.labels \| get "app" >>` | Get value from nested map |
| `<< inputs.tag \| default "latest" >>` | Default value if empty |
| `<< inputs.name \| upper >>` | Convert to uppercase |
| `<< inputs.name \| lower >>` | Convert to lowercase |
| `<< inputs.name \| trimSuffix "-app" >>` | Remove suffix |
| `<< inputs.config \| toYaml \| sha256sum \| quote >>` | Hash a value — inject into a pod-template annotation to roll workloads on data-only input changes |

Template functions come from slim-sprig (a subset of Go's sprig template functions).

**Important:** Template expressions are evaluated per input entry. For each entry in
`inputs`, the entire `resources` array is rendered once.

## Testing ResourceSets Locally

Render the generated Kubernetes objects without a cluster using the Flux CLI:

```shell
# Inline inputs (spec.inputs) — renders directly
flux operator build resourceset -f resourceset.yaml

# Inputs from a provider (spec.inputsFrom) — supply the exported inputs from a file
flux operator build resourceset -f resourceset.yaml --inputs-from inputs.yaml

# Or supply Static ResourceSetInputProvider manifests
flux operator build resourceset -f resourceset.yaml --inputs-from-provider providers.yaml

# Validate the generated objects against the Flux/Kubernetes schemas
flux operator build resourceset -f resourceset.yaml --inputs-from inputs.yaml | flux schema validate
```

## Input Strategies

### Flatten (Default)

Inputs from `inputs` and `inputsFrom` are concatenated into a flat list.
Each input entry produces one set of resources.

```yaml
spec:
  inputs:
    - tenant: "frontend"
    - tenant: "backend"
  # Produces 2 sets of resources (one per tenant)
```

### Permute (Cartesian Product)

`Permute` computes the Cartesian product of all input sources. In practice, the
**primary reason teams use `Permute` is not the cross-product but the namespaced field
access** it provides: fields from each source are placed under a key named after the
source object, so values from different providers (or from inline `.spec.inputs`)
don't collide. The canonical shape uses `limit: 1` on every
`ResourceSetInputProvider`, yielding exactly one permutation.

**Canonical shape — multiple providers, one permutation.** Combining chart version +
image tag + image tag for a single `HelmRelease` (see
`references/gitless-image-automation.md` for the full image-automation pattern):

```yaml
spec:
  inputStrategy:
    name: Permute
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: chart-version      # limit: 1 → exports one tag
    - kind: ResourceSetInputProvider
      name: image-tag          # limit: 1 → exports one tag+digest
  # 1 × 1 = 1 permutation. Inside templates:
  #   << inputs.chart_version.tag >>
  #   << inputs.image_tag.tag >>@<< inputs.image_tag.digest >>
```

Without `Permute`, both providers' fields would merge into a flat `inputs.tag`, which
would clash. `Permute` keeps them under distinct keys.

**True cross-product — static dimensions × one provider.** When an actual Cartesian
product is wanted, combine an inline `.spec.inputs` list of dimensions with `limit: 1`
providers:

```yaml
spec:
  inputStrategy:
    name: Permute
  inputs:
    - region: us-east
    - region: eu-west
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: image-tag          # limit: 1
  # 2 × 1 = 2 permutations: one HelmRelease per region, both pinned to the current image.
```

**Field access.** Each source's input set is placed under a key derived from the
*normalized name of the object* providing it — **NOT** under its source fields
directly. Normalization: uppercase → lowercase; spaces/punctuation (including `-`) →
underscores; non-alphanumeric removed.

| Object providing inputs | Template key |
|---|---|
| `ResourceSetInputProvider` named `image-tag` | `inputs.image_tag` |
| `ResourceSetInputProvider` named `chart-version` | `inputs.chart_version` |
| Inline `.spec.inputs` on a `ResourceSet` named `my-apps` | `inputs.my_apps` |

Two always-flat accessors exist alongside the namespaced keys:

- `<< inputs.id >>` — auto-generated unique ID per permutation.
- `<< inputs.provider.{apiVersion,kind,name,namespace} >>` — metadata about the source.

**Inline inputs under Permute — common gotcha.** When `.spec.inputs` is set and
`Permute` is on, those inline inputs are keyed under the **ResourceSet's own
normalized name**. So a ResourceSet named `my-apps` with inline input `{region:
us-east}` needs `<< inputs.my_apps.region >>`, not `<< inputs.region >>`. This
differs from `Flatten` (the default), where inline inputs are accessed flat.

**Never omit `limit: 1`.** Exporting multiple tags from a single provider and letting
`Permute` cross them produces N redundant `HelmRelease`s for the same app — not what
you want. The operator stalls the `ResourceSet` at 10,000 permutations as a guard.

## Dependencies

ResourceSets support rich dependency definitions:

```yaml
spec:
  dependsOn:
    # Wait for another ResourceSet to be Ready
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSet
      name: infra
      namespace: flux-system
      ready: true

    # Wait for a CRD to exist (no readiness check needed)
    - apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      name: helmreleases.helm.toolkit.fluxcd.io

    # Wait for a Kustomization to be Ready at creation time (no updates needed after that)
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      name: infra-configs
      namespace: monitoring
      ready: true
      readyExpr: "status.observedGeneration >= 0"
```

Always set `namespace` on `dependsOn` entries so cross-namespace dependencies (e.g. an app
ResourceSet in `apps-test` waiting on `flux-system/infra-controllers`) resolve unambiguously.

**CEL expressions** in `readyExpr` evaluate against the dependency resource's status.
Common patterns:
- `status.conditions.filter(e, e.type == 'Ready').all(e, e.status == 'True')` — standard Ready check
- `status.observedGeneration >= 0` — resource has been reconciled at least once

## Step-Based Reconciliation

`spec.steps` replaces the flat `resources` list with an **ordered sequence of named steps**
(max 20). Each step's resources are applied and health-checked to completion before the next
step starts, giving in-ResourceSet ordering without splitting the work across multiple
`dependsOn`-linked objects. `steps` is mutually exclusive with `resources` and
`resourcesTemplate`, but each step carries its own `resources` or `resourcesTemplate` (same
`<< >>` templating, rendered once per input). A resource may be defined in only one step —
duplicates across steps fail the build. Step names must be unique DNS labels (max 63 chars).

Use `steps` when one tenant/app's resources have internal ordering requirements (Namespace +
RBAC → sources → appliers, or Jobs around a deploy); use `dependsOn` across ResourceSets when
whole ResourceSets must be ordered relative to each other.

### Jobs Before and After a Deployment

The motivating use case is running Kubernetes Jobs in sequence with a rollout: a database
migration must complete before the new version rolls out, and a cache warmup or smoke test
should run only after the rollout has finished. Upstream Flux expresses this with **three
Kustomizations chained with `dependsOn`** (`app-pre-deploy` → `app-deploy` → `app-post-deploy`),
three repository directories, and `force: true`/`wait: true` on each. With steps it is a
single object with one status, inventory and history, and no `dependsOn` requeue latency
between stages. Pairing it with an `OCIArtifactTag` provider makes the whole sequence run for
every new image version without a Git commit:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: podinfo-image
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  type: OCIArtifactTag
  url: oci://ghcr.io/stefanprodan/podinfo
  filter:
    semver: "*"
    limit: 1
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: podinfo
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: podinfo-image
  wait: true                      # required so the final step is health-checked too
  steps:
    - name: pre-deploy
      timeout: 5m
      resources:
        - apiVersion: batch/v1
          kind: Job
          metadata:
            name: db-migration
            namespace: apps
            annotations:
              fluxcd.controlplane.io/force: enabled              # recreate when the spec changes
              fluxcd.controlplane.io/recreateOnFailure: enabled  # retry a permanently failed Job
          spec:
            template:
              spec:
                restartPolicy: Never
                containers:
                  - name: migration
                    image: ghcr.io/stefanprodan/podinfo:<< inputs.tag >>
                    command: ["sh", "-c", "echo running db migration"]
    - name: deploy
      timeout: 10m                # longer than the Kustomization's own timeout
      resources:
        - apiVersion: kustomize.toolkit.fluxcd.io/v1
          kind: Kustomization
          metadata:
            name: podinfo
            namespace: apps
          spec:
            targetNamespace: apps
            sourceRef:
              kind: GitRepository
              name: apps
            path: deploy/podinfo
            interval: 60m
            prune: true
            wait: true
            timeout: 9m
            images:
              - name: ghcr.io/stefanprodan/podinfo
                newTag: << inputs.tag | quote >>   # spec change → generation bump → real rollout wait
    - name: post-deploy
      timeout: 5m
      resources:
        - apiVersion: batch/v1
          kind: Job
          metadata:
            name: cache-warmup
            namespace: apps
            annotations:
              fluxcd.controlplane.io/force: enabled
              fluxcd.controlplane.io/recreateOnFailure: enabled
          spec:
            template:
              spec:
                restartPolicy: Never
                containers:
                  - name: cache
                    image: ghcr.io/stefanprodan/podinfo:<< inputs.tag >>
                    command: ["sh", "-c", "echo refreshing cache"]
```

On reconciliation the operator applies `pre-deploy` and waits up to 5m for the Job to complete,
then applies `deploy` and waits up to 10m for the Kustomization to become Ready, then applies
`post-deploy`. When the provider exports a new tag, the `force` annotation recreates both Jobs
so they run for the new version, and because the tag lands in the Kustomization **spec** via
`images`, its generation changes and the step waits for the actual pod rollout.

**Mapping from the upstream Kustomization pattern:**

| Three-Kustomization chain | ResourceSet steps |
|---|---|
| `dependsOn` between Kustomizations | step order |
| `wait: true` on each Kustomization | implied wait between steps; the final step is gated by `spec.wait` |
| `force: true` on the Job Kustomizations | `fluxcd.controlplane.io/force: enabled` on each Job |
| `timeout` per Kustomization | per-step `timeout` |
| `prune: true` | inventory-based garbage collection of the ResourceSet |

### Step Semantics

- **Implied wait** — after applying a step the operator health-checks its resources and waits
  for them to become ready before starting the next step, regardless of `spec.wait`. The
  **final** step is health-checked only when `spec.wait: true`, so set it or the ResourceSet
  reports Ready before the last stage has actually finished.
- **Per-step timeout** — `timeout` falls back to the `fluxcd.controlplane.io/reconcileTimeout`
  annotation (default `5m`). Give a step wrapping a Flux applier a timeout longer than that
  applier's own `spec.timeout`.
- **Fail-fast** — if a step fails to apply or its health check fails, later steps are not
  applied and `Ready=False` names the step (`step "pre-deploy" health check failed`). The next
  reconciliation retries the full sequence from step 1; re-applying unchanged resources,
  including completed Jobs, is a no-op, so retries are idempotent.
- **Garbage collection** runs only after all steps have succeeded. On a mid-sequence failure
  stale resources stay on the cluster until the first fully successful reconciliation.
- Progress is reported in the `Reconciling` condition (`Applying step 2/3 "deploy"`) and as
  per-step `ApplySucceeded` events. Termination order on deletion is not guaranteed.

### Job Lifecycle

Job specs are immutable, so re-running is governed by annotations on the Job:

- `fluxcd.controlplane.io/force: enabled` — when the rendered spec changes (a new image tag
  from an input bump), the operator deletes and recreates the Job so it runs again. Without it
  the apply fails on the immutable field. With an unchanged spec, re-applying a completed Job
  is a no-op.
- `fluxcd.controlplane.io/recreateOnFailure: enabled` — a Job whose `backoffLimit` is
  exhausted (`Failed=True`) otherwise stays failed and keeps the ResourceSet `Ready=False`
  until its spec or an input changes. With this annotation the operator deletes the failed Job
  before applying its step and recreates it. **Only for idempotent Jobs**: a non-idempotent
  migration would be re-run on every reconciliation until it succeeds, repeating partial
  changes. Works on any Job managed by a ResourceSet, with or without steps, even when the Job
  has `prune: disabled`.
- **Never set `ttlSecondsAfterFinished`** on a Job managed by a ResourceSet. When the TTL
  controller deletes the completed Job, the operator sees a missing resource as drift on the
  next reconciliation and re-applies it — the migration runs again unexpectedly.
- To keep a record of past runs while re-running per version, template the name from the
  revision: `name: db-migration-<< inputs.tag >>`. Each bump creates a new Job and the previous
  one is garbage collected after the sequence succeeds.

### Rollouts on Data-Only Input Changes

If only config inputs change (no image bump), the applier's spec is unchanged and nothing rolls
out. Hash the inputs into the pod template through a Kustomization patch:

```yaml
- apiVersion: kustomize.toolkit.fluxcd.io/v1
  kind: Kustomization
  spec:
    patches:
      - target:
          kind: Deployment
        patch: |
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: all
          spec:
            template:
              metadata:
                annotations:
                  config-checksum: << inputs.config | toYaml | sha256sum | quote >>
```

When the workload is defined directly in a step (no applier in between), use the
[`checksumFrom` annotation](#checksumfrom-annotation) on its pod template instead; the step
health check then waits on the workload rollout itself.

### Data Across Steps

`copyFrom` reads from the cluster **before the first step runs** — it is not step-aware, so a
resource in a later step cannot copy data from a Secret created by an earlier step in the same
reconciliation. The only step-safe cross-reference is `checksumFrom` pointing at ConfigMaps or
Secrets generated by the same ResourceSet: those are resolved in-memory from the pending apply,
whichever step defines them.

## Advanced Features

### Conditional Reconciliation

Control which resources are reconciled per input using the `reconcile` annotation
with conditional template expressions:

```yaml
spec:
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: << inputs.tenant >>
        annotations:
          fluxcd.controlplane.io/reconcile: << if eq inputs.tenant "team1" >>enabled<< else >>disabled<< end >>
```

When the annotation value is `disabled`, the resource is excluded from reconciliation
for that input entry.

### copyFrom Annotation

Copy ConfigMaps and Secrets from another namespace:

```yaml
metadata:
  annotations:
    fluxcd.controlplane.io/copyFrom: "source-namespace/resource-name"
```

The ResourceSet controller copies the data from the source resource and keeps it in sync.
For Secrets, you must also set the `type` field.

### checksumFrom Annotation

To roll a workload whenever a ConfigMap or Secret changes, put `fluxcd.controlplane.io/checksumFrom`
on the **pod template** annotations with a comma-separated list of `Kind/namespace/name`
references (`Kind` is `ConfigMap` or `Secret`). The operator hashes the combined data (SHA256)
into a sibling `fluxcd.controlplane.io/checksum` annotation; the pod template hash changes and
Kubernetes performs a rolling update:

```yaml
spec:
  template:
    metadata:
      annotations:
        fluxcd.controlplane.io/checksumFrom: |
          ConfigMap/<< inputs.provider.namespace >>/<< inputs.name >>-config,
          Secret/<< inputs.provider.namespace >>/<< inputs.name >>-secret
```

References to objects generated by the same ResourceSet are resolved from the pending apply.
For objects outside the ResourceSet the checksum refreshes on the next reconciliation — label
those ConfigMaps/Secrets with `reconcile.fluxcd.io/watch: Enabled` for an immediate refresh,
and on multi-tenant clusters make sure the ResourceSet's service account can read them.

### Reconciliation Annotations

Set on the ResourceSet object; `force`, `recreateOnFailure` and `prune` also work on individual
generated resources:

| Annotation | Default | Description |
|---|---|---|
| `fluxcd.controlplane.io/reconcile` | `enabled` | `disabled` pauses reconciliation |
| `fluxcd.controlplane.io/reconcileEvery` | `1h` | Drift detection/correction interval |
| `fluxcd.controlplane.io/reconcileTimeout` | `5m` | Timeout including health checks; the default per-step timeout |
| `fluxcd.controlplane.io/force` | — | `enabled`: replace resources with immutable field changes (Jobs, some Service/PVC fields) instead of failing the apply |
| `fluxcd.controlplane.io/recreateOnFailure` | — | `enabled` on a Job: delete it when `Failed=True` and recreate it on the next reconciliation |
| `fluxcd.controlplane.io/prune` | `enabled` | `disabled` excludes a resource from garbage collection (set via `commonMetadata` to protect everything) |

### resourcesTemplate

Alternative to inline `resources`: a Go template string rendered as multi-document YAML
(documents separated by `---`). Like `resources`, it is rendered **once per input**, with the
current input exposed as `inputs`. Its value over the structured `resources` list is the
`<<- range >>` and `<<- if >>` constructs — in particular, ranging over an **array field
within an input** to emit a variable number of objects per input (which the fixed `resources`
list cannot express):

```yaml
spec:
  inputs:
    - tenant: team1
      components: [frontend, backend]
    - tenant: team2
      components: [api]
  resourcesTemplate: |
    <<- range $component := inputs.components >>
    ---
    apiVersion: v1
    kind: Namespace
    metadata:
      name: << inputs.tenant >>-<< $component >>
      labels:
        tenant: << inputs.tenant >>
    <<- end >>
```

Rendered per input: `team1` yields namespaces `team1-frontend` and `team1-backend`, `team2`
yields `team2-api`. Reference input fields as `<< inputs.field >>` and the loop variable as
`<< $field >>` (no leading dot).

When both `resources` and `resourcesTemplate` are set, the generated objects are merged,
with the `resources` entries taking precedence on duplicates.

### Deduplication

When multiple inputs produce resources with the same name/namespace/kind, the last input wins.
Resources are deduplicated by their GVK + namespace + name.

## Built-in Input Fields

Every input entry automatically includes:

| Field | Description |
|-------|-------------|
| `inputs.id` | Unique identifier for the input set amongst all sets generated for the ResourceSet. Value depends on provider type: Adler-32 checksum of the branch/tag name for Git branches, Git tags and OCI tags (checksum of the provider UID for Static, of `<namespace>/<name>` for ExternalArtifact), the PR/MR number for pull/merge requests |
| `inputs.provider.apiVersion` | API version of the object providing the inputs |
| `inputs.provider.kind` | Kind of the object providing the inputs (`ResourceSet` for inline, `ResourceSetInputProvider` for external) |
| `inputs.provider.name` | Name of the providing object |
| `inputs.provider.namespace` | Namespace of the providing object |

## ResourceSetInputProvider

Fetches input values from external services for use in ResourceSets.

### Canonical YAML

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: github-prs
  namespace: flux-system
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "5m"
spec:
  type: GitHubPullRequest
  url: https://github.com/my-org/my-app
  secretRef:
    name: github-token
  filter:
    limit: 10
    labels:
      - deploy-preview
  defaultValues:
    cluster: preview
```

### Provider Types and Exported Fields

| Type | Exported Fields | Description |
|------|----------------|-------------|
| `GitHubPullRequest` | `id`, `sha`, `branch`, `author`, `title` | Open pull requests with matching labels |
| `GitHubBranch` | `id`, `branch`, `sha` | Repository branches |
| `GitHubTag` | `id`, `tag`, `sha` | Repository tags |
| `GitLabMergeRequest` | `id`, `sha`, `branch`, `author`, `title` | Open merge requests |
| `GitLabBranch` | `id`, `branch`, `sha` | Repository branches |
| `GitLabTag` | `id`, `tag`, `sha` | Repository tags |
| `GitLabEnvironment` | `id`, `sha`, `branch`, `author`, `title`, `slug` | Deployed GitLab environments |
| `AzureDevOpsPullRequest` | `id`, `sha`, `branch`, `author`, `title` | Open pull requests |
| `AzureDevOpsBranch` | `id`, `branch`, `sha` | Repository branches |
| `AzureDevOpsTag` | `id`, `tag`, `sha` | Repository tags |
| `GiteaPullRequest` | `id`, `sha`, `branch`, `author`, `title` | Open pull requests |
| `GiteaBranch` | `id`, `branch`, `sha` | Repository branches |
| `GiteaTag` | `id`, `tag`, `sha` | Repository tags |
| `AWSCodeCommitPullRequest` | `id`, `sha`, `branch`, `author`, `title` | Open pull requests (workload identity) |
| `AWSCodeCommitBranch` | `id`, `branch`, `sha` | Repository branches (workload identity) |
| `AWSCodeCommitTag` | `id`, `tag`, `sha` | Repository tags (workload identity) |
| `OCIArtifactTag` | `id`, `tag`, `digest` | OCI artifact tags (generic registries) |
| `ACRArtifactTag` | `id`, `tag`, `digest` | Azure Container Registry tags (workload identity) |
| `ECRArtifactTag` | `id`, `tag`, `digest` | AWS ECR tags (workload identity) |
| `GARArtifactTag` | `id`, `tag`, `digest` | Google Artifact Registry tags (workload identity) |
| `ExternalArtifact` | `id`, `name`, `namespace`, `revision` + **every label** on the artifact | In-cluster `ExternalArtifact` objects (e.g. generated by `ArtifactGenerator`) matched by `spec.selectors`; no `url` |
| `ExternalService` | (from HTTP response) | Custom HTTP service endpoint |
| `Static` | (from `defaultValues`) | Single input from inline values |

The `ExternalArtifact` type turns the cluster itself into the input source: an `ArtifactGenerator`
with `pathPattern` labels each generated artifact with its path captures (`app: auth`, `env: dev`),
the provider exports those labels as template variables, and the ResourceSet templates a
`Kustomization` per artifact. This is the building block for directory-driven monorepo delivery —
load `references/monorepo-delivery.md` for the end-to-end pipeline.

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: platform-apps
  namespace: apps
spec:
  type: ExternalArtifact
  selectors:
    - matchLabels:          # artifacts in the provider's own namespace
        team: platform
        env: dev
```

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

### Key Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Provider type (required, see table above) |
| `url` | string | Repository or registry URL (required for external types; must NOT be set for `Static` or `ExternalArtifact`) |
| `selectors` | array | `ExternalArtifact` only (required for it, forbidden otherwise). Each entry: `matchLabels`/`matchExpressions`, or `name` (mutually exclusive with the label selectors), plus optional `namespace` — empty = the provider's namespace, `"*"` = all namespaces (needs cluster-wide `list` on ExternalArtifacts), else that namespace. Entries are OR-ed |
| `secretRef.name` | string | Secret with credentials (`username`/`password` for Git, dockerconfigjson for OCI) |
| `serviceAccountName` | string | SA for workload identity (AzureDevOps*, AWSCodeCommit*, *ArtifactTag types) and for impersonation when listing `ExternalArtifact` objects on multi-tenant clusters |
| `filter.limit` | int | Max number of inputs to return (default: 100) — raise it for monorepos with more matching directories |
| `filter.labels` | array | Label filter for change requests |
| `filter.includeBranch` | string | Regex to include branches |
| `filter.excludeBranch` | string | Regex to exclude branches |
| `filter.includeTag` | string | Regex to include tags |
| `filter.excludeTag` | string | Regex to exclude tags |
| `filter.semver` | string | Semver range to filter and sort tags |
| `skip.labels` | array | Labels to skip input updates (prefix `!` to skip if absent) |
| `defaultValues` | map | Default key-value pairs merged with exported values — e.g. `app: frontend` so an otherwise identical ResourceSet body can template on `<< inputs.app >>` |
| `schedule` | array | Cron schedules with `cron`, `timeZone`, `window` fields |
| `insecure` | bool | Allow HTTP (ExternalService, OCIArtifactTag only) |
| `certSecretRef.name` | string | Secret with TLS CA cert (`ca.crt`) |

When `filter.semver` is fed through Flux post-build substitution (`semver: "${app_semver}"`),
the value must not start with a YAML indicator (`>`, `|`, `*`, `&`, `%`, `@`, `[`, `{`):
kustomize drops the quotes and the substituted manifest becomes invalid YAML. Use `x`,
`6.14.x`, `~6.14.0`, `^6.14.0` (same major, not `>=`) or `x || >=0.0.0-0` for prereleases.

Reconciliation is configured via annotations, not spec fields:
- `fluxcd.controlplane.io/reconcileEvery: "5m"` — poll interval (default: `10m`)
- `fluxcd.controlplane.io/reconcile: "enabled"` — enable/disable reconciliation
- `fluxcd.controlplane.io/reconcileTimeout: "2m"` — timeout for external calls

### Referencing in ResourceSet

```yaml
spec:
  inputsFrom:
    - name: github-prs                    # by name
    - selector:
        matchLabels:
          app: preview                     # by label selector
```

## Use Cases

### Multi-Component Orchestration (Gitless Pattern)

The Gitless reference architecture uses a chain of ResourceSets:
1. **policies** — Creates ValidatingAdmissionPolicies (no inputs needed)
2. **infra** — Creates per-component namespaces + OCIRepository + Kustomization for infrastructure (cert-manager, monitoring)
3. **apps** — Creates per-tenant namespaces + OCIRepository + Kustomization for applications (frontend, backend)

Dependencies: policies → infra → apps. Each ResourceSet waits for the previous one to be Ready.

### Preview Environments from Pull Requests

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-previews
  namespace: preview               # the dedicated preview namespace, not flux-system
spec:
  type: GitHubPullRequest
  url: https://github.com/org/app
  secretRef:
    name: github-app-auth
  filter:
    labels: [deploy-preview]
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app-previews
  namespace: preview
spec:
  serviceAccountName: flux     # scoped: admin within the preview namespace only
  inputsFrom:
    - name: app-previews
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: "app-<< inputs.id >>"
        namespace: preview
      spec:
        interval: 2m
        url: https://github.com/org/app.git
        ref:
          commit: << inputs.sha >>   # pin to the PR head commit
        provider: github
        secretRef:
          name: github-app-auth
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: "app-<< inputs.id >>"
        namespace: preview
      spec:
        serviceAccountName: flux
        targetNamespace: preview
        nameSuffix: "-pr<< inputs.id >>"   # isolate this PR's workloads in the shared namespace
        interval: 10m
        prune: true
        wait: true
        timeout: 5m
        retryInterval: 2m
        sourceRef:
          kind: GitRepository
          name: "app-<< inputs.id >>"
        path: ./deploy/preview
        images:
          - name: ghcr.io/org/app
            newTag: preview-<< inputs.sha >>   # the image CI built for this PR commit
```

Provision the `preview` namespace once with the `flux` ServiceAccount and a RoleBinding
to the built-in `admin` — scoping `flux` to `preview` so PR content can't escalate beyond it.

### Gitless Image Automation with ResourceSets

`ResourceSet` + `ResourceSetInputProvider` of `type: OCIArtifactTag` implements image
update automation without committing tag bumps to Git — the provider scans the
registry, the `ResourceSet` re-renders, and the downstream `HelmRelease` or
`Kustomization` upgrades directly. For the full pattern (provider filters, Permute
strategy, `tag@digest` pinning, post-renderers for images not in Helm values) load
`references/gitless-image-automation.md`.

### Directory-Driven Monorepo Delivery

`ArtifactGenerator` (`pathPattern: "@monorepo/apps/{app}/envs/{env}"`) generates one labeled
`ExternalArtifact` per matching directory, a `ResourceSetInputProvider` of `type: ExternalArtifact`
exports one input per artifact (labels included), and the ResourceSet templates a `Kustomization`
with `sourceRef.kind: ExternalArtifact` per input. Adding a directory deploys an app, removing it
prunes the deployment — no per-app Flux config to maintain. In the production layout, apps combine
both input types: the manifests come from the artifact and the image tag from an `OCIArtifactTag`
provider, pinned via the Kustomization's `spec.images`. Load `references/monorepo-delivery.md`
for the full pipeline, layered infra reconcilers, app environments and the rules that must hold.

### Staged Deployments with Jobs

A migration Job → app Kustomization → smoke-test Job sequence is a single ResourceSet with
`spec.steps` — see [Jobs Before and After a Deployment](#jobs-before-and-after-a-deployment).

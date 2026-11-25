# Flux CRD Reference

Detailed reference for Flux custom resource definitions, organized by controller.
Use this when you need to understand CRD fields, status conditions, or common failure modes.

**Contents:** [Resource Relationships](#resource-relationships) | [Flux Operator](#flux-operator) | [Source Controller](#source-controller) | [Kustomize Controller](#kustomize-controller) | [Helm Controller](#helm-controller) | [Notification Controller](#notification-controller) | [Image Automation Controllers](#image-automation-controllers)

## Resource Relationships

```
┌─────────────────────────────────────────────────────────────┐
│ Sources (source-controller)                                 │
│ GitRepository / OCIRepository / HelmChart / Bucket /        │
│ ExternalArtifact (ArtifactGenerator)                        │
└────────────────────────┬────────────────────────────────────┘
                         │ produces Artifact
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Appliers                                                    │
│ Kustomization (kustomize-controller)                        │
│ HelmRelease (helm-controller)                               │
└────────────────────────┬────────────────────────────────────┘
                         │ creates/manages
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Managed Resources (inventory)                               │
│ Deployments / DaemonSets / StatefulSets / Jobs / ...        │
└────────────────────────┬────────────────────────────────────┘
                         │ schedules
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Pods                                                        │
└─────────────────────────────────────────────────────────────┘
```

## Flux Operator

### FluxInstance (`fluxcd.controlplane.io/v1`)

**Purpose**: Manages the Flux controllers installation and configuration on a cluster.

**Key spec fields**:
- `.spec.distribution` — Flux distribution to install (registry, version, artifact)
- `.spec.components` — List of Flux controllers to install
- `.spec.cluster` — Cluster configuration (domain, multitenant, networkPolicy)
- `.spec.sync` — Bootstrap sync configuration (kind, url, ref, path, pullSecret)
- `.spec.kustomize.patches` — Kustomize patches to apply to controller manifests

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`

**Common failures**:
- Distribution artifact not found (wrong registry URL or version)
- Controller pods crashlooping (resource limits, incompatible versions)
- Sync source not accessible (auth errors, network issues)

### FluxReport (`fluxcd.controlplane.io/v1`)

**Purpose**: Reflects the state of a Flux installation, providing cluster-wide summary.

**Key fields**:
- `.spec.distribution` — Installed Flux version and entitlement status
- `.spec.components` — Status of the Flux controller deployments
- `.spec.reconcilers` — Flux resource statistics (failing, running, suspended) grouped by kind
- `.status.conditions` — Readiness of the report object itself

**Common failures**:
- Report not generated (FluxInstance not installed or not ready)

### ResourceSet (`fluxcd.controlplane.io/v1`)

**Purpose**: Manages groups of Kubernetes resources based on input matrices.

**Key spec fields**:
- `.spec.inputsFrom` — References to ResourceSetInputProvider objects that supply dynamic inputs
- `.spec.resources` — Templated Kubernetes resources using `<< inputs.field >>` delimiters (not Go's `{{ }}`)
- `.spec.serviceAccountName` — Service account for applying resources

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`

**Common failures**:
- Template rendering errors (invalid template syntax, missing input values)
- RBAC errors (service account lacks permissions)
- Input provider not ready

### ResourceSetInputProvider (`fluxcd.controlplane.io/v1`)

**Purpose**: Provides input values for ResourceSets from external services or static definitions.

**Provider types**: `Static` (inline inputs), Git servers (`GitHub*`, `GitLab*`, `AzureDevOps*`, `AWSCodeCommit*`, `Gitea*` variants for branches, tags, and pull/merge requests, plus `GitLabEnvironment`), registry scanners (`OCIArtifactTag`, `ACRArtifactTag`, `ECRArtifactTag`, `GARArtifactTag`), and `ExternalService` (custom webhook-driven inputs).

**Key spec fields**:
- `.spec.type` — Provider type (see list above)
- `.spec.url` — Repository or registry URL (required for external providers)
- `.spec.defaultValues` — Default values applied to all exported inputs
- `.spec.secretRef` — Secret with authentication credentials
- `.spec.serviceAccountName` — Service account for workload identity (Azure/AWS/GCP)
- `.spec.certSecretRef` — Secret with TLS certificates
- `.spec.filter` — Input filtering criteria
- `.spec.skip` — Skip criteria by labels
- `.spec.schedule` — List of cron schedules

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`

**Common failures**:
- Authentication error (invalid or expired token in secret)
- Stalled with `InvalidDefaultValues` (malformed default values in spec)
- Filter matches no results (wrong filter expression or no matching resources)
- URL unreachable (wrong endpoint, network issues, DNS failure)

## Source Controller

### GitRepository (`source.toolkit.fluxcd.io/v1`)

**Purpose**: Fetches and produces an Artifact from a Git repository revision.

**Key spec fields**:
- `.spec.url` — Git repository URL (HTTPS or SSH)
- `.spec.ref` — Git reference (branch, tag, semver, commit)
- `.spec.interval` — Polling interval
- `.spec.secretRef` — Auth credentials secret
- `.spec.sparseCheckout` — Directories to include

**Key status conditions**: `Ready`, `Reconciling`, `FetchFailed`, `SourceVerified`

**Common failures**:
- Authentication error (invalid credentials, expired token, wrong SSH key)
- Repository not found (wrong URL, private repo without auth)
- Reference not found (branch/tag doesn't exist)
- Timeout (large repository, slow network)

### OCIRepository (`source.toolkit.fluxcd.io/v1`)

**Purpose**: Fetches and produces an Artifact from an OCI container registry.

**Key spec fields**:
- `.spec.url` — OCI URL with `oci://` prefix
- `.spec.ref` — Artifact reference (tag, semver, digest)
- `.spec.interval` — Polling interval
- `.spec.provider` — Auth provider (generic, aws, gcp, azure)
- `.spec.secretRef` — Image pull secret reference
- `.spec.verify.provider` — Signature verification (cosign)
- `.spec.layerSelector` — Media type and operation for layer extraction

**Key status conditions**: `Ready`, `Reconciling`, `FetchFailed`, `SourceVerified`

**Common failures**:
- Authentication error (wrong pull secret, expired token, misconfigured workload identity)
- Image not found (wrong URL or tag)
- Signature verification failed (artifact not signed or wrong cosign key)

### HelmRepository (`source.toolkit.fluxcd.io/v1`)

**Purpose**: Points to a Helm chart repository (HTTP/S or OCI).

**Key spec fields**:
- `.spec.url` — Repository URL
- `.spec.interval` — Index fetch interval
- `.spec.secretRef` — Auth credentials
- `.spec.provider` — Auth provider for OCI registries

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`, `FetchFailed`

Note: OCI HelmRepositories have no status conditions.

**Common failures**:
- Repository URL unreachable (DNS, firewall, proxy)
- Authentication error (wrong credentials)
- Invalid repository index (corrupted or not a Helm repo)

### HelmChart (`source.toolkit.fluxcd.io/v1`)

**Purpose**: References a chart from a HelmRepository or GitRepository.

**Key spec fields**:
- `.spec.chart` — Chart name
- `.spec.version` — Version constraint (semver)
- `.spec.sourceRef` — Reference to HelmRepository, GitRepository, or Bucket
- `.spec.valuesFiles` — Additional values files from the source

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`, `FetchFailed`, `SourceVerified`

**Common failures**:
- Chart not found in repository (wrong name or version)
- Source not ready (HelmRepository fetch failed)
- Version constraint matches no available version

## Kustomize Controller

### Kustomization (`kustomize.toolkit.fluxcd.io/v1`)

**Purpose**: Builds and applies Kubernetes manifests from a source Artifact.

**Key spec fields**:
- `.spec.sourceRef` — Reference to a source (GitRepository, OCIRepository, Bucket)
- `.spec.path` — Path within the source artifact
- `.spec.interval` — Reconciliation interval
- `.spec.prune` — Delete resources removed from the source
- `.spec.dependsOn` — Dependencies on other Kustomizations
- `.spec.postBuild.substituteFrom` — Variable substitution from ConfigMaps/Secrets
- `.spec.serviceAccountName` — Service account for applying resources
- `.spec.targetNamespace` — Override namespace for all resources
- `.spec.healthChecks` — Resources to monitor for readiness
- `.spec.timeout` — Timeout for health checks

**Key status conditions**: `Ready`, `Reconciling`, `Healthy`

**Common failures**:
- Build error (invalid kustomization.yaml, missing resources)
- Apply conflict (field manager conflict, immutable field change)
- Health check timeout (managed resources not becoming ready)
- RBAC error (service account lacks permissions to create resources)
- Source not ready (referenced source has fetch errors)
- Variable substitution error (missing ConfigMap/Secret referenced in substituteFrom)

## Helm Controller

### HelmRelease (`helm.toolkit.fluxcd.io/v2`)

**Purpose**: Manages Helm chart releases from source Artifacts.

**Key spec fields**:
- `.spec.chartRef` / `.spec.chart.spec.sourceRef` — Chart source reference
- `.spec.chart.kind: OCIRepository` (Flux 2.10+) — `.spec.chart.spec` is an inline OCIRepository spec (`url`, `ref`); the controller creates and owns an OCIRepository named `<hr-namespace>-<hr-name>`, reported in `.status.helmChart` as `OCIRepository/<ns>/<name>` (event `OCIRepositoryCreated`). Debug that OCIRepository like any other source
- `.spec.interval` — Reconciliation interval
- `.spec.values` — Inline Helm values
- `.spec.valuesFrom` — Values from ConfigMaps/Secrets
- `.spec.install` — Install configuration (strategy, remediation)
- `.spec.upgrade` — Upgrade configuration (strategy, remediation)
- `.spec.driftDetection` — Detect and correct configuration drift
- `.spec.dependsOn` — Dependencies on other HelmReleases
- `.spec.serviceAccountName` — Service account for Helm operations

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`, `Released`, `TestSuccess`, `Remediated`

**Common failures**:
- Install/upgrade failed (chart rendering error, invalid values)
- Values merge error (conflicting valuesFrom sources)
- Drift detected (resources modified outside Flux)
- Remediation exhausted (max retries reached, manual intervention needed)
- Source not ready (HelmChart or OCIRepository not available)
- Health check timeout (deployed resources not becoming ready)
- Image pull backoff (container image not found or auth error)

## Notification Controller

### Provider (`notification.toolkit.fluxcd.io/v1`)

**Purpose**: Represents a notification service (Slack, MS Teams, GitHub, etc.).

**Key spec fields**:
- `.spec.type` — Provider type (slack, msteams, github, generic, etc.)
- `.spec.address` — Webhook URL or API endpoint
- `.spec.secretRef` — Secret with credentials (`address`, `token`, `username`, `password`, `headers`)
- `.spec.proxySecretRef` — Secret with proxy `address`; replaces `.spec.proxy` (removed in v1)
- `.spec.certSecretRef` — TLS data (`ca.crt`, `tls.crt`, `tls.key`)

Note: Providers have no status conditions. `v1beta3` is deprecated in Flux 2.10 (warning
`v1beta3 Provider is deprecated, upgrade to v1`); `v1` drops `.spec.proxy` and `.spec.interval`.

### Alert (`notification.toolkit.fluxcd.io/v1`)

**Purpose**: Configures events to be forwarded to notification providers.

**Key spec fields**:
- `.spec.providerRef` — Reference to a Provider
- `.spec.eventSources` — List of resources to watch for events
- `.spec.eventSeverity` — Minimum severity to forward (info, error)

Note: Alerts have no status conditions.
`v1beta3` is deprecated in Flux 2.10; `v1` drops `.spec.summary` (use `.spec.eventMetadata.summary`).

### Receiver (`notification.toolkit.fluxcd.io/v1`)

**Purpose**: Defines webhooks for triggering reconciliations from external sources.

**Key spec fields**:
- `.spec.type` — Receiver type (github, gitlab, bitbucket, generic, etc.)
- `.spec.resources` — Resources to trigger reconciliation on
- `.spec.secretRef` — Webhook secret for validation

**Key status conditions**: `Ready`

## Image Automation Controllers

### ImageRepository (`image.toolkit.fluxcd.io/v1`)

**Purpose**: Scans container registries for new image tags.

**Key spec fields**:
- `.spec.image` — Container image to scan
- `.spec.interval` — Scan interval
- `.spec.provider` — Auth provider (aws, gcp, azure)

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`

### ImagePolicy (`image.toolkit.fluxcd.io/v1`)

**Purpose**: Selects the latest image tag based on a policy.

**Key spec fields**:
- `.spec.imageRepositoryRef` — Reference to ImageRepository
- `.spec.policy` — Selection policy (semver, alphabetical, numerical)

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`

### ImageUpdateAutomation (`image.toolkit.fluxcd.io/v1`)

**Purpose**: Updates a Git repository with new image tags selected by ImagePolicies.

**Key spec fields**:
- `.spec.sourceRef` — Reference to GitRepository
- `.spec.git.push` — Branch to push updates to
- `.spec.git.commit` — Commit message template
- `.spec.update.path` — Path to scan for image policy markers

**Key status conditions**: `Ready`, `Reconciling`, `Stalled`

# Best Practices

Prescriptive guidance for production Flux deployments with Flux Operator.

## Repository Structure

- **Separate concerns:** Use distinct directories (or repos) for infrastructure and applications.
  Infrastructure includes CRDs, operators, and cluster configs. Applications include workloads.
- **Base + overlay pattern:** Use Kustomize base directories with per-environment overlays
  (staging, production). Overlays patch values like replicas, resources, and domains.
- **Per-cluster directories:** Each cluster gets a directory under `clusters/` containing
  its FluxInstance, runtime-info ConfigMap, and root Kustomization.
- **Component isolation:** Each infrastructure or application component gets its own directory
  with a controllers and configs split (controllers install the operator, configs apply settings).

## Dependency Management

- **Use `dependsOn` for ordering:** Install CRDs and operators before their custom resources.
  Install infrastructure before applications.
- **Set `wait: true`:** On Kustomizations that others depend on. This ensures the dependency
  is fully rolled out before dependents proceed.
- **CRDs before CRs:** Deploy CRD-installing HelmReleases in a separate Kustomization
  that runs before the Kustomization applying custom resources.
- **ResourceSet dependency chains:** Use `ready: true` for cross-ResourceSet
  dependencies (policies → infra → apps). Optionally add a `readyExpr` CEL expression
  to replace the built-in readiness check with a custom one.

## Remediation and Reliability

- **Install/upgrade strategies:** Always use `RetryOnFailure` — it retries without
  uninstalling or rolling back, avoiding downtime and data loss on transient failures.
- **Set `retryInterval`:** Configure retry intervals on Kustomizations (`retryInterval: 5m`)
  and HelmReleases (`strategy.retryInterval: 5m`) for faster recovery from transient failures.
- **Drift detection:** Enable `driftDetection.mode: enabled` on HelmReleases for critical
  applications to prevent manual cluster changes from diverging. Ignore HPA-managed fields
  like `/spec/replicas`.
- **Timeouts:** Set explicit `timeout` on Kustomizations and HelmReleases to prevent
  indefinite hanging during apply operations.
- **CRD handling:** Use `install.crds: Create` and `upgrade.crds: CreateReplace` for
  HelmReleases that manage CRDs. Note that CRDs installed via a Helm chart are **not**
  deleted on uninstall — Helm has no native support for removing CRDs.
- **Reactivity:** Add `reconcile.fluxcd.io/watch: Enabled` label to ConfigMaps and Secrets
  that feed into `postBuild.substituteFrom` or `valuesFrom` — this triggers immediate
  reconciliation when values change instead of waiting for the next interval.

## Versioning

- **Semver ranges for charts:** Use `version: "18.x"` or `semver: ">=1.0.0 <2.0.0"` to
  auto-update within safe ranges. Avoid `*` in production.
- **Pinned refs for production:** Use specific tags or digests for OCI artifacts in production.
  Use `latest` or branch-based tags for staging.
- **Tag promotion:** Push `latest` from main branch (staging) and `latest-stable` from
  release tags (production). Each environment's FluxInstance references its appropriate tag.

## Namespace Isolation

- **Per-component namespaces:** Deploy each component into its own namespace. Infrastructure
  components get namespaces like `cert-manager`, `monitoring`. Applications get tenant namespaces.
- **Create namespaces in Kustomization or ResourceSet:** Always create the target namespace
  as part of the Kustomization or ResourceSet that deploys the component. Do NOT use
  `targetNamespace` or `createNamespace` in HelmRelease — these bypass proper namespace
  lifecycle management. The namespace should exist before the HelmRelease runs, created
  by the parent Kustomization or ResourceSet template.
- **Tenant labels:** Label tenant namespaces with `toolkit.fluxcd.io/role: "tenant"` for
  policy targeting and identification.
- **`targetNamespace`:** Set on Kustomizations to ensure all resources land in the correct namespace.
- **Service account impersonation:** Use `serviceAccountName` on Kustomizations and HelmReleases
  with per-namespace ServiceAccounts and RoleBindings for least-privilege access.

## Flux Operator Configuration

- **Cluster sizing:** Match `cluster.size` to workload count — `small` for dev (<50 resources),
  `medium` for standard production, `large` for clusters with hundreds of Flux resources.
- **Multi-tenancy:** Enable `cluster.multitenant: true` with `tenantDefaultServiceAccount`
  to enforce RBAC isolation between tenants.
- **GitHub App auth for sync:** Use a GitHub App installation token for FluxInstance sync
  credentials instead of personal access tokens for better security and auditability.
- **Self-managed operator:** Use a ResourceSet that depends on the HelmRelease CRD to manage
  the Flux Operator itself via Flux (operator manages itself pattern).
- **Cosign verification:** Add Cosign verification patches to the FluxInstance for the sync
  OCIRepository to verify artifact signatures.
- **Object identity (Flux 2.10+):** set `cluster.objectIdentityRBAC: true` when objects use
  object identity (`serviceAccountName` for cloud or `kubernetes` provider auth) — the 2.10 distribution
  no longer grants `serviceaccounts/token` create. Add `cluster.multitenantObjectIdentity: true`
  to force tenants onto object identity. Both need a Flux Operator release with 2.10 support.

## SPIFFE: Inter-Controller mTLS and Object Identity (RFC-0014, Flux 2.10+)

Opt-in, configured only through controller env vars and flags (no CRD fields or feature gates).
With Flux Operator, apply them as FluxInstance `spec.kustomize.patches` on the controller
Deployments; with bootstrap, as patches in `flux-system/kustomization.yaml`.

- **Prerequisite:** a SPIFFE runtime (e.g. SPIRE) deployed *before* Flux — `flux bootstrap` does
  not install it; the Flux Operator bootstrap Terraform module can. Each controller needs the
  Workload API socket: env `SPIFFE_ENDPOINT_SOCKET=unix:///spiffe-workload-api/<socket>` plus a
  volume mount, e.g. a CSI volume (`driver: csi.spiffe.io`, `readOnly: true`) at `/spiffe-workload-api`.
- **Inter-controller mTLS:** each traffic type is enabled by passing at least one of its repeatable
  flags, each taking the authorized peer's SPIFFE ID:

| Flag | Set on |
|------|--------|
| `--authorized-artifact-client-spiffe-id` | source-controller, source-watcher (artifact servers) |
| `--authorized-artifact-server-spiffe-id` | kustomize-controller, helm-controller, source-watcher (artifact clients) |
| `--authorized-notifier-spiffe-id` | notification-controller (event server) |
| `--authorized-event-server-spiffe-id` | all controllers except notification-controller |
| `--authorized-receiver-spiffe-id` | notification-controller (ingress/gateway peer of the webhook receiver) |
| `--authorized-kube-apiserver-spiffe-id` | all controllers (optional) |

  The source-controller, source-watcher, notification-controller and webhook-receiver Services
  expose port `443` (`https`); clients rewrite artifact URLs to HTTPS in memory, so
  `.status.artifact.url` is unchanged.
- **Object identity via `spiffe`:** set `--spiffe-broker-endpoint=dns:///<svc>.<ns>.svc.cluster.local:443`
  and one or more `--spiffe-broker-id=spiffe://<trust-domain>/<path>` (trailing `/` = prefix) on the
  controllers, then annotate the object's ServiceAccount with
  `credential.auth.fluxcd.io/provider: spiffe` and `credential.auth.fluxcd.io/type: jwt` (JWT-SVID).
  GCP also needs `gcp.auth.fluxcd.io/workload-identity-provider`; AWS can set
  `aws.auth.fluxcd.io/sts-endpoint`. Tenant isolation is up to the SPIFFE runtime issuing
  namespaced SPIFFE IDs. Object identity basics are in `references/kustomization.md`.
- **Deferred to Flux 2.11:** `credential.auth.fluxcd.io/type: x509` (AWS IAM Roles Anywhere, GCP
  WIF with X.509), the `spiffe.mtls.fluxcd.io/server-ids` annotation, and the `dockerhub`
  provider with `dockerhub.auth.fluxcd.io/oidc-connection-id`. Don't generate these for 2.10.

## Operational Excellence

- **Alerts on failures:** Create Provider + Alert for Slack/Teams with `eventSeverity: error`
  watching all Kustomizations and HelmReleases. Every cluster should have failure notifications.
- **Intervals:** Use short intervals for sources (`5m`) and longer intervals for appliers
  (`30m`). Receivers handle immediate triggers; intervals are the fallback.
- **Prune: true:** Always set `prune: true` on Kustomizations to enable garbage collection
  of removed resources. Without it, deleted manifests leave orphaned resources.
- **Monitoring:** Use FluxReport to monitor the health of the Flux installation. Set up
  Prometheus ServiceMonitors for Flux controller metrics. Enable the Flux Web UI
  (built into the Flux Operator Helm chart) for a dashboard showing FluxInstance status,
  reconciler stats, and cluster sync state. The Web UI supports SSO via OIDC providers
  (Dex, Keycloak, Microsoft Entra ID, OpenShift) and can be exposed via Ingress.
- **Image automation isolation:** Run image automation controllers on a dedicated cluster
  to isolate Git write access from production clusters.
- **Receivers for fast sync:** Set up Receiver webhooks for GitHub/GitLab push events to trigger
  immediate GitRepository reconciliation instead of waiting for the poll interval.

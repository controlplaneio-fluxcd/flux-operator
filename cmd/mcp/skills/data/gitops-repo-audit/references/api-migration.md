# Flux API Migration Guide

Reference for migrating from deprecated Flux API versions to current stable versions.
Source: https://github.com/fluxcd/flux2/discussions/5572

## Deprecated API Removals

**Removed in Flux v2.7:**
- `source.toolkit.fluxcd.io/v1beta1`
- `kustomize.toolkit.fluxcd.io/v1beta1`
- `helm.toolkit.fluxcd.io/v2beta1`
- `image.toolkit.fluxcd.io/v1beta1`
- `notification.toolkit.fluxcd.io/v1beta1`

**Removed in Flux v2.8:**
- `source.toolkit.fluxcd.io/v1beta2`
- `kustomize.toolkit.fluxcd.io/v1beta2`
- `helm.toolkit.fluxcd.io/v2beta2`
- `notification.toolkit.fluxcd.io/v1beta2`

**Deprecated in Flux v2.10** (still served, scheduled for removal in a future release):
- `notification.toolkit.fluxcd.io/v1beta3` (`Alert`, `Provider`) — migrate to `notification.toolkit.fluxcd.io/v1`

## Fields Removed in Notification v1

`Alert` and `Provider` in `notification.toolkit.fluxcd.io/v1` drop fields that were deprecated in
`v1beta3`. The API server drops them when objects are read or written at `v1` (the new storage version), and
`flux migrate -f .` (Flux CLI 2.10+) removes them from manifests, reporting each removal:

| Kind | Removed field | Replacement |
|------|---------------|-------------|
| Alert | `spec.summary` | `spec.eventMetadata.summary` (or the `event.toolkit.fluxcd.io/summary` annotation on the source object) |
| Provider | `spec.proxy` | `spec.proxySecretRef` (Secret with `address`, optional `username`/`password`) |
| Provider | `spec.interval` | none |

When auditing, report these fields as findings even on `v1beta3` objects: they are silently
dropped on upgrade, so an Alert summary or Provider proxy stops working without an error.

## Notification Behavior Changes in Flux v2.10

notification-controller reads every Provider at the `v1` storage version, so these apply to
`v1beta3` manifests too. Flag them in audits:

- A `proxy` key in the Provider `secretRef` Secret is rejected; move it to a Secret referenced by
  `spec.proxySecretRef` (`address`, optional `username`/`password`). Supported `secretRef` keys are
  `address`, `token`, `username`, `password`, `headers`.
- TLS data is read only from `spec.certSecretRef` (`ca.crt`, `tls.crt`, `tls.key`; the legacy
  `caFile`/`certFile`/`keyFile` keys still work but are deprecated). Commit status Providers fail if
  the `secretRef` Secret carries TLS data and `certSecretRef` is unset.
- `azureeventhub` fails when its Secret has a `token` key (static JWT auth was removed); use SAS
  (`address`) or object identity (`spec.serviceAccountName`).
- `bitbucket` commit status needs a Bitbucket Cloud API token (scopes `read:repository:bitbucket`,
  `write:repository:bitbucket`); the `<username>:<app-password>` format no longer works.

## CLI-Based Upgrade

### Step 1: Migrate Git resources

Update all manifests in the Git repository to use stable API versions:

```bash
git clone <your-git-repo>
cd <your-git-repo>
flux migrate -f .
git commit -am "Migrate to Flux stable APIs"
git push
```

Then reconcile: `flux reconcile ks flux-system --with-source`

### Step 2: Migrate cluster resources

Update the resources stored in Kubernetes etcd:

```bash
flux migrate
```

This is idempotent and safe to run multiple times. Supports `--context` and `--kubeconfig` flags.

### Step 3: Upgrade Flux components

```bash
flux bootstrap [same parameters as initial installation]
flux check  # Verify upgrade success
```

## Operator-Based Upgrade

### Step 1: Update Flux Operator to v0.43.0+

Update the operator via OperatorHub, Terraform/OpenTofu, or HelmRelease.

### Step 2: Migrate Git resources

Same as CLI Step 1 — update manifests in Git with `flux migrate -f .`

### Step 3: Update FluxInstance manifest

Set the distribution version to `2.10.x` (Flux Operator with 2.10 support required):

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.10.x"
    registry: "ghcr.io/fluxcd"
```

The operator automatically handles in-cluster API migrations.

## Key Notes

- `flux migrate` is idempotent — safe to run multiple times
- `flux migrate -f .` updates files in-place; use `--dry-run` to preview changes
- Clear `.kube/cache` locally if old API versions persist after migration
- Use `scripts/validate.sh` to verify manifests after migration

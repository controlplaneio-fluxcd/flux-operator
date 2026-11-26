# Troubleshooting Quick Reference

Common failure patterns and debugging procedures for Flux on Kubernetes clusters.

## Controller Failures

### Controller not running

**Symptoms**: Flux resources stuck in last known state, no new reconciliation events.

**Diagnosis**:
1. Check FluxInstance status — is it `Ready: True`?
2. List pods in `flux-system` namespace — are all controller pods running?
3. Check for pod scheduling issues (insufficient resources, node affinity, taints).

**Common causes**:
- Node resource pressure (CPU/memory limits hit)
- Image pull failure (private registry without pull secret)
- CRD not installed (controller starts but can't register watches)

### Controller crashlooping

**Symptoms**: Controller pod restarts frequently, `CrashLoopBackOff` status.

**Diagnosis**:
1. Get controller pod logs — look for panic/fatal messages.
2. Check pod events for OOMKilled or other termination reasons.
3. Check resource limits on the controller deployment.

**Common causes**:
- OOMKilled (increase memory limits in FluxInstance kustomize patches)
- Incompatible CRD version (controller binary doesn't match installed CRDs)
- Corrupt cache or storage (delete the pod to force fresh start)
- Unsupported Kubernetes version (Flux 2.10 requires Kubernetes 1.35 or later)

### Object identity credentials fail (Flux 2.10+)

**Symptoms**: Sources, Providers, image or applier objects with `.spec.serviceAccountName` fail auth
(cloud `.spec.provider`, `generic` with `credential.auth.fluxcd.io/*` ServiceAccount annotations, or
`kubeConfig.configMapRef`) — typically right after upgrading to 2.10; errors mention `serviceaccounts/token` or forbidden.

**Common causes**:
- **Missing token RBAC**: Flux 2.10 default RBAC no longer grants `create` on `serviceaccounts/token`. Bind a ClusterRole with that permission to the controller ServiceAccounts (or set FluxInstance `.spec.cluster.objectIdentityRBAC: true`).
- **Incomplete annotations**: `generic` ServiceAccount tokens need both `credential.auth.fluxcd.io/provider: kubernetes` and `credential.auth.fluxcd.io/type: jwt`; one without the other is an error, and they can't be combined with `imagePullSecrets`.
- **Lockdown default SA**: With the object identity lockdown (`--default-service-account`, `multitenantObjectIdentity`), objects without `serviceAccountName` use the tenant's default SA, not the controller identity.
- **`spiffe` provider**: Requires the SPIFFE Workload API socket plus `--spiffe-broker-endpoint` and `--spiffe-broker-id` flags on the controller.

### Inter-controller mTLS failures (SPIFFE, Flux 2.10+)

**Symptoms**: Kustomizations/HelmReleases fail to fetch artifacts while the source is `Ready`, or
events stop reaching notification-controller, after `--authorized-*-spiffe-id` flags were added.

**Diagnosis**: mTLS is enabled per traffic type by the presence of a flag, on both sides. Check that the
peer's flag is also set (e.g. `--authorized-artifact-client-spiffe-id` on source-controller pairs with
`--authorized-artifact-server-spiffe-id` on kustomize/helm-controller), that each flag holds the **peer's** SPIFFE ID
exactly as issued by the SPIFFE runtime, that `SPIFFE_ENDPOINT_SOCKET` and the socket mount exist on every
controller, and that the Services expose port 443 (`https`).

## Source Failures

### GitRepository fetch error

**Symptoms**: `FetchFailed` condition, Kustomizations/HelmReleases stuck on old revision.

**Common causes**:
- **Authentication error**: Secret missing, expired token, wrong SSH key format. Check that the `secretRef` secret exists and has correct keys (`username`/`password` for HTTPS, `identity`/`known_hosts` for SSH).
- **Repository not found**: Wrong URL, repo renamed/deleted, or private without auth.
- **Branch/tag not found**: Referenced branch or tag doesn't exist in the remote.
- **Timeout**: Large repository or slow network. Consider using `sparseCheckout`.
- **TLS error**: Self-signed certificate without `caFile` in the secret.

### OCIRepository fetch error

**Symptoms**: `FetchFailed` condition on the OCIRepository.

**Common causes**:
- **Authentication error**: Pull secret missing or expired. For cloud registries (ECR, GCR, ACR), check `.spec.provider` and workload identity configuration.
- **Image not found**: Wrong `oci://` URL or tag doesn't exist.
- **Signature verification failed**: Cosign verification enabled but artifact not signed, wrong public key, or wrong cosign secret.
- **Layer selector mismatch**: Wrong `mediaType` in `layerSelector` for the artifact type.

### HelmChart fetch error

**Symptoms**: `FetchFailed` condition on HelmChart, HelmRelease stuck.

**Common causes**:
- **Chart not found**: Chart name or version doesn't exist in the repository.
- **Source not ready**: The referenced HelmRepository has its own fetch errors.
- **Version constraint**: Semver range matches no available version.

## Kustomization Failures

### Build error

**Symptoms**: `Ready: False` with build error message.

**Common causes**:
- Invalid `kustomization.yaml` (syntax error, missing fields)
- Referenced resource file missing from the source artifact
- Patch target not found (strategic merge or JSON patch targets a non-existent resource)
- Variable substitution error (missing ConfigMap/Secret in `substituteFrom`)

### Apply error

**Symptoms**: `Ready: False` with validation or apply error message.

**Common causes**:
- Manifest validation error (invalid fields, missing required fields, wrong apiVersion)
- Immutable field changed (e.g., `spec.selector` on a Deployment) - suggest setting  `.spec.force: true`.

### Health check timeout

**Symptoms**: `HealthCheckFailed` condition, resources applied but not healthy.

**Common causes**:
- Deployment pods not becoming ready (image pull error, crashloop, probe failure)
- Insufficient timeout in `.spec.timeout` for slow-starting applications
- Health check targets wrong resource (check `.spec.healthChecks` entries)
- Dependent service not available (database, external API)

### RBAC error

**Symptoms**: `Ready: False` with "forbidden" or "unauthorized" error.

**Common causes**:
- Service account missing or doesn't have required permissions
- Role binding not created
- Multi-tenant mode restricts cross-namespace access

## HelmRelease Failures

### Install/upgrade failed

**Symptoms**: `Ready: False` with Helm error message, `Released: False`.

**Common causes**:
- **Chart rendering error**: Invalid values, required value missing, template error in the chart
- **Resource invalid**: Resource contains invalid fields or violates Kubernetes API constraints
- **CRD not installed**: Chart depends on CRDs that aren't present on the cluster
- **Namespace doesn't exist**: Target namespace not created

### Values merge error

**Symptoms**: `Ready: False` with values-related error.

**Common causes**:
- Referenced ConfigMap/Secret in `valuesFrom` doesn't exist
- Key specified in `valuesFrom[].valuesKey` not found in the ConfigMap/Secret
- YAML syntax error in the values ConfigMap/Secret

### Remediation exhausted

**Symptoms**: `RemediationFailed` condition, release stuck in failed state.

**Common causes**:
- Underlying issue not resolved (the install/upgrade keeps failing)
- Max retries reached in `.spec.install.remediation.retries` or `.spec.upgrade.remediation.retries`

Suggest setting modern `install.strategy.name: RetryOnFailure`
and `upgrade.strategy.name: RetryOnFailure` instead of the legacy remediation retries pattern.

## Image Automation Failures

### Tags not detected

**Symptoms**: ImageRepository `Ready: False`, or `Ready: True` but ImagePolicy selects no tag.

**Common causes**:
- **Registry auth error**: Pull secret missing/expired, or `.spec.provider` not set for cloud registries (ECR, ACR, GAR) using workload identity.
- **Policy matches nothing**: Semver range excludes all published tags, or tags don't parse as semver (use `filterTags` with `extract` to strip prefixes).
- **Controllers not running**: `image-reflector-controller`/`image-automation-controller` are optional FluxInstance components — verify they are listed and running.

### No commits pushed

**Symptoms**: ImagePolicy has a new `latestImage` but no commits appear in Git.

**Common causes**:
- **Read-only Git credentials**: The GitRepository `secretRef` used by ImageUpdateAutomation lacks push permission.
- **Missing markers**: Manifests under `.spec.update.path` have no `$imagepolicy` comment markers, or the marker references the wrong `<namespace>:<policy-name>`.
- **Wrong branch**: Commits go to `.spec.git.push.branch` — the user may be watching a different branch.
- **Nothing to update**: The tag in Git already matches `latestImage`.

## Notification Failures

### Alerts not delivered

**Symptoms**: Reconciliation events happen but nothing arrives in Slack/Teams/etc.

Provider and Alert have no status conditions — diagnose from
notification-controller logs, not object status.

**Common causes**:
- **Event filtered**: Alert `.spec.eventSeverity: error` drops info events; `.spec.eventSources` doesn't match the resource kind/name/namespace producing the event.
- **Provider auth/address error**: Wrong or expired token in the `secretRef`, wrong webhook `address` — look for HTTP 401/404/timeout errors in controller logs.
- **Wrong provider type**: `.spec.type` doesn't match the receiving service's expected payload format.
- **Secret keys rejected after upgrading to Flux 2.10** (applies to `v1beta3` objects too, the controller reads `v1`):
  - a `proxy` key in the `secretRef` Secret is rejected — move it to a Secret referenced by `.spec.proxySecretRef`;
  - commit status Providers (`github`, `gitlab`, `gitea`, `bitbucket`, `bitbucketserver`, `azuredevops`) fail when the `secretRef` Secret holds TLS data and `.spec.certSecretRef` is unset;
  - `azureeventhub` fails when its Secret has a `token` key (static JWT auth removed; use SAS `address` or object identity);
  - `bitbucket` rejects `<username>:<app-password>` tokens — use a Bitbucket Cloud API token.
- **Alert summary or Provider proxy gone after upgrade**: `v1` dropped Alert `.spec.summary` and Provider `.spec.proxy`/`.spec.interval`; the API server drops them when objects are served or written at the `v1` storage version, with no error. Restore with `.spec.eventMetadata.summary` and `.spec.proxySecretRef`. Warnings like `v1beta3 Alert is deprecated, upgrade to v1` mean manifests in Git still use `v1beta3` — run `flux migrate -f .`.

### Receiver not triggering

**Symptoms**: Pushes to the Git server don't trigger immediate reconciliation.

**Common causes**:
- **Webhook not reaching the cluster**: No incoming requests in notification-controller logs — wrong URL on the Git server, ingress/firewall blocking, or webhook not configured at all. Verify `status.webhookPath`.
- **Signature validation failure**: Webhook secret mismatch between the Git server and the Receiver `secretRef`.
- **Wrong resources list**: `.spec.resources` doesn't include the source/Kustomization that should be reconciled.

## General Debugging Checklist

Use this checklist when the issue is unclear:

1. **Check Flux installation**: `get_flux_instance` — is Flux healthy?
2. **Check the resource**: Use `get_kubernetes_resources` to look at status. Is it `Ready`? Any error conditions?
3. **Check events**: Use `get_kubernetes_resources` to look at the resource events for recent error.
4. **Check the source**: Is the referenced source (GitRepository, OCIRepository, etc.) `Ready`?
5. **Check dependencies**: Does the resource have `dependsOn`? Are those dependencies ready?
6. **Check the managing resource**: What Kustomization/ResourceSet created this resource? Is it healthy?
7. **Check RBAC**: If using `serviceAccountName`, does the service account have required permissions?
8. **Check logs**: Look at pod logs for detailed error messages.
9. **Check cluster nodes**: Use `get_kubernetes_resources` to list nodes. Is the cluster under resource pressure (CPU, memory, disk)?

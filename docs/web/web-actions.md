---
title: Flux Web UI Actions
description: Guide for user actions in the Flux Web UI.
---

# Flux Web UI Actions

The Flux Web UI provides interactive actions that allow users to manage
Flux resources and Kubernetes workloads directly from the browser.
Actions include triggering reconciliations, suspending and resuming resources,
downloading artifacts, restarting workloads, running jobs, deleting pods,
and viewing pod logs.

Actions are only available when [authentication](web-user-management.md) is configured.
Each action requires specific RBAC permissions granted through Kubernetes
`ClusterRole` or `Role` resources. The Flux Operator ships with
two predefined roles: `flux-web-user` (read-only) and `flux-web-admin`
(full access including actions). See the
[Role-Based Access Control](web-user-management.md#role-based-access-control) section
for details on assigning roles to users and groups.

## Access Modes

The `.spec.userActions.access` field in the
[Web Config API](web-config-api.md#access-mode) determines which Kubernetes
identity performs user actions that use per-action RBAC verbs:

- `Impersonated` (default): The Web UI checks that the user has the per-action
  RBAC verb, then performs the action as that user. The user must also have
  every native Kubernetes verb required by the operation.
- `FineGrained`: The Web UI checks that the user has the per-action verb,
  then performs the native Kubernetes operations using the Web UI service
  account. The user does not need the corresponding native verbs unless the
  action verb is itself native, as with `delete` for Pod deletion.

In both modes, users still need read permissions to find and view resources in
the Web UI. The access mode only changes the permissions used to perform an
action. Viewing pod logs is a Web UI user action and is disabled when
authentication is not configured. It has no per-action RBAC verb: the user must
hold the native `get` verb on `pods/log` in either access mode, and the request
is always performed with the user's identity.

To enable fine-grained access with the Flux Operator Helm chart, configure:

```yaml
web:
  config:
    userActions:
      access: FineGrained
```

For a [standalone Web UI deployment](web-standalone.md), also set the chart's
`web.userActions.access` value so that the Web UI service account is granted the
native permissions it needs:

```yaml
web:
  config:
    userActions:
      access: FineGrained
  userActions:
    access: FineGrained
```

The embedded Web UI uses the Flux Operator service account, which already has
the necessary cluster-wide permissions. If the standalone service account lacks
a required permission, the action returns an internal error explaining that the
Web UI application, rather than the user, needs additional RBAC.

## GitOps Actions

GitOps actions operate on Flux resources such as Kustomizations, HelmReleases,
and source resources. These actions are available on the resource detail page
for all reconcilable Flux resources.

### Reconcile

The Reconcile action triggers an immediate reconciliation of a Flux resource,
bypassing the regular reconciliation interval. For HelmRelease and
ResourceSetInputProvider resources, a force request is also added
to ensure the reconciliation runs even when no changes are detected
or the schedule hasn't elapsed.

!!! note "RBAC"

    The user always needs the `reconcile` verb on the target resource.
    With `Impersonated` access, the user also needs `get` and `patch`.
    With `FineGrained` access, the Web UI service account needs `get` and
    `patch` instead.

    For example, to allow reconciling Kustomizations with fine-grained access,
    grant the user `reconcile` on `kustomizations` in the
    `kustomize.toolkit.fluxcd.io` API group.

### Pull

The Pull action reconciles the upstream source (GitRepository, OCIRepository,
Bucket, or HelmChart) of a Kustomization or HelmRelease. This is useful
when you want to fetch the latest changes from the source repository
without waiting for the source's reconciliation interval.

The Pull action is not available when the resource or its source is suspended,
or when the source is an ExternalArtifact.

!!! note "RBAC"

    The user always needs the `reconcile` verb on the source resource.
    With `Impersonated` access, the user also needs `get` and `patch`.
    With `FineGrained` access, the Web UI service account needs `get` and
    `patch` instead.

### Suspend and Resume

The Suspend action pauses reconciliation of a Flux resource. While suspended,
the resource will not be reconciled by its controller. The Web UI displays
which user suspended the resource.

The Resume action re-enables reconciliation and triggers an immediate
reconciliation request. Resuming removes the suspension tracking annotation.

!!! note "RBAC"

    The user always needs `suspend` to suspend a resource and `resume` to
    resume it. With `Impersonated` access, the user also needs `get` and
    `patch` on the target resource. With `FineGrained` access, the Web UI
    service account needs `get` and `patch` instead.

### Download Artifact

The Download Artifact action downloads the artifact tarball
from a Flux source resource. This is available for Bucket, GitRepository,
OCIRepository, HelmChart, and ExternalArtifact resources.

For ArtifactGenerator resources, a dropdown menu is displayed listing
all ExternalArtifacts from the resource's inventory. Each artifact
can be downloaded individually.

!!! note "RBAC"

    The user always needs the `download` verb on the source resource.
    With `Impersonated` access, the user also needs `get` so the Web UI can
    read the artifact URL from the resource status. With `FineGrained` access,
    the Web UI service account needs `get` instead.

    For example, to download an OCIRepository artifact with fine-grained
    access, grant the user `download` on `ocirepositories` in the
    `source.toolkit.fluxcd.io` API group.

## Workload Actions

Workload actions operate on Kubernetes workloads managed by Flux.
These actions are available on the workload detail page for
Deployments, StatefulSets, DaemonSets, CronJobs, and Pods.

### View Logs

The View Logs action streams a pod's container logs in a read-only viewer.
It is available per pod from the Pods list and from the action bar as
a "View logs" button that aggregates the logs of all the workload's pods.

!!! note "RBAC"

    Authentication must be configured to enable the action, and the user must
    have the `get` verb on the `pods/log` subresource in the core API group.
    This requirement is the same for both access modes because viewing logs
    does not use a custom RBAC verb and is not controlled by
    `.spec.userActions.access`.

### Rollout Restart

The Rollout Restart action triggers a rolling restart of a Deployment,
StatefulSet, or DaemonSet by patching the pod template with a
`kubectl.kubernetes.io/restartedAt` annotation. This causes the controller
to recreate all pods in a rolling fashion, similar to `kubectl rollout restart`.

!!! note "RBAC"

    The user always needs the `restart` verb on the workload resource.
    With `Impersonated` access, the user also needs `patch`. With
    `FineGrained` access, the Web UI service account needs `patch` instead.

    For example, to restart a Deployment with fine-grained access, grant the
    user `restart` on `deployments` in the `apps` API group.

### Run Job

The Run Job action creates a new Job from a CronJob's job template,
triggering an immediate execution outside of the CronJob's schedule.
The created Job is owned by the CronJob so that its pods appear
under the CronJob in the Web UI. The Web UI displays which user
created the Job.

!!! note "RBAC"

    The user always needs the `restart` verb on `cronjobs`. With
    `Impersonated` access, the user also needs `get` on `cronjobs` and `create`
    on `jobs`. With `FineGrained` access, the Web UI service account needs
    those native permissions instead.

### Delete Pod

The Delete Pod action deletes an individual pod. For pods owned by a
Deployment, StatefulSet, or DaemonSet, the Kubernetes controller will automatically
recreate the pod. A confirmation dialog is shown before the deletion
is performed.

!!! note "RBAC"

    Requires the `delete` verb on `pods` in the core API group in both access
    modes. For this action, the action verb is also the native Kubernetes verb,
    so granting it allows the user to delete pods directly with other
    Kubernetes clients as well as through the Web UI.

## Audit

The Web UI can generate audit events for user actions that have per-action
RBAC verbs, providing a trail of who performed what action and when. Audit
events are recorded as Kubernetes Events and forwarded to Flux's
notification-controller, enabling integration with external notification
systems. Viewing pod logs is not audited because it relies only on the native
`get` verb and is not part of the configurable audit actions.

### Enabling Audit

To enable auditing, set `spec.userActions.audit` in the
[web configuration](web-config-api.md) to a list of actions to audit.
Use the special value `["*"]` to audit all actions that support auditing.

```yaml
web:
  config:
    userActions:
      audit:
        - "*"
    authentication:
      type: OAuth2
      oauth2:
        provider: OIDC
        # ... omitted for brevity ...
```

To audit only specific actions, list them individually:

```yaml
web:
  config:
    userActions:
      audit:
        - reconcile
        - suspend
        - resume
        - download
        - restart
        - delete
```

### Audit Events

Each audited action generates a Kubernetes Event with the reason `WebAction`.
The event message includes the username, action, and target resource.
The following annotations are set on the event:

| Annotation | Description |
|---|---|
| `event.toolkit.fluxcd.io/action` | The action performed (e.g., `reconcile`, `suspend`) |
| `event.toolkit.fluxcd.io/username` | The username of the user who performed the action |
| `event.toolkit.fluxcd.io/groups` | The groups of the user who performed the action |
| `event.toolkit.fluxcd.io/subject` | The target workload (for workload actions only) |

For workload actions (rollout restart, create job, delete pod), the audit event
is associated with the Flux resource managing the workload
(e.g., the Kustomization, HelmRelease or ResourceSet).

### Notifications with Flux Alerts

Audit events are automatically forwarded to Flux's notification-controller
when it is installed in the cluster. You can configure Flux `Alert` and `Provider`
resources to send audit notifications to external systems such as Slack,
Microsoft Teams, or any webhook endpoint.

Example configuration for sending audit notifications to Slack:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: web-actions-audit
  namespace: apps
spec:
  providerRef:
    name: slack
  eventSources:
    - kind: Kustomization
      name: "*"
    - kind: HelmRelease
      name: "*"
    - kind: ResourceSet
      name: "*"
  inclusionList:
    - ".*on the web UI$"
```

The `inclusionList` regex filter matches the audit event message format,
ensuring that only Web UI action events are forwarded to the notification provider.

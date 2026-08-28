---
title: Flux Web UI RBAC Minimization & Least Privilege
description: Transparency documentation for all elevated system privileges in the Flux Web UI backend.
---

# Flux Web UI RBAC Minimization & Least Privilege

The Flux Web UI backend enforces strict
[Role-Based Access Control](web-user-management.md#role-based-access-control)
by impersonating the authenticated user for every Kubernetes API call.
However, a small number of operations intentionally bypass user impersonation
and run with the operator's own service-account privileges instead.

Most such internal operations are implemented by calling an internal
`WithPrivileges()` option on the Kubernetes client. The documented exception is
namespace visibility: the kubeclient wrapper directly uses its privileged base
client to enumerate namespaces before filtering the result through the user's
RBAC checks. This page documents **every** elevated backend path, explains how
it leverages system privileges to minimize the amount of RBAC permissions
administrators need to grant to users, and describes how the system fulfills
these critical internal requirements without exposing sensitive data.

By relying on the system to safely handle these internal operations,
administrators can enforce a much stricter least-privilege posture.

## Guiding Principles

1. **Least privilege by default.** All resource reads and writes go through
   the impersonated user client unless there is a documented reason not to.
2. **No sensitive data exposure.** System calls never return Secret values,
   ConfigMap data, or any other sensitive content to the user.
3. **RBAC Minimization.** Each usage of elevated system privileges exists because it enables a
   specific, high-value feature that significantly decreases the permissions
   that users would otherwise require, improving support for the principle of least privilege.
4. **Web UI scope.** Operator reconciliation paths outside the Web UI backend
   are documented here only when they change Web UI RBAC requirements.

---

## 1. CronJob Pod Listing

**Where:** Workload detail page – listing pods for a CronJob.

**Internal operation:**
The system reads Jobs and Pods created by a CronJob on behalf of a user who only has read access to the CronJob itself.

**How it works:**
CronJob ownership is cascading (CronJob → Job → Pod). The operator's
controller-runtime cache maintains a server-side field index
(`metadata.ownerReferences.cronJob`) that maps Jobs to their owning
CronJob. This index is only available on the privileged cached client
because it was registered at startup with the operator's own credentials.
The privileged client is used solely to query this index for Jobs owned
by the CronJob; the resulting Pod statuses (name, phase, timestamps) are
returned to the user without exposing any sensitive pod spec data.

**Least privilege benefit:**
Without this internal usage, users would need explicit read permissions on all Jobs and Pods just to see their scheduled workloads running. By handling this internally, we limit the user's required RBAC to just the CronJob itself while still providing critical observability.

---

## 2. Flux Resource GVK Resolution

**Where:** Any page that displays a Flux custom resource
(Kustomizations, HelmReleases, Sources, etc.).

**Internal operation:**
The system makes a REST API discovery call to resolve the preferred `GroupVersionKind` for a Flux resource kind.

**How it works:**
To correctly fetch or list a Flux resource, the backend must know which
API version the cluster considers "preferred" (e.g., `v1` vs `v1beta2`
of a Kustomization). The preferred version is obtained from the Kubernetes
API server's discovery endpoint via the REST mapper. The privileged client
is used for this single discovery call because the REST mapper is a
cluster-level metadata operation that does not read any actual resource
data.

**Least privilege benefit:**
API discovery is a metadata-only operation that returns no workload data,
no secrets, and no resource content. If we required the user to have
explicit RBAC permissions for API discovery, every user role would need
extra rules for a purely internal concern. The system handles this to keep user RBAC configuration simple and minimal, preventing confusing errors on resources the user genuinely has permission to view.

---

## 3. Audit Event Recording

**Where:** After any user-initiated action (reconcile, suspend, resume,
restart, delete pod, run job, download artifact).

**Internal operation:**
The system fetches the Flux resource managing the target workload, reads the FluxInstance for the notification-controller endpoint, and creates the Kubernetes Event.

**How it works:**
When a user performs an action on a workload (e.g., restarting a
Deployment), the audit system needs to associate the event with the Flux
resource that owns that workload (e.g., the Kustomization). To do this,
it walks the workload's reconciler-ref label or owner-reference chain
using the privileged client. It then reads the FluxInstance to find the
notification-controller address and emits a Kubernetes Event tied to the
managing Flux resource.

**Least privilege benefit:**
Audit is a security feature, not a user-facing data feature. The user has
already been authorized to perform the action itself (via their own RBAC). By using the system client, we guarantee that every auditable action produces a complete, traceable event regardless of the acting user's read permissions. Administrators don't need to weaken audit coverage or inflate user RBAC just to ensure logs are written. No data from the privileged reads is returned to the user.

---

## 4. Audit Pod-to-Workload Resolution

**Where:** After a user deletes a Pod via the Web UI.

**Internal operation:**
The system reads the Pod's owner chain (ReplicaSet or Job, up to Deployment or CronJob) to record the correct audit event.

**How it works:**
When a user deletes a Pod, the audit system resolves the Pod's ownership
chain (Pod → ReplicaSet/Job → Deployment/CronJob) to find the top-level
workload. It then looks up the Flux resource managing that workload so
the audit event is associated with the correct Flux resource. This entire
chain traversal uses the privileged client.

**Least privilege benefit:**
The user has already been authorized to delete the Pod, which is a
destructive action with a higher privilege bar than reading. Walking the
owner chain to produce a meaningful audit trail is an internal task that
enables administrators to correlate pod deletions back to the Flux
pipeline that manages them. The system handles this resolution internally without requiring the user to have read access to all intermediate resources, keeping RBAC minimal.

---

## 5. Cluster-Wide Report Building

**Where:** The main dashboard, the periodic background report refresh,
the Workloads search page, and the global quick-search workload results.

**Internal operation:**
The system scans all Flux custom resources across all namespaces to compute
reconciler statistics, build the `FluxReport`, and extract Kubernetes workload
references from Flux applier inventories.

**How it works:**
The operator builds a `FluxReport` by scanning all Flux custom resources
(Kustomizations, HelmReleases, Sources, etc.) across all namespaces,
computing reconciler statistics, and aggregating the results into a
single report object. This report is built periodically on a background
goroutine and cached. When a user requests the report, the cached data
is filtered to show only the namespaces the user has access to.

During the same scan, the reporter also reads applier
`status.inventory.entries`, keeps local Kubernetes workload entries
(Deployment, StatefulSet, DaemonSet, CronJob), stamps them with the owning
reconciler's reference and status, and caches them in a workload index.
Appliers targeting remote clusters are skipped. The Workloads page and
global quick-search workload results are served from this cache and filtered
to the namespaces the user can access.

**Least privilege benefit:**
The report is the backbone of the Web UI dashboard. Building it requires
cross-namespace visibility that no single user is guaranteed to have —
especially in multi-tenant clusters. The privileged scan exposes summarized
reconciler data (counts, readiness percentages, status summaries) and
inventory-derived workload reference (kind, namespace, name, apiVersion, and
parent reconciler reference/status), never workload specs, workload status
conditions, pod data, or secrets. By handling this internally and filtering the
response based on the user's namespace access, we avoid granting users
cluster-wide read access or `apps`/`batch` read permissions while still
delivering a meaningful, isolated dashboard and workload search experience.

---

## 6. Pod Metrics and Workload Usage

**Where:** The main dashboard Flux controller resource usage display, the
workload dashboard Resource Usage charts and per-pod usage in the Pods tab, and
the Flux resource dashboard Resource Usage tab.

**Internal operation:**
The system collects current pod CPU/memory usage for all pods from the
Kubernetes Metrics API into an in-memory ring buffer. It also reads Flux
controller pod specs in the operator namespace for requests/limits and may read
named workload generation object metadata to draw rollout markers on usage
charts.

**How it works:**
A background collector queries the Metrics API (`metrics.k8s.io/v1beta1`)
cluster-wide with the privileged client and retains a ~30 minute usage window
per container in memory. The Metrics API only serves instantaneous values, so
history must be accumulated server-side. The scrape interval defaults to one
minute and is configurable via `spec.metrics.scrapeInterval`; collection can be
disabled with `spec.metrics.disabled`.

When building the report, the operator lists pods labeled
`app.kubernetes.io/part-of=flux` in its own namespace with the privileged
client, attaches their latest usage from the pod metrics collector, and includes
the requests/limits read from the pod specs.

When a user opens a workload dashboard, the backend first fetches the workload
with the user's impersonated client as the access-control gate. Only after that
does it attach usage series for the workload's current pods and
selector-matching buffered pods so the history stays continuous across rollouts.
Off-schedule catch-up scrapes for running pods without a recent sample are
rate-limited to one cluster-wide scrape per 15 seconds regardless of request
volume.

The Flux resource dashboard reuses the same cache for its Resource Usage tab:
the workloads batch endpoint fetches each inventoried workload with the user's
impersonated client and aggregates series from the in-memory buffer. Workloads
the user cannot read are reported without usage data. This path does not add
per-request Metrics API calls; metrics reads are performed by the background
collector.

The rollout marker uses a similarly gated path. After the workload has been
fetched with the user's impersonated client, the backend resolves the current
generation object names and fetches only those named objects' metadata with the
privileged API reader: Deployments use the highest deployment revision among the
workload's pod ReplicaSets, StatefulSets use `status.updateRevision`, and
DaemonSets use the newest ControllerRevision referenced by the workload's pods.
No generation objects are listed and no spec or data fields are read; the
response yields a single timestamp (`rolledOutAt`).

**Least privilege benefit:**
Users see CPU/memory usage only for workloads they can already view, and
controller usage without needing access to `flux-system` pod specs.
Administrators are not forced to grant every dashboard user cluster-wide read
access on `metrics.k8s.io`, ReplicaSets, or ControllerRevisions. The exposed data
is limited to CPU/memory usage, Flux controller resource requests/limits, and
the workload rollout timestamp. Clusters without metrics-server simply hide the
usage features.

---

## 7. Fine-Grained User Actions

**Where:** Flux resource actions, artifact downloads, workload restarts, CronJob
job runs, and Pod deletions triggered through the Web UI when
`.spec.userActions.access` is configured as `FineGrained`. Pod log viewing is a
Web UI user action and is disabled when authentication is not configured, but
it is not part of this elevated path: it has no custom RBAC verb and always uses
the user's native `get` permission on `pods/log`.

**Internal operation:**
The system performs the native Kubernetes reads and mutations required by an
action after confirming that the user possesses its specific action verb, such
as `suspend`, `download`, or `restart`.

**How it works:**
In the default `Impersonated` mode, the backend verifies the action verb and
performs the operation using the user's impersonated client. The user must
therefore hold both the action verb and all native verbs used by the operation,
such as `get`, `patch`, or `create`. In `FineGrained` mode, the action verb is
still checked against the user's identity, but the backend uses its privileged
client for the supporting reads and the action itself. For artifact downloads,
only the requested artifact is returned to the authorized user; data from other
privileged reads is not exposed.

Pod deletion is a special case: its action verb, `delete`, is also the native
Kubernetes verb. Granting that action therefore permits direct Pod deletion in
either access mode.

**Artifact fetch hardening:**
The artifact download action retrieves the artifact from the URL advertised in
the source's status over plain HTTP. This fetch is not a Kubernetes API call,
so it runs with the backend pod's own network identity in either access mode.
To keep the fetch scoped to Flux artifact servers, the backend does not follow
redirects and requires DNS hostnames; literal IPv4 and IPv6 hostnames are
rejected. It resolves every destination address, rejects the request if any
address is loopback, link-local, or the IPv6 instance metadata service address
(`fd00:ec2::254`), and connects directly to a validated address to prevent DNS
rebinding between validation and connection. Other private addresses remain
reachable through DNS names so artifacts can be served by in-cluster Flux
controllers.

**Least privilege benefit:**
Without fine-grained access, native verbs such as `patch` can let users bypass
the action boundaries enforced by the Web UI and make unrelated changes with
`kubectl` or other SSO-integrated tools. Moving those native operations to the
Web UI service account lets administrators grant individual action verbs while
keeping users' direct Kubernetes access read-only, except where the action verb
is itself native, as with Pod deletion.

---

## 8. Namespace Visibility

**Where:** Namespace search filter dropdown and dashboard statistics filtering.

**Internal operation:**
The kubeclient wrapper lists all namespaces with its privileged base client to
determine which ones the user is permitted to view.

**How it works:**
To populate the namespace filter dropdown and filter the main dashboard
statistics, the backend needs to know which namespaces the user is allowed to
see. This path does not call `WithPrivileges()` because it already runs inside
the kubeclient wrapper: `ListUserNamespaces` uses the wrapper's privileged base
client to list all namespaces, and then performs a `SelfSubjectAccessReview` for
the user to check if they have `get` permissions on `ResourceSets` in each
namespace. If they do, the namespace's existence is revealed to the user in the
UI.

**Least privilege benefit:**
Users do not need cluster-wide `list` permissions on namespaces just to populate the UI dropdown. The system determines what the user is allowed to see internally, keeping user permissions tightly scoped to only the resources they actively manage. This preserves strict multi-tenant boundaries by removing the need for broad, cluster-level namespace access.

---

## Summary

| # | Feature                              | Internal Operation                                   | Data Exposed to User                                                                           |
|---|--------------------------------------|------------------------------------------------------|------------------------------------------------------------------------------------------------|
| 1 | CronJob pod listing                  | System reads Jobs/Pods                               | Pod name, phase, timestamps                                                                    |
| 2 | Flux GVK resolution                  | System API discovery                                 | None (internal metadata only)                                                                  |
| 3 | Audit event recording                | System writes event                                  | None (server-side only)                                                                        |
| 4 | Audit pod-owner resolution           | System reads owner chain                             | None (server-side only)                                                                        |
| 5 | Dashboard report and workloads index | System scans Flux resources and applier inventories  | Aggregated stats and workload reference + parent reconciler status, filtered by user namespace |
| 6 | Pod metrics and workload usage       | System scrapes Metrics API cluster-wide and reads named generation metadata | CPU/memory usage for permitted workloads, Flux controller requests/limits, and workload rollout timestamp |
| 7 | Fine-grained user actions            | System performs native action operations             | Requested artifact for downloads; action result only otherwise                                 |
| 8 | Namespace visibility                 | Wrapper lists namespaces with privileged base client | Visible namespace names after RBAC filtering                                                   |


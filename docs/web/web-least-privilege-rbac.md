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

## 6. Flux Controller Pod Metrics

**Where:** The main dashboard – Flux controller resource usage display.

**Internal operation:**
The system reads pod metrics (CPU, memory) from the Kubernetes Metrics API for Flux controller pods.

**How it works:**
The operator queries the Metrics API (`metrics.k8s.io/v1beta1`) for pods
labeled `app.kubernetes.io/part-of=flux` in the operator's namespace. It
also reads the pod specs to determine resource limits. The privileged
client is used because the Metrics API call and the pod spec read target
the operator's own namespace (typically `flux-system`), which users may
not have access to.

**Least privilege benefit:**
Flux controller health is a cluster-wide concern visible to all dashboard
users, regardless of their namespace-scoped permissions. Showing CPU and
memory utilization of Flux controllers helps users understand whether Flux itself is under resource pressure. The metrics data contains no sensitive information. By fetching this internally, administrators are not forced to break namespace isolation by granting everyone read access to the `flux-system` namespace.

---

## 7. Fine-Grained GitOps Actions

**Where:** Any GitOps action (e.g. suspend, resume, reconcile, delete, download) triggered via the Web UI when `.spec.userActions.access` is configured as `FineGrained`.

**Internal operation:**
The system performs the actual resource mutation on the target resource, as long as the user possesses the specific custom action verb (e.g. `suspend` or `delete`).

**How it works:**
Normally, GitOps actions are executed using the user's impersonated client, which requires the user to hold native Kubernetes verbs such as `patch` for reconcile/suspend/resume or `delete` for resource deletion. When fine-grained access control is enabled, the backend verifies that the user holds the specific custom verb for the action, and if so, it uses the privileged client to perform the underlying patch or delete operation, removing the need for user impersonation during the mutation.

**Least privilege benefit:**
This feature is crucial for least-privilege security scenarios. If users were required to have native `patch` or `delete` verbs to operate resources, they could potentially bypass the Web UI and perform unauthorized modifications via `kubectl` or other SSO-integrated tools. By handling the mutation internally, cluster administrators can assign restrictive, fine-grained access policies ensuring tenants can solely perform permitted actions.

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

| # | Feature                     | Internal Operation                                 | Data Exposed to User                                                          |
|---|-----------------------------|----------------------------------------------------|-------------------------------------------------------------------------------|
| 1 | CronJob pod listing         | System reads Jobs/Pods                             | Pod name, phase, timestamps                                                   |
| 2 | Flux GVK resolution         | System API discovery                               | None (internal metadata only)                                                 |
| 3 | Audit event recording       | System writes event                                | None (server-side only)                                                       |
| 4 | Audit pod-owner resolution  | System reads owner chain                           | None (server-side only)                                                       |
| 5 | Dashboard report and workloads index | System scans Flux resources and applier inventories | Aggregated stats and workload reference + parent reconciler status, filtered by user namespace |
| 6 | Controller metrics          | System reads metrics API                           | CPU/memory usage of Flux controllers                                          |
| 7 | Fine-Grained GitOps Actions | System patches or deletes resource                 | None (server-side mutation only)                                              |
| 8 | Namespace visibility        | Wrapper lists namespaces with privileged base client | Visible namespace names after RBAC filtering                                  |

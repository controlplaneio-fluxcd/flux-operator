# Flux Web UI

**Mission control dashboard for Kubernetes app delivery powered by Flux CD**

A lightweight, mobile-friendly web interface for monitoring and operating your GitOps pipelines.
Embedded directly within the Flux Operator, it requires no additional installation.

Designed for DevOps engineers and platform teams, the Web UI shows the state of Flux and the
workloads it manages, and lets you act on them (reconcile, suspend, restart, delete pods, download
artifacts) without the CLI, guarded by Kubernetes RBAC.

## Features

- **Operational Insight:** View the status and readiness of all Flux resources and the workloads they manage.
- **GitOps Actions:** Reconcile, suspend and resume Flux resources, reconcile sources, download artifacts, rollout restart workloads, run CronJobs and delete pods. Each action is gated by Kubernetes RBAC and can be recorded as an audit event.
- **Pinpoint Issues:** Identify failing resources, unhealthy workloads and node problems from the cluster dashboard.
- **Navigate Efficiently:** Search Flux resources and workloads by kind, namespace, name and status, jump anywhere with quick search (`/`) and use keyboard shortcuts (`?` lists them).
- **Deep Dive:** Dedicated dashboards for Flux resources (Kustomizations, HelmReleases, ResourceSets, sources, etc.), Kubernetes workloads (Deployments, StatefulSets, DaemonSets, CronJobs) and cluster nodes.
- **Inspect Logs:** View the logs of workload pods, including the previous container, scoped to your RBAC permissions (requires authentication).
- **Track Resource Usage:** CPU and memory charts covering the last 30 minutes (requires metrics-server). Pods approaching their limits are highlighted. Flux resource dashboards aggregate the usage of all managed workloads, with per-workload breakdowns.
- **Node Health:** Spot unreachable nodes, kubelet pressure, memory overcommitment, unschedulable pods and missing N-1 headroom.
- **Favorites:** Mark Flux resources and workloads as favorites for quick access.
- **Multi-Tenancy:** Lists, searches and events only show the namespaces the user can access.
- **Mobile-Optimized:** Fully responsive interface for on-the-go checks.
- **Theming:** Light, dark and system themes.

## Dashboards

### Cluster Dashboard

Shows the overall Flux status, the Kubernetes, Flux and operator versions, the CPU and memory usage of Flux,
the cluster sync status, the status and resource usage of each Flux controller, and the running, failing
and suspended counts for every Flux kind. The Cluster Nodes panel summarizes node health and the CPU and
memory usage and requests of the cluster, without exposing node details.

### Flux Resource Dashboard

View the current state, conditions, events and reconciliation history of a Flux resource. Panels cover
the source and its artifact, the HelmRelease values, ResourceSet inputs and the managed objects:
an inventory list with live status and the YAML of each object, a pipeline graph, and the aggregated
CPU and memory usage of the managed workloads. Trigger reconcile, reconcile source, suspend, resume and
artifact downloads, guarded by Kubernetes RBAC.

### Workload Dashboard

Monitor the workloads managed by Flux. Trace the delivery pipeline from source to running pods, inspect
pods, containers, events, spec and status, and track CPU and memory usage with the last rollout marked
on the charts. Trigger reconcile, rollout restart (or run a Job from a CronJob), delete pods and view
logs, guarded by Kubernetes RBAC.

### Nodes Dashboard

Check the health and capacity of the cluster nodes. The dashboard runs the health checks below, lists the
nodes worst first with their issues, taints and system info, and ranks the Flux-managed workloads using the
most memory above their requests. Available to users allowed to list nodes, from the Cluster Nodes panel.

| Check                 | Severity                                     | Rule                                                                                                  |
|-----------------------|----------------------------------------------|-------------------------------------------------------------------------------------------------------|
| Node readiness        | critical                                     | `Ready` condition `Unknown` (unreachable) or `False`                                                  |
| Memory pressure       | critical                                     | `MemoryPressure` condition `True`                                                                     |
| Disk pressure         | critical                                     | `DiskPressure` condition `True`                                                                       |
| PID pressure          | critical                                     | `PIDPressure` condition `True`                                                                        |
| Network               | critical                                     | `NetworkUnavailable` condition `True`                                                                 |
| Node conditions       | warning                                      | Any other node condition `True`, e.g. `KernelDeadlock` from node-problem-detector                     |
| Memory usage          | warning at 80%, critical at 90%              | Node memory working set                                                                               |
| CPU usage             | warning at 80%, critical at 95%              | Node CPU usage                                                                                        |
| Memory overcommitment | warning or advisory                          | Memory limits above 100%; warning when the node uses 70% with 10% above requests, advisory at 150%    |
| OOM kills             | warning                                      | Containers OOM-killed on the node in the last hour                                                    |
| Pod scheduling        | warning                                      | Pending pods with reason `Unschedulable`                                                              |
| N-1 headroom          | warning                                      | Requests above the allocatable capacity minus the largest node                                        |
| Pod capacity          | warning at 90%                               | Pods out of the node's allocatable pod slots                                                          |
| Heartbeat             | warning above 20s, critical above 40s        | Age of the node Lease renewal in `kube-node-lease`                                                    |
| Readiness flapping    | warning                                      | More than 2 `Ready` transitions in 15 minutes                                                         |
| Requests              | warning at 90%                               | Node CPU or memory requests                                                                           |
| Memory requests       | advisory                                     | Pods without a memory request                                                                         |
| Memory limits         | advisory                                     | Pods with a memory request and no memory limit                                                        |
| Cordoned nodes        | advisory                                     | Node marked unschedulable                                                                             |
| Kubelet versions      | advisory, warning outside the supported skew | More than one kubelet version; warning when newer than the API server or more than 3 minors older     |

Percentages are shares of the node allocatable capacity. Cordoned nodes count as advisories only,
and control-plane nodes are left out of the capacity totals. Without metrics-server the usage checks
are skipped.

### Log Viewer

Tail and follow pod logs, including the previous container, in a full-screen viewer. The builtin parser
detects log levels and folds stack traces for popular logging frameworks across Go, Java, .NET, Node.js,
Python, Ruby, PHP and JSON formats. Filter by pod, container, level and keyword (`!` to exclude), pick JSON
fields, switch between formatted and raw output, and download raw logs.

### Event Viewer

Browse the Kubernetes events of Flux resources across your cluster. Filter by resource name with wildcard
and exclusion support, by namespace, kind and severity to spot reconciliation failures.

### Search

The Resources view lists every Flux resource, filtered by kind, namespace, name and status. The Workloads
view lists the workloads managed by Flux, filtered by kind, namespace and name. Quick search (`/`) jumps to
any resource or workload, with `ns:` and `kind:` filters and recently visited items.

### Favorites

Pin your most important Flux resources and workloads. The favorites view shows their status at a glance.

### GitOps Graph

Each Flux resource dashboard draws its delivery pipeline: the sources, the reconciler and the managed
Flux resources, workloads and other objects, with their status and links to their dashboards.

### Reconciliation History

Review the past reconciliations of Kustomizations, HelmReleases, ResourceSets and FluxInstances:
status, time, duration, count, digest and revision or chart version per entry.

### Single Sign-On

Sign in with your organization identity provider through OAuth2 and OpenID Connect. Token claims are mapped
to Kubernetes users and groups with CEL, and every request is impersonated for fine-grained RBAC.
Actions, logs and Helm values require authentication.

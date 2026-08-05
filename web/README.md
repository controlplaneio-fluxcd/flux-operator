# Flux Web UI

**Mission control dashboard for Kubernetes app delivery powered by Flux CD**

A lightweight, mobile-friendly web interface providing real-time visibility into your GitOps pipelines.
Embedded directly within the Flux Operator, it requires no additional installation.

Designed for DevOps engineers and platform teams, the Web UI offers direct insight
into your Kubernetes clusters. It allows you to track app deployments, monitor
controller readiness, and troubleshoot issues instantly, without needing to access the CLI.

## Features

- **Operational Insight:** View the real-time status and readiness of all workloads managed by Flux.
- **Monitor Reconciliation:** Track the sync state of GitOps pipelines across your cluster and infrastructure.
- **Pinpoint Issues:** Quickly identify and troubleshoot failures within your app delivery pipelines.
- **Navigate Efficiently:** Use advanced search and filtering to find specific resources instantly.
- **Deep Dive:** Access dedicated dashboards for Flux resources (HelmReleases, ResourceSets, etc.) and Kubernetes Workloads (Deployments, StatefulSets, DaemonSets, CronJobs).
- **Inspect Logs:** View the logs of workload pods directly from the browser, scoped to your RBAC permissions.
- **Track Resource Usage:** Monitor CPU and memory usage of workloads with charts covering the last 30 minutes (requires metrics-server). Pods approaching their CPU or memory limits are highlighted to signal throttling and OOM risk. Flux resource dashboards aggregate the usage of all managed workloads, with per-workload breakdowns linking to the workload dashboards.
- **Favorites:** Mark important resources as favorites for quick access and at-a-glance status monitoring.
- **Mobile-Optimized:** Stay informed with a fully responsive interface designed for on-the-go checks.
- **Adaptive Theming:** Toggle between dark and light modes to suit your environment and preference.

## Dashboards

### Cluster Dashboard

Get a complete overview of your Flux installation at a glance. The cluster dashboard displays the status of all Flux controllers, recent reconciliation activity, and quick stats about your GitOps resources including Kustomizations, HelmReleases, and source repositories.

### Flux Resource Dashboard

Dive deep into individual Flux configurations. View the current state, revision history, applied values, and any conditions or errors. Track the aggregated CPU and memory usage of the managed workloads with the last reconciliation marked on the charts. Trigger Flux actions such as reconcile, suspend and resume guarded by Kubernetes RBAC.

### Workload Dashboard

Monitor all workloads managed by Flux with dedicated dashboards. Trace the delivery pipeline from source to running pods, drill into pod and container status, and trigger actions such as reconcile, rollout restart and log viewing guarded by Kubernetes RBAC.

### Log Viewer

Tail, filter and follow pod logs in a dedicated full-featured viewer. The builtin parser detects log levels and groups stack traces for popular logging frameworks across Go, Java, .NET, Python, Ruby, PHP and JSON formats. Search with exclusions, filter by pod, container and level, and download raw logs.

### Event Viewer

Watch Kubernetes events issued by the Flux controllers in real-time across your cluster. Filter events by resource name with wildcard and exclusion support, by namespace, kind and severity to quickly spot reconciliation failures as they happen.

### Advanced Search

Find any resource instantly with the advanced search functionality. Filter by type, namespace, status, or name to quickly locate specific Kustomizations, HelmReleases, or source repositories.

### Favorites

Pin your most important resources for quick access. The favorites view provides an at-a-glance status of the resources you care about most, perfect for monitoring critical production deployments.

### GitOps Graph

Visualize your app delivery pipeline in an interactive graph. See real-time status updates as resources reconcile, trace dependencies from sources to deployments, and instantly spot issues as they occur.

### Reconciliation History

Track changes over time with the reconciliation history view. See when resources were updated, what changed, and identify patterns in your deployment pipeline.

### Single Sign-On

Secure access with your organization identity provider. The builtin OpenID Connect support allows seamless SSO integration, leveraging Kubernetes RBAC for fine-grained access control.

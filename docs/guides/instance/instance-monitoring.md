---
title: Flux Monitoring and Reporting
description: Flux Operator monitoring guide for Kubernetes and OpenShift clusters
---

# Flux Monitoring and Reporting

The Flux Operator supervises the Flux controllers and provides a unified view
of all the Flux resources that define the GitOps workflows for the target cluster.
The operator generates reports, emits events, and exports Prometheus metrics
to help with monitoring and troubleshooting Flux.

## Flux Status Reporting

The Flux Operator automatically generates a report that reflects the observed state of the Flux
installation. The report provides information about the installed components and their readiness,
the Flux distribution details, reconcilers statistics, cluster sync status and more.

The report is generated as a custom resource of kind `FluxReport`, named `flux`,
located in the same namespace where the operator is running.

!!! tip "Flux installation method"

    The report is available no matter the tool used to install Flux,
    be it the `flux` CLI, Terraform, Helm or the Flux Operator itself.
    For the report to be accurate, the operator must be running
    in the same namespace where the Flux controllers are deployed.

To view the report in YAML format run:

```shell
kubectl -n flux-system get fluxreport/flux -o yaml
```

The operator updates the report at regular intervals, by default every five minutes.
To manually trigger the reconciliation of the report, run:

```shell
kubectl -n flux-system annotate --overwrite fluxreport/flux \
 reconcile.fluxcd.io/requestedAt="$(date +%s)"
```

Find more information about the reporting features
in the [Flux Report API documentation](fluxreport.md).

## Flux Instance Events

The Flux Operator emits events to the Kubernetes API server to report on the status of the Flux
instance. The events are useful to monitor the Flux lifecycle and troubleshoot upgrade issues.

To list the events related to the Flux instance, run:

```shell
kubectl -n flux-system events --for fluxinstance/flux
```

The Flux Operator integrates with notification-controller. To receive notifications with the
events issued by the operator, you can configure alerting as follows:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: slack-bot
  namespace: flux-system
spec:
  type: slack
  channel: general
  address: https://slack.com/api/chat.postMessage
  secretRef:
    name: slack-bot-token
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  providerRef:
    name: slack-bot
  eventSeverity: info
  eventSources:
    - kind: FluxInstance
      name: flux
```

Besides Slack, the notification-controller supports other providers like Microsoft Teams, Datadog, Grafana, etc.,
for more information see the [alert provider documentation](https://fluxcd.io/flux/components/notification/providers/).

## Prometheus Metrics

The Flux Operator exports metrics in the Prometheus format for monitoring
and alerting purposes. The metrics are exposed inside the cluster by the
`flux-operator` Kubernetes Service on the `8080` port.

On clusters where the Prometheus Operator is installed, the metrics can be scraped
by creating a `ServiceMonitor` resource as follows:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: flux-operator
  namespace: flux-system
  labels:
    release: kube-prometheus-stack
spec:
  namespaceSelector:
    matchNames:
      - flux-system
  selector:
    matchLabels:
      app.kubernetes.io/name: flux-operator
  endpoints:
    - targetPort: 8080
      path: /metrics
      interval: 30s
```

It is recommended to change the reporting interval to `30s` when using the Prometheus metrics
exported by the operator:

```shell
helm upgrade flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system \
  --set reporting.interval=30s
```

!!! tip "Helm Chart"

    The Flux Operator [Helm chart](install.md#helm) includes a `ServiceMonitor` resource that can
    be enabled by setting the `serviceMonitor.create` value to `true`.

On clusters with Prometheus auto-discovery enabled, the metrics are automatically scraped
from the `flux-operator` pods that have the `prometheus.io/scrape: "true"` annotation.

### Flux Instance Metrics

The Flux Operator exports metrics for the [FluxInstance](fluxinstance.md) resource.
These metrics are refreshed every time the operator reconciles the instance.

Metrics:

```text
flux_instance_info{uid, kind, name, exported_namespace, ready, suspended, registry, revision}
```

Labels:

- `uid`: The Kubernetes unique identifier of the resource.
- `kind`: The kind of the resource (e.g. `FluxInstance`).
- `name`: The name of the resource (e.g. `flux`).
- `exported_namespace`: The namespace where the resource is deployed (e.g. `flux-system`).
- `ready`: The readiness status of the resource (e.g. `True`, `False` or `Unkown`).
- `reason`: The reason for the readiness status (e.g. `Progressing`, `BuildFailed`, `HealthCheckFailed`, etc.).
- `suspended`: The suspended status of the resource (e.g. `True` or `False`).
- `registry`: The container registry used by the instance (e.g. `ghcr.io/fluxcd`).
- `revision`: The Flux revision installed by the instance (e.g. `v2.3.0@sha256:75aa209c6a...`).

### Flux ResourceSet Metrics

The Flux Operator exports metrics for the [ResourceSet APIs](resourcesets/introduction.md)
that can be used to monitor the reconciliation status.

Metrics:

```text
flux_resourceset_info{uid, kind, name, exported_namespace, resources, ready, suspended, revision}
flux_resourcesetinputprovider_info{uid, kind, name, exported_namespace, ready, suspended, url}
```

Labels:

- `uid`: The Kubernetes unique identifier of the resource.
- `kind`: The kind of the resource (e.g. `ResourceSet`).
- `name`: The name of the resource (e.g. `podinfo`).
- `exported_namespace`: The namespace where the resource is deployed (e.g. `apps`).
- `ready`: The readiness status of the resource (e.g. `True`, `False` or `Unkown`).
- `reason`: The reason for the readiness status (e.g. `ReconciliationSucceeded`, `BuildFailed`, `HealthCheckFailed`, etc.).
- `suspended`: The suspended status of the resource (e.g. `True` or `False`).

### Flux Resource Metrics

The Flux Operator exports metrics for all Flux resources found in the cluster.
These metrics are refreshed at the same time with the update of the [FluxReport](fluxreport.md).

Metrics:

```text
flux_resource_info{uid, kind, name, exported_namespace, ready, suspended, ...}
```

Common labels:

- `uid`: The Kubernetes unique identifier of the resource.
- `kind`: The kind of the resource (e.g. `GitRepository`, `Kustomization`, etc.).
- `name`: The name of the resource (e.g. `flux-system`).
- `exported_namespace`: The namespace of the resource (e.g. `flux-system`).
- `ready`: The readiness status of the resource (e.g. `True`, `False` or `Unkown`).
- `reason`: The reason for the readiness status (e.g. `Progressing`, `BuildFailed`, `HealthCheckFailed`, etc.).
- `suspended`: The suspended status of the resource (e.g. `True` or `False`).

Specific labels per resource kind:

| Resource Kind         | Labels                            |
|-----------------------|-----------------------------------|
| Kustomization         | `revision`, `source_name`, `path` |
| GitRepository         | `revision`, `url`, `ref`          |
| OCIRepository         | `revision`, `url`, `ref`          |
| Bucket                | `revision`, `url`, `ref`          |
| HelmRelease           | `revision`, `source_name`         |
| HelmChart             | `revision`, `source_name`         |
| HelmRepository        | `revision`, `url`                 |
| Receiver              | `url`                             |
| ImageRepository       | `url`                             |
| ImagePolicy           | `source_name`                     |
| ImageUpdateAutomation | `source_name`                     |

### Controller Runtime Metrics

The Flux Operator exports Kubernetes
[controller runtime metrics](https://book.kubebuilder.io/reference/metrics-reference)
and Go runtime metrics.

Relevant metrics for troubleshooting:

- `controller_runtime_reconcile_errors_total{controller}`: Total number of reconciliation errors per controller.
- `rest_client_requests_total{code, method}`: Number of Kubernetes API requests, partitioned by status code and method.
- `go_memstats_alloc_bytes`: Number of bytes allocated and still in use.
- `go_goroutines`: Number of goroutines that currently exist.
- `workqueue_longest_running_processor_seconds`: Longest time a workqueue item has been processed.

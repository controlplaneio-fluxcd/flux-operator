---
title: Flux MCP Server Tools
description: MCP tools for interacting with Kubernetes clusters managed by FluxCD
---

# Flux MCP Server Tools

The Flux Model Context Protocol (MCP) Server provides a comprehensive set of tools
that enable AI assistants to interact with Kubernetes clusters managed by
[Flux Operator](https://github.com/controlplaneio-fluxcd/flux-operator).

## Reporting Tools

These tools gather information from the cluster without making any changes to the system state.

### get_flux_instance

Retrieves detailed information about the Flux installation.

**Parameters:** None

**Output:**

The tool returns comprehensive details about the Flux instance configuration, including
the distribution version information, component status and health, cluster sync statistics.

### get_kubernetes_resources

Retrieves Kubernetes resources from the cluster, including Flux custom resources, their status, and associated events.

**Parameters:**

- `apiVersion` (required): The API version of the resource(s)
- `kind` (required): The kind of the resource(s)
- `name` (optional): The name of a specific resource
- `namespace` (optional): The namespace to query
- `selector` (optional): Label selector in the format `key1=value1,key2=value2`
- `limit` (optional): Maximum number of resources to return
- `fields` (optional): List of kubectl JSONPath expressions to include in the result, e.g. `spec.chart.spec.version`, `status.conditions[?(@.type=="Ready")].message` or `status.inventory`

**Output:**

Returns the requested resources in YAML format, including:
- Resource specifications
- Status conditions
- Related events (`status.events`)
- HelmRelease inventory (`status.inventory`)
- Metadata including Flux source references

When `fields` is set, each resource is reduced to its `apiVersion`, `kind`, `metadata.name`
and `metadata.namespace`, along with the values selected by the expressions. For example, the fields
`["spec.chart.spec.version", "status.conditions[?(@.type==\"Ready\")].message"]` return:

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: podinfo
  namespace: apps
spec.chart.spec.version: 6.x
status.conditions[?(@.type=="Ready")].message: Helm upgrade succeeded for release apps/podinfo.v3 with chart podinfo@6.9.0
```

Use `fields` to reduce the size of the result when
listing many resources or when only a few fields are relevant.

### get_kubernetes_logs

Retrieves timestamped logs for workloads, allowing AI Agents to analyze
application behavior and troubleshoot issues.

**Parameters:**

- `kind` (optional): Resource kind. Supported values are `Pod` (the default), `Deployment`,
  `StatefulSet`, `DaemonSet`, `CronJob`, and `Job` (case-insensitive).
- `name` (required): The name of the pod or workload.
- `namespace` (required): The namespace of the pod or workload.
- `container` (optional): A regular container name. When omitted, logs are read from all
  `spec.containers`; init and ephemeral containers are excluded.
- `limit` (optional): Maximum number of merged log entries to return (default: 100).
- `previous` (optional): Read logs from previously terminated container instances (default: false).

For workloads, the tool resolves owned pods, orders them newest-first, and concurrently reads every
selected pod and container stream. When multiple pods and containers are selected, log lines use
the `<pod> <container> <timestamp> <message>` format, where the timestamp is RFC 3339 at seconds precision.

**Output:**

Returns YAML with `kind`, `name`, `namespace`, selected `pods`, de-duplicated `containers`,
`podsTotal` (matches before the pod cap), `podsStreamed` (pods with a successful stream), `tagged`,
`truncated` (a pod or stream cap dropped targets), and the merged `logs` payload.

For example, `kind: Deployment`, `name: backend`, `namespace: apps-staging` and `limit: 4` return:

```yaml
kind: Deployment
name: backend
namespace: apps-staging
pods:
- backend-567b7494c-ddddm
- backend-567b7494c-2vxhm
containers:
- podinfod
podsTotal: 2
podsStreamed: 2
tagged: true
truncated: false
logs: |
  backend-567b7494c-2vxhm 2026-08-28T15:35:51Z {"level":"info","ts":"2026-08-28T15:35:51.199Z","caller":"podinfo/main.go:170","msg":"Starting podinfo","version":"6.14.0","revision":"a30fa3224289a3f3e413157104dee8844e329926","port":"9898"}
  backend-567b7494c-2vxhm 2026-08-28T15:35:51Z {"level":"info","ts":"2026-08-28T15:35:51.199Z","caller":"http/server.go:273","msg":"Starting HTTP Server.","addr":":9898"}
  backend-567b7494c-ddddm 2026-08-28T15:36:05Z {"level":"info","ts":"2026-08-28T15:36:05.911Z","caller":"podinfo/main.go:170","msg":"Starting podinfo","version":"6.14.0","revision":"a30fa3224289a3f3e413157104dee8844e329926","port":"9898"}
  backend-567b7494c-ddddm 2026-08-28T15:36:05Z {"level":"info","ts":"2026-08-28T15:36:05.911Z","caller":"http/server.go:273","msg":"Starting HTTP Server.","addr":":9898"}
```

When a single container has no output, `logs` contains `no logs found for container <name>`.

When a workload has no pods, `logs` contains `no pods found for <kind> <namespace>/<name>` and
`podsTotal` is `0`.

### get_kubernetes_events

Retrieves Kubernetes events for workloads and other Kubernetes objects, including Pods,
Deployments, PersistentVolumeClaims, Nodes, and Flux resources. For a `Deployment`, `StatefulSet`,
`DaemonSet`, `CronJob` or `Job`, the events of the workload, of the ReplicaSets or Jobs it owns,
and of its newest 10 pods are returned together.

**Parameters:**

- `apiVersion` (optional): Exact API version of the involved object, such as `v1`, `apps/v1`,
  or `helm.toolkit.fluxcd.io/v2`.
- `kind` (optional): Exact, case-sensitive kind of the involved object. Workload kinds are
  resolved to their owned objects when `name` and `namespace` are set.
- `name` (optional): Exact name of the involved object.
- `namespace` (optional): Namespace of the involved objects. When omitted, events are listed
  across all namespaces.
- `type` (optional): Event type, either `Normal` or `Warning`.
- `since` (optional): Only return events newer than this Go duration, such as `10m` or `1h`.
- `grep` (optional): Case-insensitive RE2 expression matched against the event reason, message,
  and involved object rendered as `Kind/namespace/name` (`Kind/name` when cluster-scoped).
- `limit` (optional): Maximum number of events to return after filtering and sorting
  (default: 100).

**Output:**

Returns events newest-first in YAML, with the matched total before the requested limit and a
`truncated` indicator when the limit drops entries, when the pod cap drops workload pods, or when
more than 5,000 events matched the selectors; the cap inspects the first 5,000 in API order, so
narrow the request with `namespace`, `kind` or `type` when `truncated` is set. The `events`
value contains one event per line with space-separated `<time> <type> <reason> <object> [x<count>]
<message>` columns. The object is rendered as `Kind/namespace/name`, or `Kind/name` for a
cluster-scoped object, and the count is omitted when it is one.

For example, `namespace: kube-system`, `kind: Pod`, `type: Warning`, `since: 1h` and `limit: 3` return:

```yaml
namespace: kube-system
total: 6
truncated: true
events: |
  2026-08-28T15:32:21Z Warning Unhealthy Pod/kube-system/coredns-589f44dc88-244lp Readiness probe failed: Get "http://10.244.0.4:8181/ready": dial tcp 10.244.0.4:8181: connect: connection refused
  2026-08-28T15:32:21Z Warning Unhealthy Pod/kube-system/coredns-589f44dc88-km8w5 Readiness probe failed: Get "http://10.244.0.2:8181/ready": dial tcp 10.244.0.2:8181: connect: connection refused
  2026-08-28T15:32:09Z Warning FailedScheduling Pod/kube-system/coredns-589f44dc88-244lp 0/1 nodes are available: 1 node(s) had untolerated taint(s). no new claims to deallocate, preemption: 0/1 nodes are available: 1 Preemption is not helpful for scheduling.
```

When no events match, the tool returns `No events found` as a normal text result, unless the
cap was reached, in which case the YAML result is returned with `truncated: true`.

### get_kubernetes_metrics

Retrieves CPU and Memory usage for Kubernetes pods, allowing AI assistants to monitor resource consumption and performance.
This tool depends on the Kubernetes metrics-server being installed in the cluster.

**Parameters:**

- `pod_name` (optional): The name of the pod, when not specified all pods are selected.
- `pod_namespace` (required): The namespace of the pods.
- `pod_selector` (optional): Label selector in the format `key1=value1,key2=value2`
- `limit` (optional): Maximum number of metrics to return (default: 100)

**Output:**

Returns the metrics for the specified pods, including CPU and Memory for each container, in YAML format.

### get_kubernetes_api_versions

Retrieves the Kubernetes CRDs registered on the cluster and returns the preferred apiVersion for each kind.

**Parameters:** None

**Output:**

Returns a mapping of Kubernetes resource kinds to their preferred API versions,
which is essential for crafting valid API calls.

## Multi-Cluster Tools

These tools facilitate interaction with multiple Kubernetes clusters, enabling cross-cluster comparisons and operations.

### get_kubeconfig_contexts

Retrieves the available Kubernetes cluster contexts from the kubeconfig.

**Parameters:** None

**Output:**

List of available Kubernetes contexts with their associated cluster name.

### set_kubeconfig_context

Switches the current session to use a specific Kubernetes cluster context, without modifying the kubeconfig file.

**Parameters:**

- `name` (required): The name of the context to set

**Output:**

Confirmation message indicating the context has been switched.

## Reconciliation Tool

This tool triggers the reconciliation of Flux resources, causing Flux to synchronize the desired state with the current state.

### reconcile_flux_resource

Triggers an on-demand reconciliation of a Flux resource and can optionally reconcile its source first.

**Parameters:**

- `apiVersion` (optional): The API version of the Flux resource, resolved from the kind when omitted
- `kind` (required): The kind of the Flux resource
- `name` (required): The name of the Flux resource
- `namespace` (required): The namespace of the Flux resource
- `with_source` (optional): Whether to reconcile the referenced source first (default: false)

Note that HelmRelease and ResourceSetInputProvider receive a forced reconciliation.

**Output:**

Confirmation that reconciliation was triggered, whether a referenced source was reconciled or skipped,
and instructions for verifying the reconciliation status.

## Suspend/Resume Tools

These tools allow for pausing and resuming the reconciliation of Flux resources.

### suspend_flux_reconciliation

Suspends the reconciliation of a Flux resource.

**Parameters:**

- `apiVersion` (optional): The API version of the Flux resource, resolved from the kind when omitted
- `kind` (required): The kind of the resource
- `name` (required): The name of the resource
- `namespace` (required): The namespace of the resource

**Output:**

Confirmation message indicating the resource has been suspended.

### resume_flux_reconciliation

Resumes the reconciliation of a previously suspended Flux resource.

**Parameters:**

- `apiVersion` (optional): The API version of the Flux resource, resolved from the kind when omitted
- `kind` (required): The kind of the resource
- `name` (required): The name of the resource
- `namespace` (required): The namespace of the resource

**Output:**

Confirmation message indicating the resource has been resumed.

## Diff Tool

This tool previews Kubernetes changes without mutating the cluster.

### diff_kubernetes_manifest

Diffs a multi-document YAML manifest against the cluster using server-side apply dry-run with
the Flux controller's field manager. Use it to preview a direct apply or the effect of a GitOps
commit before pushing it to the repo. The owner transforms are applied before the dry-run:
Kustomization build options and `postBuild` substitutions, ResourceSet `copyFrom`, HelmRelease
`postRenderers` and release metadata, and the `commonMetadata` and owner labels of all kinds.

**Parameters:**

When `yaml_path` is advertised, exactly one of `yaml_content` or `yaml_path` must be specified;
otherwise, `yaml_content` must be specified.

- `yaml_content` (optional): The complete multi-document YAML build output to diff
- `yaml_path` (optional): The absolute path to a local multi-document YAML build output file;
  mutually exclusive with `yaml_content`, advertised only when the server runs locally over the
  `stdio` transport, with a 64 MiB file size limit
- `flux_object` (optional): The YAML definition of the Kustomization, ResourceSet, or HelmRelease
  that applies the manifest, as it will exist after the change
- `owner_kind`, `owner_name`, `owner_namespace` (optional): A reference to an existing owner;
  all three parameters must be specified together

**Output:**

Each manifest object is reported as `create`, `recreate`, `update`, `unchanged`, `skipped`, or
`error`; inventory objects absent from the manifest are reported as `delete`. Updates include an
RFC 6902 JSON patch in YAML form, with Secret values masked. The final summary line counts every
state. For example:

```text
Diff for Kustomization/apps-staging/backend (field manager: kustomize-controller, prune: enabled)

ConfigMap/apps-staging/backend-7f2k9c4mbt create
Deployment/apps-staging/backend update
- op: replace
  path: /spec/template/spec/containers/0/envFrom/0/configMapRef/name
  value: backend-7f2k9c4mbt
Service/apps-staging/backend unchanged

Not in the manifest (pruned if the manifest is complete):
ConfigMap/apps-staging/backend-m759gh88kd delete

Summary: 1 create, 1 update, 1 unchanged, 1 delete
```

**Limitations:**

- Owners targeting remote clusters are not supported.
- Kustomization `spec.components` is not applied.
- ResourceSet `checksumFrom` and `convertKubeConfigFrom` are not resolved.
- SOPS-encrypted objects are compared by key set only.
- Kustomization `namePrefix` and `nameSuffix` are added to the names in the build output, while
  kustomize-controller replaces the ones set in the source `kustomization.yaml`; when both are set,
  the diff reports different names than the cluster.
- Kustomization `buildMetadata` is not applied; the labels and annotations it adds are ignored.
- Managed-fields cleanup is not simulated.

## Apply Tool

This tool allows creating or updating Kubernetes resources in the cluster.
If the resources already exist and are managed by Flux, the tool will error out unless
explicitly told to overwrite them.

### apply_kubernetes_manifest

Applies a YAML manifest on the cluster using Kubernetes server-side apply.

**Parameters:**

- `yaml_content` (required): The multi-doc YAML content
- `overwrite` (optional): Whether to overwrite resources managed by Flux (default: false)

**Output:**

The list of applied resources in the format `kind/namespace/name [created|updated|unchanged]`.

## Deletion Tool

This tool enables the removal of resources from your cluster.

### delete_kubernetes_resource

Deletes a Kubernetes resource from the cluster.

**Parameters:**

- `apiVersion` (required): The API version of the resource
- `kind` (required): The kind of the resource
- `name` (required): The name of the resource
- `namespace` (required for namespaced resources): The namespace of the resource

**Output:**

Confirmation message indicating the resource has been deleted.

## Install Tool

This tool enables automated installation of Flux Operator and Flux instances on Kubernetes clusters.

### install_flux_instance

Installs Flux Operator and a Flux instance on the cluster from a manifest URL.

**Parameters:**

- `instance_url` (required): The URL pointing to the Flux Instance manifest file (supports HTTPS and OCI URLs)
- `timeout` (optional): The installation timeout duration (default: 5m)

**Output:**

Returns a detailed installation log including deployed resources with their change status.

**Installation Steps:**

The tool performs the following operations:

1. Downloads the Flux instance manifest from the provided URL
2. Downloads the Flux Operator manifests from the distribution artifact
3. Installs or upgrades the Flux Operator in the `flux-system` namespace
4. Installs or upgrades the Flux instance according to the manifest configuration
5. Waits for the Flux instance to become ready
6. Configures automatic updates for the Flux Operator

**Example URLs:**

- OCI Artifact: `oci://ghcr.io/org/manifests:latest#clusters/dev/flux-system/flux-instance.yaml`
- GitHub Gist: `https://gist.github.com/user/id#file-flux-instance-yaml`
- GitHub Repo: `https://github.com/org/repo/blob/main/clusters/dev/flux-system/flux-instance.yaml`
- GitLab Repo: `https://gitlab.com/org/proj/-/blob/main/clusters/dev/flux-system/flux-instance.yaml`

## Documentation Tool

This tool provides access to Flux documentation in concise and complete formats.

### search_flux_docs

Searches the Flux documentation for specific information. By default, the tool returns concise Flux reference documentation optimized for low agent context usage. Use the complete format only when the full upstream API documentation is needed.

**Parameters:**

- `query` (required): The search query
- `limit` (optional): Maximum number of results to return (default: 1)
- `format` (optional): Documentation format, one of `concise` or `complete` (default: `concise`)

**Output:**

Relevant Flux documentation that matches the search query. The `concise` format returns compact reference docs, while the `complete` format returns full upstream API docs.

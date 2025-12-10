---
title: Using ResourceSets for Time-Based Delivery
description: How to define GitOps deployment windows with Flux Operator
---

# Using ResourceSets for Time-Based Delivery

In highly regulated industries, deploying software changes requires strict adherence to compliance
frameworks and operational policies. These organizations must demonstrate control over when changes
are deployed to production systems, often requiring:

- **Change Advisory Board (CAB) approval windows** - Deployments only during pre-approved time slots
- **Business continuity requirements** - No deployments during peak business hours or critical operations
- **Compliance auditing** - Detailed records of when and why deployments occurred
- **Risk management** - Controlled rollout windows to minimize business impact
- **Operational readiness** - Ensuring sufficient staff coverage during deployment windows

The Flux Operator addresses these requirements through time-based reconciliation schedules, providing
organizations with the governance controls they need while maintaining the benefits of GitOps automation.

## How ResourceSets and Input Providers Work

Before diving into the configuration, it's important to understand how The Flux Operator APIs
work together to enable controlled deployments.

The [ResourceSet](resourceset.md) API allows you to define a set of Flux resources
for deploying an application, while the [ResourceSetInputProvider](resourcesetinputprovider.md) API
is used to provide inputs to the `ResourceSet`, such as Git commit SHA and branch name or tag name,
that determine what version of the application should be deployed.

Instead of using a Flux `GitRepository` and `Kustomization` directly, we'll generate these
resources dynamically with a `ResourceSet`. To control when Flux pulls changes from Git
we'll pin the `GitRepository` to a specific commit SHA, the `ResourceSetInputProvider`
will be responsible for fetching the latest commit SHA from a Git branch or tag,
at the defined reconciliation schedule.

## GitOps Workflow

- **Define a ResourceSetInputProvider**: This provider will scan a Git branch or tag
   for changes and export the commit SHA as an input.
- **Configure schedule**: The provider will have a reconciliation schedule
   that defines when it should check for changes in the Git repository.
- **Define a ResourceSet**: The ResourceSet will use the inputs from the provider
   to create a `GitRepository` and `Kustomization` that deploys the application
   at the specified commit SHA.

### ResourceSetInputProvider Definition

Assuming the Kubernetes deployment manifests for an application are stored in a Git repository,
you can define a input provider that scans a branch for changes
and exports the commit SHA:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: my-app-main
  namespace: apps
  labels:
    app.kubernetes.io/name: my-app
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
    fluxcd.controlplane.io/reconcileTimeout: "1m"
spec:
  schedule:
    - cron: "0 8 * * 1-5"
      timeZone: "Europe/London"
      window: 8h
  type: GitHubBranch # or GitLabBranch / AzureDevOpsBranch
  url: https://github.com/my-org/my-app
  secretRef:
    name: gh-app-auth
  filter:
    includeBranch: "^main$"
  defaultValues:
    env: "production"
```

For when Git tags are used to version the application, you can define an input provider
that scans the Git tags and exports the latest tag according to a semantic versioning:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: my-app-release
  namespace: apps
  labels:
    app.kubernetes.io/name: my-app
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
    fluxcd.controlplane.io/reconcileTimeout: "1m"
spec:
  schedule:
    - cron: "0 8 * * 1-5"
      timeZone: "Europe/London"
      window: 8h
  type: GitHubTag # or GitLabTag / AzureDevOpsTag
  url: https://github.com/my-org/my-app
  secretRef:
    name: gh-auth
  filter:
    semver: ">=1.0.0"
    limit: 1
```

### ResourceSet Definition

The exported inputs can then be used in a `ResourceSet` to deploy the application
using the commit SHA from the input provider:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: my-app
  namespace: apps
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      selector:
        matchLabels:
          app.kubernetes.io/name: my-app
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: my-app
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 12h
        url: https://github.com/my-org/my-app
        ref:
          commit: << inputs.sha >>
        secretRef:
          name: gh-auth
        sparseCheckout:
          - deploy
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: my-app
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 30m
        retryInterval: 5m
        prune: true
        wait: true
        timeout: 5m
        sourceRef:
          kind: GitRepository
          name: my-app
        path: deploy/<< inputs.env >>
```

When the `ResourceSetInputProvider` runs according to its schedule, if it finds a new commit,
the `ResourceSet` will be automatically updated with the new commit SHA which will trigger
an application deployment for the new version.

## Helm Release Workflow

For applications packaged with Helm, you can use a similar approach to trigger a Helm release upgrade
in a controlled manner when a new chart version is available.
For this to work, the Helm chart must be stored in a container registry as an OCI artifact.

Example `ResourceSetInputProvider` that scans an OCI repository and exports the latest stable version as an input:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
   name: podinfo-release
   namespace: apps
   labels:
      app.kubernetes.io/name: podinfo
   annotations:
      fluxcd.controlplane.io/reconcileEvery: "10m"
      fluxcd.controlplane.io/reconcileTimeout: "1m"
spec:
   schedule:
      - cron: "0 12 * * 1-5"
        timeZone: "UTC"
   type: OCIArtifactTag
   url: oci://ghcr.io/stefanprodan/charts/podinfo
   filter:
      semver: ">=1.0.0"
      limit: 1
```

Example `ResourceSet` that deploys a Flux HelmRelease using
the artifact tag exported by the input provider as the latest chart version:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: podinfo
  namespace: apps
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      selector:
        matchLabels:
          app.kubernetes.io/name: podinfo
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: podinfo
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 10m
        url: oci://ghcr.io/stefanprodan/charts/podinfo
        ref:
          tag: << inputs.tag >>
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: podinfo
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 30m
        releaseName: podinfo
        chartRef:
          kind: OCIRepository
          name: podinfo
        values:
          replicaCount: 2
```

!!! tip "OCI Artifacts Support"

    Note that Flux Operator supports OIDC-based authentication for container registries such as Amazon ECR, Azure ACR and Google GAR.
    For more details, see the [ResourceSetInputProvider API reference](resourcesetinputprovider.md#secret-less).

## Scheduling Configuration

The `.spec.schedule` field in the `ResourceSetInputProvider` allows you to define when the input provider
should run to check for changes in source repositories.

### Schedule Definition

The schedule is defined as a list of cron expressions, each with an optional time zone and window.

Example:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
spec:
  schedule:
    # Every day-of-week from Monday through Thursday
    # between 10:00 to 16:00
    - cron: "0 10 * * 1-4"
      timeZone: "America/New_York"
      window: "6h"
```

The `cron` field accepts standard cron expressions with five fields:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

Use [crontab.guru](https://crontab.guru/) to help generate and validate cron expressions.

The `timeZone` field specifies the time zone for interpreting the cron schedule
using [IANA time zone names](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).
If not specified, the time zone defaults to `UTC`.

The `window` field defines the duration during which reconciliations are allowed to run after the scheduled time.
The format is a Go duration string, e.g. `30m`, `1h`, `2h30m`.
Must be either `0s` (no window) or at least twice the reconciliation timeout `4m`.

### Schedule Window

When a non-zero window is specified, reconciliation is allowed throughout the entire window duration:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
   annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  schedule:
    - cron: "0 8 * * 1-5"
      timeZone: "UTC"
      window: "8h"
```

In this case:

- At creation time, the input provider will not execute immediately, but will wait for the next scheduled time.
- The input provider will start reconciling at 08:00 UTC every weekday (Monday to Friday).
- The reconciliation will continue until 16:00 UTC, every 10 minutes, as specified by the `reconcileEvery` annotation.
- Any changes to the input provider object during this window will be reconciled immediately.

### Zero-Duration Window

When the window is omitted or set to `0s`, flux-operator makes the best effort to reconcile
at the exact scheduled time:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
   annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  schedule:
    - cron: "0 8 * * 1-5"
      timeZone: "UTC"
      window: "0s"
```

In this case:

- At creation time, the input provider will execute immediately ignoring the schedule.
- The input provider will reconcile at 08:00 UTC every weekday (Monday to Friday).
- Any changes to the input provider object will be reconciled immediately, even outside the scheduled time.

### Multiple Schedules

The schedule can contain multiple cron expressions, allowing for complex scheduling scenarios:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
spec:
  schedule:
    # Every day-of-week from Monday through Thursday
    # between 10:00 to 16:00 NY time
    - cron: "0 10 * * 1-4"
      timeZone: "America/New_York"
      window: "6h"
    # Every Friday from 10:00 to 13:00 UK time
    - cron: "0 10 * * 5"
      timeZone: "Europe/London"
      window: "3h"
```

In this case:

- The input provider reconciles if **any** schedule matches the current time.
- The next scheduled time is determined by the **earliest** upcoming schedule.
- Each schedule operates independently with its own time zone and window.

## Command-Line Operations

The `flux-operator` CLI can be used to perform manual operations
such as forcing a reconciliation, checking the status of the input provider or disabling it.

To force a reconciliation outside the defined schedule:

```sh
flux-operator reconcile rsip my-app-main --namespace apps --force
```

To check the status of input providers including their next schedule time:

```sh
flux-operator get rsip --all-namespaces
```

To suspend an input provider and prevent it from reconciling:

```sh
flux-operator suspend rsip my-app-main --namespace apps
```

To resume a suspended input provider:

```sh
flux-operator resume rsip my-app-main --namespace apps
```

See the [Flux Operator CLI documentation](cli.md) for more details on how to use the CLI.

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)

---
title: Flux Sharding Configuration
description: Flux Operator horizontal scaling and sharding guide
---

# Flux Sharding Configuration

The Flux Operator supports sharding the workload across multiple instances
of Flux controllers allowing you to horizontally scale the reconciliation
of resources.

This feature is useful when you have a large number of resources to manage
and want to distribute the workload across multiple controller replicas. Another use
case is to isolate the resources reconciliation for different teams and environments.

## Sharding Configuration

To enable sharding, add the following configuration to the `FluxInstance`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
  cluster:
    size: large
  sharding:
    key: "sharding.fluxcd.io/key"
    shards:
      - "shard1"
      - "shard2"
```

The `.spec.sharding.key` field specifies the sharding key label to use for the Flux controllers
and the `.spec.sharding.shards` field specifies the list of shards.

Based on the above configuration, the Flux Operator will create a separate set of controllers
for each shard and will configure the controllers to reconcile only the resources that have the
sharding key label set to the shard name.

To list the Flux controllers and their shards:

```console
$ kubectl -n flux-system get deploy -l app.kubernetes.io/part-of=flux
NAME                          READY   UP-TO-DATE   AVAILABLE   AGE
helm-controller               1/1     1            1           77s
helm-controller-shard1        1/1     1            1           77s
helm-controller-shard2        1/1     1            1           77s
kustomize-controller          1/1     1            1           77s
kustomize-controller-shard1   1/1     1            1           77s
kustomize-controller-shard2   1/1     1            1           77s
notification-controller       1/1     1            1           77s
source-controller             1/1     1            1           77s
source-controller-shard1      1/1     1            1           77s
source-controller-shard2      1/1     1            1           77s
```

Note that only the `source-controller`, `kustomize-controller` and `helm-controller` controllers
support sharding.

It is recommended to use the main controller instances to reconcile the cluster add-ons and
the sharded controllers to reconcile the application workloads belonging to tenants.

## Sharding with Persistent Storage

Enabling persistent storage for source-controller can speed up startup time and
reduce the network traffic after a restart, as the controller will not need to
re-download all the artifacts from the source repositories.

To enable persistent storage for the source-controller shards,
you can add the following configuration to the `FluxInstance`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
  cluster:
    size: large
  storage:
    class: "standard"
    size: "10Gi"
  sharding:
    key: "sharding.fluxcd.io/key"
    shards:
      - "shard1"
      - "shard2"
    storage: "persistent"
```

The operator will create a `PersistentVolumeClaim` for each shard including the main source-controller instance:

```console
$ kubectl -n flux-system get pvc
NAME                          STATUS
source-controller             Bound 
source-controller-shard1      Bound
source-controller-shard2      Bound
```

## Distributing Resources Across Shards

To assign a group of Flux resources to a particular shard, add the sharding key label to the resources,
using the shard name as the value.

Note that the Flux Kustomizations and HelmReleases must have the sharding key label set to the same shard name
as their source GitRepository, OCIRepository, HelmRepository or HelmChart.

### Examples

To assign a Flux Kustomization and its GitRepository source to the `shard1` controllers:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard1
spec:
  interval: 10m
  url: https://github.com/stefanprodan/podinfo
  ref:
    semver: 6.x
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard1
spec:
  interval: 10m
  targetNamespace: default
  sourceRef:
    kind: GitRepository
    name: podinfo
  path: ./kustomize
  prune: true
```

To assign a Flux HelmRelease and its OCIRepository source to the `shard2` controllers:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard2
spec:
  interval: 10m
  url: oci://ghcr.io/stefanprodan/charts/podinfo
  layerSelector:
    mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
    operation: copy
  ref:
    semver: ">6.0.0"
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard2
spec:
  interval: 10m
  releaseName: podinfo
  chartRef:
    kind: OCIRepository
    name: podinfo
```

To assign a Flux HelmRelease and its HelmChart & HelmRepository source to the `shard2` controllers:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard2
spec:
  interval: 1h
  url: https://stefanprodan.github.io/podinfo
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmChart
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard2
spec:
  interval: 30m
  chart: podinfo
  version: 6.x
  sourceRef:
    kind: HelmRepository
    name: podinfo
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: podinfo
  namespace: default
  labels:
    sharding.fluxcd.io/key: shard2
spec:
  interval: 10m
  releaseName: podinfo
  chartRef:
    kind: HelmChart
    name: podinfo
```

To list all the resources assigned to a particular shard, you can pass the label selector to the `flux` CLI:

```console
$ flux get all -A -l sharding.fluxcd.io/key=shard2

NAME                  	REVISION       	SUSPENDED	READY	MESSAGE                                     
helmrepository/podinfo	sha256:3dfe15d8	False    	True 	stored artifact: revision 'sha256:3dfe15d8'	

NAME             	REVISION	SUSPENDED	READY	MESSAGE                                     
helmchart/podinfo	6.7.0   	False    	True 	pulled 'podinfo' chart with version '6.7.0'	

NAME               	REVISION	SUSPENDED	READY	MESSAGE                                                                             
helmrelease/podinfo	6.7.0   	False    	True 	Helm install succeeded for release podinfo-helm/podinfo.v1 with chart podinfo@6.7.0
```

## Sharding per Tenant

To isolate the resources reconciliation for different teams and environments, you can use the sharding feature
to create separate controllers for each tenant.

### Kustomization Example

To assign all the resources of a particular tenant to a specific shard, add the sharding key label to the
Flux Kustomization responsible for reconciling the tenant resources:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: tenant1
  namespace: tenant1
  labels:
    sharding.fluxcd.io/key: shard1
spec:
  commonMetadata:
    labels:
      sharding.fluxcd.io/key: shard1
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: tenant1
  path: ./deploy
  prune: true
```

The `commonMetadata.labels` field is used to propagate the sharding key label to the resources
reconciled by the Kustomization, such as HelmReleases, OCIRepositories, HelmCharts, HelmRepositories, etc.

### ClusterPolicy Example

Another option to assign all the resources of a particular tenant to a specific shard is to use a mutating
webhook to inject the sharding key label in the resources created for the tenant in their namespace.

Example Kyverno policy to inject the sharding key label to all the Flux
resources created in the `tenant1` namespace:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: tenant1-shard1
spec:
  rules:
    - name: add-shard-label
      match:
        any:
          - resources:
              namespaces:
                - tenant1
              kinds:
                - Kustomization
                - HelmRelease
                - HelmChart
                - HelmRepository
                - GitRepository
                - OCIRepository
                - Bucket
      mutate:
        patchStrategicMerge:
          metadata:
            labels:
              sharding.fluxcd.io/key: shard1
```

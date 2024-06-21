# Flux Report CRD

**FluxReport** is an API that reflects the observed state of a Flux installation.
Its purpose is to aid in monitoring and troubleshooting Flux by providing
information about the installed components and their readiness, the distribution details,
reconcilers statistics, cluster sync status, etc.

A single custom resource of this kind can exist in a Kubernetes cluster
with the name `flux`. The resource is automatically generated in the same namespace
where the flux-operator is deployed and is updated by the operator at regular intervals.

## Example

The following example shows a FluxReport custom resource generated on a cluster
where a [FluxInstance](fluxinstance.md) was deployed:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxReport
metadata:
  name: flux
  namespace: flux-system
spec:
  components:
    - image: ghcr.io/fluxcd/helm-controller:v1.0.1@sha256:a67a037faa850220ff94d8090253732079589ad9ff10b6ddf294f3b7cd0f3424
      name: helm-controller
      ready: true
      status: 'Current Deployment is available. Replicas: 1'
    - image: ghcr.io/fluxcd/kustomize-controller:v1.3.0@sha256:48a032574dd45c39750ba0f1488e6f1ae36756a38f40976a6b7a588d83acefc1
      name: kustomize-controller
      ready: true
      status: 'Current Deployment is available. Replicas: 1'
    - image: ghcr.io/fluxcd/notification-controller:v1.3.0@sha256:c0fab940c7e578ea519097d36c040238b0cc039ce366fdb753947428bbf0c3d6
      name: notification-controller
      ready: true
      status: 'Current Deployment is available. Replicas: 1'
    - image: ghcr.io/fluxcd/source-controller:v1.3.0@sha256:161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc
      name: source-controller
      ready: true
      status: 'Current Deployment is available. Replicas: 1'
  distribution:
    entitlement: Issued by controlplane
    managedBy: flux-operator
    status: Installed
    version: v2.3.0
  reconcilers:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      stats:
        failing: 1
        running: 42
        suspended: 3
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      stats:
        failing: 0
        running: 5
        suspended: 0
    - apiVersion: notification.toolkit.fluxcd.io/v1
      kind: Receiver
      stats:
        failing: 0
        running: 1
        suspended: 0
    - apiVersion: notification.toolkit.fluxcd.io/v1beta3
      kind: Alert
      stats:
        failing: 0
        running: 1
        suspended: 0
    - apiVersion: notification.toolkit.fluxcd.io/v1beta3
      kind: Provider
      stats:
        failing: 0
        running: 1
        suspended: 0
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      stats:
        failing: 0
        running: 2
        suspended: 0
        totalSize: 3.7 MiB
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: HelmChart
      stats:
        failing: 1
        running: 55
        suspended: 0
        totalSize: 15.7 MiB
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: HelmRepository
      stats:
        failing: 0
        running: 7
        suspended: 3
        totalSize: 40.5 MiB
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: Bucket
      stats:
        failing: 0
        running: 0
        suspended: 0
    - apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: OCIRepository
      stats:
        failing: 0
        running: 1
        suspended: 0
        totalSize: 78.1 KiB
  sync:
    ready: true
    id: kustomization/flux-system
    path: clusters/production
    source: https://github.com/my-org/my-fleet.git
    status: 'Applied revision: refs/heads/main@sha1:a90cd1ac35de01c175f7199315d3f4cd60195911'
status:
  conditions:
    - lastTransitionTime: "2024-06-20T19:59:30Z"
      message: Reporting finished in 272ms
      observedGeneration: 4
      reason: ReconciliationSucceeded
      status: "True"
      type: Ready
```

1. Export the report in YAML format:

   ```shell
   kubectl -n flux-system get fluxreport/flux -o yaml
   ```

2. Trigger a reconciliation of the report:

   ```shell
   kubectl -n flux-system annotate --overwrite fluxreport/flux \
    reconcile.fluxcd.io/requestedAt="$(date +%s)"
   ```

3. Change the report reconciliation interval:

   ```shell
   kubectl -n flux-system annotate --overwrite fluxreport/flux \
    fluxcd.controlplane.io/reconcileEvery=5m
   ```

4. Pause the report reconciliation:

   ```shell
   kubectl -n flux-system annotate --overwrite fluxreport/flux \
    fluxcd.controlplane.io/reconcile=disabled
   ```

5. Resume the reconciliation of the report:

   ```shell
    kubectl -n flux-system annotate --overwrite fluxreport/flux \
     fluxcd.controlplane.io/reconcile=enabled
    ```

## Reading a FluxReport

As with all other Kubernetes config, a FluxReport is identified by `apiVersion`,
`kind`, and `metadata` fields. The `spec` field contains detailed information
about the Flux installation, including statistic data for the Flux custom resources
that are reconciled by the Flux controllers.

### Distribution information

The `.spec.distribution` field contains information about the Flux distribution,
including the version, installation status, entitlement issuer
and tool that is managing the distribution.

Example distribution information for when Flux
was installed using the bootstrap command:

```yaml
spec:
  distribution:
    entitlement: Issued by controlplane
    managedBy: 'flux bootstrap'
    status: Installed
    version: v2.3.0
```

### Components information

The `.spec.components` field contains information about the Flux controllers,
including the controller name, the image repository, tag, and digest, and the
deployment readiness status.

Example:

```yaml
spec:
  components:
    - image: ghcr.io/fluxcd/kustomize-controller:v1.3.0@sha256:48a032574dd45c39750ba0f1488e6f1ae36756a38f40976a6b7a588d83acefc1
      name: kustomize-controller
      ready: true
      status: 'Current Deployment is available. Replicas: 1'
    - image: ghcr.io/fluxcd/source-controller:v1.3.0@sha256:161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc
      name: source-controller
      ready: true
      status: 'Current Deployment is available. Replicas: 1'
```

### Reconcilers statistics

The `.spec.reconcilers` field contains statistics about the Flux custom resources
that are reconciled by the Flux controllers, including the API version, kind, and
the number of resources in each state: failing, running and suspended.
For source type resources, the storage size of the locally cached artifacts is also reported.

Example:

```yaml
spec:
  reconcilers:
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      stats:
       failing: 1
       running: 5
       suspended: 5
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      stats:
       failing: 1
       running: 2
       suspended: 3
       totalSize: 5.5 MiB
```

### Cluster sync status

The `.spec.sync` field contains information about the cluster sync status,
including the Flux Kustomization name, source URL, the applied revision,
and the sync readiness status.

Example:

```yaml
spec:
  sync:
    ready: true
    id: kustomization/flux-system
    path: tests/v2.3/sources
    source: https://github.com/controlplaneio-fluxcd/distribution.git
    status: 'Applied revision: refs/heads/main@sha1:a90cd1ac35de01c175f7199315d3f4cd60195911'
```

## Generating a FluxReport

The FluxReport is automatically generated by the operator for the following conditions:

- At startup, when the operator is installed or upgraded.
- When the [FluxInstance](fluxinstance.md) is created or updated.
- When the `reconcile.fluxcd.io/requestedAt` annotation is set on the FluxReport resource.
- At regular intervals, controlled by the `fluxcd.controlplane.io/reconcileEvery` annotation.

### Reconciliation configuration

The reconciliation behaviour can be configured using the following annotations:

- `fluxcd.controlplane.io/reconcile`: Enable or disable the reconciliation loop. Default is `enabled`, set to `disabled` to pause the reconciliation.
- `fluxcd.controlplane.io/reconcileEvery`: Set the reconciliation interval. Default is `10m`.

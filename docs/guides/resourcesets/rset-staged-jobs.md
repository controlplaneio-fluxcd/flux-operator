---
title: Running Jobs with ResourceSet Steps
description: How to run Kubernetes Jobs in sequence with app deployments using Flux Operator
---

# Running Jobs with ResourceSet Steps

A common requirement when deploying applications is to run one-off tasks in sequence
with the deployment itself: a database migration must complete before the new version
rolls out, and a cache warmup or smoke test should run only after the rollout has finished.

The [ResourceSet](resourceset.md) API supports this workflow through the
[`.spec.steps`](resourceset.md#steps-configuration) field: an ordered list of named steps
that combine Kubernetes Jobs with Flux appliers (Kustomization, HelmRelease).
Each step's resources are applied and health-checked before the next step starts,
and a failed step blocks the rest of the sequence.

## Staged Deployment Workflow

The upstream Flux [running jobs](https://fluxcd.io/flux/use-cases/running-jobs/) use case
implements this workflow with **three Kustomization objects chained with `dependsOn`**
(`app-pre-deploy` → `app-deploy` → `app-post-deploy`), three repository directories,
and `force: true`/`wait: true` set on each Kustomization.

With ResourceSet steps, the equivalent workflow is expressed as a single object,
with the same execution flow: the migration runs first, the app deployment is blocked
on its completion, and the cache warmup Job runs only after the rollout has finished.

Combined with a [ResourceSetInputProvider](resourcesetinputprovider.md) that scans
the container registry, the workflow becomes **Gitless GitOps**
[image automation](rset-image-automation.md): when a new image version is published,
the whole sequence runs for it without any Git commit.

First, define an input provider that picks the latest stable version of the
`podinfo` image according to semver:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: podinfo-image
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  type: OCIArtifactTag
  url: oci://ghcr.io/stefanprodan/podinfo
  filter:
    semver: "*"
    limit: 1
```

Then define the staged workflow, with the exported tag templated into the Jobs
and set in the Flux Kustomization using an image patch:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: podinfo
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: podinfo-image
  wait: true
  steps:
    - name: pre-deploy
      timeout: 5m
      resources:
        - apiVersion: batch/v1
          kind: Job
          metadata:
            name: db-migration
            namespace: apps
            annotations:
              fluxcd.controlplane.io/force: enabled
              fluxcd.controlplane.io/recreateOnFailure: enabled
          spec:
            template:
              spec:
                restartPolicy: Never
                containers:
                  - name: migration
                    image: ghcr.io/stefanprodan/podinfo:<< inputs.tag >>
                    command: ["sh", "-c", "echo running db migration"]
    - name: deploy
      timeout: 10m
      resources:
        - apiVersion: kustomize.toolkit.fluxcd.io/v1
          kind: Kustomization
          metadata:
            name: podinfo
            namespace: apps
          spec:
            targetNamespace: apps
            sourceRef:
              kind: GitRepository
              name: apps
            path: deploy/podinfo
            interval: 60m
            prune: true
            wait: true
            timeout: 9m
            images:
              - name: ghcr.io/stefanprodan/podinfo
                newTag: << inputs.tag | quote >>
    - name: post-deploy
      timeout: 5m
      resources:
        - apiVersion: batch/v1
          kind: Job
          metadata:
            name: cache-warmup
            namespace: apps
            annotations:
              fluxcd.controlplane.io/force: enabled
              fluxcd.controlplane.io/recreateOnFailure: enabled
          spec:
            template:
              spec:
                restartPolicy: Never
                containers:
                  - name: cache
                    image: ghcr.io/stefanprodan/podinfo:<< inputs.tag >>
                    command: ["sh", "-c", "echo refreshing cache"]
```

On reconciliation, the operator applies the `pre-deploy` step and waits up to `5m`
for the `db-migration` Job to complete. It then applies the `deploy` step and waits
up to `10m` for the Flux Kustomization to become ready, before applying the
`post-deploy` step. Because `.spec.wait` is set to `true`, the final step is also
health-checked.

When the input provider detects a new image version, the `force` annotation on the
Jobs makes the operator recreate them, so the migration and cache warmup run again
for the new version. Because the tag is set in the Kustomization **spec** through
the `.spec.images` patch, the version bump changes its `metadata.generation` and
the `deploy` step waits for the actual rollout of the new pods (see
[Triggering Rollouts on Data-Only Changes](#triggering-rollouts-on-data-only-changes)).

### Comparison with the Flux Kustomization Pattern

The ResourceSet steps map onto the upstream pattern as follows:

- The `dependsOn` chain between Kustomizations → the step order.
- The per-Kustomization `wait: true` → the implied wait between steps
  (the final step is gated by the ResourceSet `.spec.wait` field).
- The per-Job Kustomization `force: true` → the per-Job
  `fluxcd.controlplane.io/force: enabled` annotation
  (the Job is recreated when the image tag changes).
- The per-Job Kustomization `timeout` → the per-Job `timeout`.
- The per-Job Kustomization `prune: true` → the ResourceSet inventory-based
  [garbage collection](resourceset.md#garbage-collection).

Beyond replicating the upstream pattern, the single-object form adds:
one status, inventory and history to inspect instead of three; no `dependsOn`
requeue polling latency between stages; the opt-in
[`recreateOnFailure`](#re-running-jobs) annotation to retry a permanently
failed migration; and version bumps that flow automatically from the registry
scan through a single input, instead of Git edits in three places.

## Re-running Jobs

Kubernetes Job specs are immutable, so the supported re-run mechanism is the
`fluxcd.controlplane.io/force: enabled` annotation: when the rendered Job spec
changes (e.g. the image tag from an input bump), the operator deletes and
recreates the Job, and it runs again. With an unchanged spec, re-applying a
completed Job is a no-op, which makes retrying a failed sequence idempotent.

The same no-op behavior means a Job that has failed permanently (e.g. its
`backoffLimit` is exhausted) stays failed: the ResourceSet remains `Ready=False`
until the Job spec or an input changes. To automatically retry failed Jobs,
add the `fluxcd.controlplane.io/recreateOnFailure: enabled` annotation:

```yaml
metadata:
  annotations:
    fluxcd.controlplane.io/force: enabled
    fluxcd.controlplane.io/recreateOnFailure: enabled
```

Before applying a step, the operator deletes any Job in that step that carries
this annotation and has the `Failed=True` condition, then recreates it when
applying the step.

**Warning**: only use `recreateOnFailure` for idempotent Jobs. A non-idempotent
migration would be re-run on every reconciliation until it succeeds, repeating
any partial changes it made before failing. The annotation works for any Job
managed by a ResourceSet, with or without steps.

## Triggering Rollouts on Data-Only Changes

To trigger a rolling update of the app workloads when the config inputs
change, hash the inputs with the `sha256sum` template function and inject the
result into the pod template using a Kustomization patch:

```yaml
- apiVersion: kustomize.toolkit.fluxcd.io/v1
  kind: Kustomization
  spec:
    patches:
      - target:
          kind: Deployment
        patch: |
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: all
          spec:
            template:
              metadata:
                annotations:
                  config-checksum: << inputs.config | toYaml | sha256sum | quote >>
```

When the workload is defined directly in a step (without an applier in
between), the
[`checksumFrom`](resourceset.md#computing-checksums-from-configmaps-and-secrets)
annotation on its pod template works as documented and the step health check
waits on the workload rollout itself.

## Job Lifecycle Caveats

Do **not** set `ttlSecondsAfterFinished` on Jobs managed by a ResourceSet.
When the TTL controller deletes a completed Job, the operator detects the
missing resource as drift on the next reconciliation and re-applies it,
causing the migration to run again unexpectedly.

To explicitly re-run a Job for every new revision while keeping a record of
past runs, template the Job name from an input revision, e.g.
`name: db-migration-<< inputs.tag >>`. Each version bump creates a new Job,
and the previous one is removed by garbage collection after the sequence succeeds.

## Copying Data Between Steps

The [`copyFrom`](resourceset.md#copying-data-from-existing-configmaps-and-secrets) annotation reads from the cluster **before the first step runs**,
it is not step-aware. A resource in a later step cannot copy data from a Secret created by
an earlier step in the same reconciliation.

The only step-safe cross-reference is `checksumFrom` pointing at ConfigMaps or
Secrets generated by the same ResourceSet: those references are resolved
in-memory from the pending apply, regardless of which step defines them.

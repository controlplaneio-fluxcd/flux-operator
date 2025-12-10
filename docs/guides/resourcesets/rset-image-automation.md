---
title: Using ResourceSets for Image Update Automation
description: Flux Operator ResourceSets guide for automating container image updates
---

# Using ResourceSets for Image Update Automation

This guide demonstrates how to use the Flux Operator APIs as an alternative to the
Flux [Image Automation](https://fluxcd.io/docs/components/image/automation/) controllers.

The Flux Operator approach to image update automation is suitable for **Gitless GitOps** workflows
where instead of pushing changes to a Git repository, the updates are applied directly to the cluster
based on policies defined in the desired state.

## How ResourceSets and Input Providers Work

Before diving into the configuration, it's important to understand how The Flux Operator APIs
work together to enable deployment rollouts based on container image updates.

The [ResourceSet](resourceset.md) API allows you to define a set of Flux resources
for deploying an application, while the [ResourceSetInputProvider](resourcesetinputprovider.md) API
is used to provide inputs to the `ResourceSet`, such as Helm chart versions and container image tags
that determine which configuration of the application should be deployed.

## GitOps Workflow

To demonstrate the image update automation workflow, we'll define a series of Flux Operator
custom resources in a cluster. Note that the cluster must be provisioned with a
[Flux Instance](fluxinstance.md).

For this example, we'll use the `podinfo` demo application, that consists of a Helm chart
stored as an OCI artifact in GitHub Container Registry that deploys two container images:
`podinfo` and `redis`.

### Configure Registry Scanning

First, we'll create a `ResourceSetInputProvider` that scan the registry for new versions
of the `podinfo` Helm chart and pick the latest stable version according to semver:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: podinfo-chart
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "15m"
spec:
  type: OCIArtifactTag
  url: oci://ghcr.io/stefanprodan/charts/podinfo
  filter:
    semver: ">=6.0.0"
    limit: 1
```

Next, we'll create a `ResourceSetInputProvider` the podinfo container image that scans
the registry for new digests of `ghcr.io/stefanprodan/podinfo:latest` image:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: podinfo-image
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "5m"
spec:
  type: OCIArtifactTag
  url: oci://ghcr.io/stefanprodan/podinfo
  filter:
    includeTag: "latest"
    limit: 1
```

Finally, we'll create a `ResourceSetInputProvider` for the `docker.io/redis` image
that picks the latest semver version of the alpine variant:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: redis-image
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "15m"
spec:
  type: OCIArtifactTag
  url: oci://docker.io/redis
  filter:
    semver: ">0.0.0-0"
    includeTag: ".*-alpine$"
    limit: 1
```

Note that you can provide credentials for private registries by referencing
a Secret of type `kubernetes.io/dockerconfigjson` in the `spec.secretRef` field.

### Configure the App Deployment

With the providers in place, we can now create a `ResourceSet` that generates
the Flux resources required to deploy the `podinfo` application using the latest chart
and container images exported by the input providers:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: podinfo
  namespace: apps
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: "5m"
spec:
  inputStrategy:
    name: Permute
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: podinfo-chart
    - kind: ResourceSetInputProvider
      name: podinfo-image
    - kind: ResourceSetInputProvider
      name: redis-image
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: podinfo
        namespace: << inputs.podinfo_chart.provider.namespace >>
      spec:
        interval: 12h
        url: oci://ghcr.io/stefanprodan/charts/podinfo
        ref:
          tag: << inputs.podinfo_chart.tag >>
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: podinfo
        namespace: << inputs.podinfo_chart.provider.namespace >>
      spec:
        interval: 30m
        releaseName: podinfo
        chartRef:
          kind: OCIRepository
          name: podinfo
        values:
          image:
            tag: "<< inputs.podinfo_image.tag >>@<< inputs.podinfo_image.digest >>"
          redis:
            enabled: true
            tag: "<< inputs.redis_image.tag >>@<< inputs.redis_image.digest >>"
```

In the resources section of the `ResourceSet`, we define an `OCIRepository` that points
to the Helm chart and a `HelmRelease` that deploys the application using the chart.

The chart version is set using the `<< inputs.podinfo_chart.tag >>` template variable,
which is populated by the `podinfo-chart` input provider. Every time the input provider
detects a new chart version, the `ResourceSet` will trigger a Helm release upgrade to
deploy the new version.

The container image tags for `podinfo` and `redis` are set along with their digests
using the following template variables:

- `<< inputs.podinfo_image.tag >>@<< inputs.podinfo_image.digest >>`
- `<< inputs.redis_image.tag >>@<< inputs.redis_image.digest >>`

These template variables are populated by the respective input providers and will
trigger a Helm release upgrade whenever a new image version is detected, either
based on a new digest for the `latest` tag of `podinfo` or a new semver version
of the `redis` image.

### Patching Container Images

There are cases when a Helm chart does not expose all its images in values. In such cases,
you can use Kustomize patches to modify the manifests before helm-controller applies them:

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
spec:
  postRenderers:
    - kustomize:            
        images:
          - name: ghcr.io/stefanprodan/podinfo
            newTag: << inputs.podinfo_image.tag | quote >>
            digest: << inputs.podinfo_image.digest | quote >>
```

Similarly, when an application is deployed using a Flux `Kustomization`,
you can use the `.spec.images` field to define the container images to be updated:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
spec:
  images:
    - name: ghcr.io/stefanprodan/podinfo
      newTag: << inputs.podinfo_image.tag | quote >>
      digest: << inputs.podinfo_image.digest | quote >>
```

### Configure Notifications

To get notified when the image update triggers a deployment, we can create
a Flux [Alert](https://fluxcd.io/flux/components/notification/alerts/) that sends notifications to e.g. a Slack channel:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: slack-bot
  namespace: apps
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
  name: slack
  namespace: apps
spec:
  providerRef:
    name: slack-bot
  eventSources:
    - kind: OCIRepository
      name: '*'
    - kind: HelmRelease
      name: '*'
    - kind: Kustomization
      name: '*'
    - kind: ResourceSet
      name: '*'
  eventMetadata:
    cluster: "dev-cluster-1"
    region: "us-east-1"
```

## Working with ResourceSets

Using the [Flux Operator CLI](cli.md), you can interact with ResourceSets and their input providers.

To view the status of the `ResourceSet`, its input providers and the deployed `HelmRelease`:

```shell
flux-operator -n apps get all
```


To pause the update automation for a particular image, you can suspend the corresponding
`ResourceSetInputProvider` with:

```shell
flux-operator -n apps suspend rsip redis-image
```

To pause the entire image update automation workflow, you can suspend the `ResourceSet` with:

```shell
flux-operator -n apps suspend rset podinfo
```

To resume the automation:

```shell
flux-operator -n apps resume rsip redis-image
flux-operator -n apps resume rset podinfo
```

To trigger an immediate image scan:

```shell
flux-operator -n apps reconcile rsip redis-image
```

In addition, you can use the CLI to build the `ResourceSet` locally and verify
that the templates are valid with mock input data:

```shell
flux-operator build rset -f podinfo-resourceset.yaml \
  --inputs-from-provider static-inputs.yaml
```

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)

---
title: Ephemeral Environments for GitLab Merge Requests
description: Flux Operator preview environments integration with GitLab
---

# Ephemeral Environments for GitLab Merge Requests

This guide demonstrates how to use the Flux Operator ResourceSet API to automate the deployment of
applications changes made in GitLab Merge Requests to ephemeral environments for testing and validation.

If you need more precise control over when an ephemeral environment gets created or torn down for a merge request,
consider using the [GitLab Environment integration](rset-gitlab-environments.md) instead. That integration is also an
option if you need to create dynamic environments that are not linked to merge requests.

## Development workflow

- A developer opens a Merge Requests with changes to the app code and Helm chart.
- The CI builds and pushes the app container image to GitLab Container Registry. The image is tagged with the Git commit SHA.
- Another developer reviews the changes and labels the Merge Request with the `deploy/flux-preview` label.
- Flux Operator running in the preview cluster scans the GitLab project and finds the new MR using the label filter.
- Flux Operator installs a Helm release using the MR number and the commit SHA inputs to deploy the app and chart changes in the cluster.
- The app is accessible at a preview URL composed of the MR number and the app name.
- The developers iterate over changes, with each push to the MR branch triggering a Helm release upgrade in the cluster.
- The developers are notified of the Helm release status in the MS Teams channel.
- Once the MR is approved and merged, the Flux Operator uninstalls the Helm release from the cluster.

## GitOps workflow

To enable the development workflow, we'll define a series of Flux Operator custom resources in the preview cluster.
Note that the preview cluster must be provisioned with a [Flux Instance](fluxinstance.md) and the Kubernetes
manifests part of the GitOps workflow should be stored in the GitLab project used by the Flux Instance.

### Preview namespace

First we'll create a dedicated namespace called `app-preview` where all the app instances generated
from GitLab Merge Requests will be deployed. We'll also create a service account for Flux that limits
the permissions to the `app-preview` namespace.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: app-preview
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: flux
  namespace: app-preview
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: flux
  namespace: app-preview
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: admin
subjects:
  - kind: ServiceAccount
    name: flux
    namespace: app-preview
```

In this namespace, we'll create a Kubernetes Secret
containing a GitLab PAT that grants read access to the app project and MRs.

```shell
echo $GITLAB_TOKEN | flux-operator -n app-preview create secret basic-auth gitlab-token-readonly \
  --username=flux \
  --password-stdin
```

### ResourceSet input provider

In the `app-preview` namespace, we'll create a [ResourceSetInputProvider](resourcesetinputprovider.md)
that tells Flux Operator to scan the GitLab project for MRs labeled with `deploy/flux-preview`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-merge-requests
  namespace: app-preview
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  type: GitLabMergeRequest
  url: https://gitlab.com/group/app
  secretRef:
    name: gitlab-token-readonly
  filter:
    labels:
      - "deploy/flux-preview"
  defaultValues:
    chart: "charts/app"
```

### GitLab Webhook

Optionally, we can create a Flux [Webhook Receiver](https://fluxcd.io/flux/components/notification/receivers/)
that GitLab will call to notify the Flux Operator when a new MR is opened or updated: 

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: gitlab-receiver
  namespace: app-preview
spec:
  type: gitlab
  secretRef:
    name: receiver-token
  resources:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: app-merge-requests
```

### ResourceSet template

Finally, to deploy the app from MRs, we'll create a [ResourceSet](resourceset.md)
that uses the `ResourceSetInputProvider` as its input source:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app
  namespace: app-preview
spec:
  serviceAccountName: flux
  inputsFrom:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: app-merge-requests
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: app-<< inputs.id >>
        namespace: app-preview
      spec:
        interval: 1h
        url: https://gitlab.com/group/app
        ref:
          commit: << inputs.sha >>
        secretRef:
          name: gitlab-token-readonly
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: app-<< inputs.id >>
        namespace: app-preview
        annotations:
          event.toolkit.fluxcd.io/change_request: << inputs.id | quote >>
          event.toolkit.fluxcd.io/commit: << inputs.sha | quote >>
          event.toolkit.fluxcd.io/preview-url: "https://app-<< inputs.id >>.example.com"
          event.toolkit.fluxcd.io/branch: << inputs.branch | quote >>
          event.toolkit.fluxcd.io/author: << inputs.author | quote >>
      spec:
        serviceAccountName: flux
        interval: 10m
        releaseName: app-<< inputs.id >>
        chart:
          spec:
            chart: << inputs.chart >>
            reconcileStrategy: Revision
            sourceRef:
              kind: GitRepository
              name: app-<< inputs.id >>
        values:
          image:
            tag: << inputs.sha >>
          ingress:
            hosts:
              - host: app-<< inputs.id >>.example.com
```

The above `ResouceSet` will generate a Flux `GitRepository` and a `HelmRelease` for each opened MR.
The MR number passed as `<< inputs.id >>` is used as the name suffix for the Flux objects,
and is also used to compose the Ingress host name where the app can be accessed.

The latest commit SHA pushed to the MR HEAD is passed as `<< inputs.sha >>`,
the SHA is used to set the app image tag in the Helm release values.

The `change_request` and `commit` annotations are used by the Flux notification providers
to post comments and commit statuses on the Merge Request. The preview URL, branch name
and author are set as extra metadata to enrich the notifications that the dev team receives.

To verify the ResourceSet templates are valid, we can use the
[Flux Operator CLI](cli.md) and build them locally:

```shell
flux-operator build resourceset -f app-resourceset.yaml \
  --inputs-from test-inputs.yaml
```

The `test-inputs.yaml` file should contain mock MR data e.g.:

```yaml
   - author: test
     branch: feat/test
     id: "1"
     sha: bf5d6e01cf802734853f6f3417b237e3ad0ba35d
     title: 'testing'
```

### Notifications

To receive notifications when a MR triggers a Helm release install,
upgrade and uninstall (including any deploy errors),
a Flux [Alert](https://fluxcd.io/flux/components/notification/alerts/)
can be created in the `app-preview` namespace:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: msteams
  namespace: app-preview
spec:
  type: msteams
  secretRef:
    name: msteams-webhook
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: msteams
  namespace: app-preview
spec:
  providerRef:
    name: msteams
  eventSources:
    - kind: GitRepository
      name: '*'
    - kind: HelmRelease
      name: '*'
  eventMetadata:
    cluster: "preview-cluster-1"
    region: "eastus-1"
```

### Status reporting on Merge Requests

To notify the developers of the preview deployment status directly on the Merge Request page,
we can use the Flux notification providers for
[GitLab MR comments](https://fluxcd.io/flux/components/notification/providers/#gitlab-merge-request-comment)
and [GitLab commit statuses](https://fluxcd.io/flux/components/notification/providers/#gitlab-commit-status).

The `gitlabmergerequestcomment` provider posts a comment on the MR page with the deployment status
and metadata. The comment is automatically updated on subsequent events, so it doesn't spam the MR
with multiple comments. The `gitlab` commit status provider posts a status check on the commit
that triggered the deployment.

The MR comment provider requires the `change_request` annotation and the commit status provider
requires the `commit` annotation, both set on the HelmRelease as shown in the
[ResourceSet template](#resourceset-template) above.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: gitlab-mr-comment
  namespace: app-preview
spec:
  type: gitlabmergerequestcomment
  address: https://gitlab.com/group/app
  secretRef:
    name: gitlab-token-readonly
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: gitlab-mr-comment
  namespace: app-preview
spec:
  providerRef:
    name: gitlab-mr-comment
  eventSeverity: info
  eventSources:
    - kind: HelmRelease
      name: '*'
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: gitlab-commit-status
  namespace: app-preview
spec:
  type: gitlab
  address: https://gitlab.com/group/app
  secretRef:
    name: gitlab-token-readonly
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: gitlab-commit-status
  namespace: app-preview
spec:
  providerRef:
    name: gitlab-commit-status
  eventSeverity: info
  eventSources:
    - kind: HelmRelease
      name: '*'
```

Every time a commit is pushed to the MR branch, the Flux Operator will upgrade the Helm release
and will update the MR comment and commit status with the latest deployment status.

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)

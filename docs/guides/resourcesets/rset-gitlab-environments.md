---
title: Ephemeral Environments for GitLab Environments
description: Flux Operator preview environments integration with GitLab using GitLab Environments
---

# Ephemeral Environments for GitLab Environments 

This guide demonstrates how to use the Flux Operator ResourceSet API to automate the deployment of
[GitLab Environments](https://docs.gitlab.com/ci/environments/) to ephemeral environments for
testing and validation of Merge Requests.

We recommend this method if you want more precise control over when an ephemeral environment gets created and torn down.
For a simpler approach, consider using the [Merge Request integration](rset-gitlab-merge-requests.md) instead,
which provides a `ResourceSet` input for every open merge request. Note that the GitLab Environment integration can
also be used dynamic use cases other than merge requests.

## Development workflow

- A developer opens a Merge Requests with changes to the app code and Helm chart.
- The CI builds and pushes the app container image to GitLab Container Registry. The image is tagged with the Git commit SHA.
- The CI or another developer after review creates a new GitLab Environment with a `review/` name prefix for the Merge Request.
- Flux Operator running in the preview cluster scans the GitLab project and finds the new environment.
- Flux Operator installs a Helm release using the environment slug and the commit SHA inputs to deploy the app and chart changes in the cluster.
- The app is accessible at a preview URL composed of the environment slug and the app name.
- The developers iterate over changes, with each deployment to the environment triggering a Helm release upgrade in the cluster.
- The developers are notified of the Helm release status in the MS Teams channel.
- Once the environment is stopped, e.g. after the Merge Request was closed, the Flux Operator uninstalls the Helm release from the cluster.

## GitOps workflow

To enable the development workflow, we'll define a series of Flux Operator custom resources in the preview cluster.
Note that the preview cluster must be provisioned with a [Flux Instance](fluxinstance.md) and the Kubernetes
manifests part of the GitOps workflow should be stored in the GitLab project used by the Flux Instance.

### Preview namespace

First we'll create a dedicated namespace called `app-preview` where all the app instances generated
from GitLab Environments will be deployed. We'll also create a service account for Flux that limits
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
containing a GitLab PAT that grants read access to the app project and environments.

```shell
flux -n app-preview create secret git gitlab-token-readonly \
  --url=https://gitlab.com/group/app \
  --username=flux \
  --password=${GITLAB_TOKEN}
```

### ResourceSet input provider

In the `app-preview` namespace, we'll create a [ResourceSetInputProvider](resourcesetinputprovider.md)
that tells Flux Operator to scan the GitLab project for environments whose name starts with `review/`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-review-environments
  namespace: app-preview
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  type: GitLabEnvironment
  url: https://gitlab.com/group/app
  secretRef:
    name: gitlab-token-readonly
  filter:
    includeEnvironment: review/.*
  defaultValues:
    chart: "charts/app"
```

### Deploying the GitLab Environment

We need to create and start deployments in a GitLab environment for Flux Operator to pick it up. The following is an
example `.gitlab-ci.yml` that both creates an environment and manually triggers a reconciliation
of the `ResourceSetInputProvider` in the cluster. This assumes a Kubernetes context to be available in your pipeline,
we recommend [GitLab's Kubernetes Agent](https://docs.gitlab.com/user/clusters/agent/) for providing this.

Reconciling from within the deploy and stop jobs ensures that the deployment's success is tied to the validity of the
`ResourceSetInputProvider`. You may also use the deployed application in subsequent CI jobs, e.g. to run some tests
against it from within the pipeline.

```yaml
stages:
  - review

# Create (and start) a review environment
start-review:
  stage: review
  environment:
    name: review/$CI_COMMIT_REF_SLUG
    # Matches configuration in the ResourceSet
    url: https://app-$CI_ENVIRONMENT_SLUG.example.com
    action: start
    on_stop: "stop-review"
  # We need an image with a shell, the flux-operator CLI and kubectl installed
  image: flux-operator-cli
  script: |
    # Set up Kubernetes context using GitLab Agent
    kubectl config use-context kubernetes_agent
    # Reconcile both the ResourceSetInputProvider and the ResourceSet
    flux-operator reconcile inputprovider --namespace app-preview --timeout 1m app-review-environments
    flux-operator reconcile resourceset --namespace app-preview --timeout 1m app-preview

# Stop the review environment
stop-review:
  stage: review
  when: manual
  environment:
    name: review/$CI_COMMIT_REF_SLUG
    action: stop
  # We need an image with a shell, the flux-operator CLI and kubectl installed
  image: flux-operator-cli
  script: |
    # Set up Kubernetes context using GitLab Agent
    kubectl config use-context kubernetes_agent
    # Reconcile both the ResourceSetInputProvider and the ResourceSet
    flux-operator reconcile inputprovider --namespace app-preview --timeout 1m app-review-environments
    flux-operator reconcile resourceset --namespace app-preview --timeout 1m app-preview
```

### GitLab Webhook

As an alternative to the synchronous reconciliation, we can create a Flux [Webhook Receiver](https://fluxcd.io/flux/components/notification/receivers/)
that GitLab will call to notify the Flux Operator when a deployment to an environment starts or gets stopped: 

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
      name: app-review-environments
```

### ResourceSet template

Finally, to deploy the app from GitLab environments, we'll create a [ResourceSet](resourceset.md)
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
      name: app-environments
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: app-<< inputs.slug >>
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
        name: app-<< inputs.slug >>
        namespace: app-preview
        annotations:
          event.toolkit.fluxcd.io/preview-url: "https://app-<< inputs.slug >>.example.com"
          event.toolkit.fluxcd.io/branch: << inputs.branch | quote >>
          event.toolkit.fluxcd.io/author: << inputs.author | quote >>
      spec:
        serviceAccountName: flux
        interval: 10m
        releaseName: app-<< inputs.slug >>
        chart:
          spec:
            chart: << inputs.chart >>
            reconcileStrategy: Revision
            sourceRef:
              kind: GitRepository
              name: app-<< inputs.slug >>
        values:
          image:
            tag: << inputs.sha >>
          ingress:
            hosts:
              - host: app-<< inputs.slug >>.example.com
```

The above `ResouceSet` will generate a Flux `GitRepository` and a `HelmRelease` for each available environment.
The environment slug passed as `<< inputs.slug >>` is used as the name suffix for the Flux objects,
and is also used to compose the Ingress host name where the app can be accessed.

The latest commit SHA from which a deployment was run is passed as `<< inputs.sha >>`,
the SHA is used to set the app image tag in the Helm release values.

The preview URL, branch name and author are set as annotations on the HelmRelease
object to enrich the Flux [notifications](#notifications) that the dev team receives.

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
     title: 'review/testing-env'
     slug: 'review-testing-env'
```

### Notifications

To receive notifications when a deployment triggers a Helm release install,
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

### GitLab Environment Cleanup
Since the integration for GitLab Environments has to list *all* environments, including currently stopped ones
to catch environments that get restarted, reasonable performance requires that there not be too many stale environments.

When naming environments with the `review/` prefix, you have the option to use a [dedicated API endpoint](https://docs.gitlab.com/api/environments/#delete-multiple-stopped-review-apps)
for deleting old stopped environments. We recommend creating a scheduled pipeline that invokes this or otherwise run this periodically.

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)

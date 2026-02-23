---
title: Preview Environments for Feature Branches
description: Flux Operator preview environments for feature branches on GitHub, GitLab, Azure DevOps and Gitea
---

# Preview Environments for Feature Branches

This guide demonstrates how to use the Flux Operator ResourceSet API to automate the deployment of
applications from feature branches to preview environments for testing and validation.

The Flux Operator supports branch-based preview environments for the following Git providers
via the ResourceSetInputProvider
[`spec.type`](resourcesetinputprovider.md#type) field:

- `GitHubBranch`
- `GitLabBranch`
- `AzureDevOpsBranch`
- `GiteaBranch`

## Development workflow

- A developer creates a feature branch with a naming convention (e.g. `feat/` prefix) in the app repository.
- The CI builds and pushes the app container image tagged with the Git commit SHA.
- Flux Operator running in the preview cluster scans the repository and finds branches matching the configured pattern.
- Flux Operator installs a Helm release for each matching branch to deploy the app changes in the cluster.
- The app is accessible at a preview URL composed of the branch identifier and the app name.
- The developers iterate over changes, with each push to the branch triggering a Helm release upgrade in the cluster.
- The developers are notified of the deployment status via Slack and commit statuses on the Git provider.
- Once the branch is deleted (e.g. after the PR is merged), the Flux Operator uninstalls the Helm release from the cluster.

## GitOps workflow

To enable the development workflow, we'll define a series of Flux Operator custom resources in the preview cluster.
Note that the preview cluster must be provisioned with a [Flux Instance](fluxinstance.md) and the Kubernetes
manifests part of the GitOps workflow should be stored in the Git repository used by the Flux Instance.

### Preview namespace

First we'll create a dedicated namespace called `app-preview` where all the app instances generated
from feature branches will be deployed. We'll also create a service account for Flux that limits
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

### Authentication

In the `app-preview` namespace, create a Kubernetes Secret
containing credentials that grant read access to the app repository.

```shell
echo $GITHUB_TOKEN | flux-operator -n app-preview create secret basic-auth git-auth \
  --username=flux \
  --password-stdin
```

### ResourceSet input provider

In the `app-preview` namespace, we'll create a [ResourceSetInputProvider](resourcesetinputprovider.md)
that tells Flux Operator to scan the repository for branches matching a pattern (e.g. `feat/`):

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-branches
  namespace: app-preview
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  type: GitHubBranch
  url: https://github.com/org/app
  secretRef:
    name: git-auth
  filter:
    includeBranch: "feat/.*"
  defaultValues:
    chart: "charts/app"
```

The branch providers export the following inputs for use in the ResourceSet template:

| Input | Description |
|-------|-------------|
| `inputs.id` | A short identifier derived from the branch name, safe for use in Kubernetes resource names |
| `inputs.sha` | The latest commit SHA on the branch |
| `inputs.branch` | The branch name |

### Webhook

Optionally, we can create a Flux [Webhook Receiver](https://fluxcd.io/flux/components/notification/receivers/)
to notify the Flux Operator when a branch is created, updated or deleted:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: git-receiver
  namespace: app-preview
spec:
  type: github
  secretRef:
    name: receiver-token
  resources:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: app-branches
```

### ResourceSet template

To deploy the app from feature branches, we'll create a [ResourceSet](resourceset.md)
that takes its inputs from the `ResourceSetInputProvider`:

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
      name: app-branches
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: app-<< inputs.id >>
        namespace: app-preview
      spec:
        interval: 1h
        url: https://github.com/org/app
        ref:
          commit: << inputs.sha >>
        secretRef:
          name: git-auth
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: app-<< inputs.id >>
        namespace: app-preview
        annotations:
          event.toolkit.fluxcd.io/commit: << inputs.sha | quote >>
          event.toolkit.fluxcd.io/preview-url: "https://app-<< inputs.id >>.example.com"
          event.toolkit.fluxcd.io/branch: << inputs.branch | quote >>
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

The above `ResourceSet` will generate a Flux `GitRepository` and a `HelmRelease` for each matching branch.
The branch identifier passed as `<< inputs.id >>` is used as the name suffix for the Flux objects,
and is also used to compose the Ingress host name where the app can be accessed.

The latest commit SHA pushed to the branch is passed as `<< inputs.sha >>`,
the SHA is used to set the app image tag in the Helm release values.

The `commit` annotation is used by the Flux notification providers
to post commit statuses on the Git provider. The preview URL and branch name
are set as extra metadata to enrich the notifications that the dev team receives.

To verify the ResourceSet templates are valid, we can use the
[Flux Operator CLI](cli.md) and build them locally:

```shell
flux-operator build resourceset -f app-resourceset.yaml \
  --inputs-from test-inputs.yaml
```

The `test-inputs.yaml` file should contain mock branch data e.g.:

```yaml
   - branch: feat/test
     id: "123456"
     sha: bf5d6e01cf802734853f6f3417b237e3ad0ba35d
     chart: "charts/app"
```

### Notifications

To receive notifications when a branch triggers a Helm release install,
upgrade and uninstall (including any deploy errors),
a Flux [Alert](https://fluxcd.io/flux/components/notification/alerts/)
can be created in the `app-preview` namespace:

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: slack-bot
  namespace: app-preview
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
  namespace: app-preview
spec:
  providerRef:
    name: slack-bot
  eventSources:
    - kind: GitRepository
      name: '*'
    - kind: HelmRelease
      name: '*'
  eventMetadata:
    cluster: "preview-cluster-1"
    region: "us-east-1"
```

### Commit status reporting

To report the deployment status as a commit check on the Git provider,
we can use the Flux [commit status providers](https://fluxcd.io/flux/components/notification/providers/#git-commit-status).
This requires the HelmRelease to be annotated with the `commit` metadata key
as shown in the [ResourceSet template](#resourceset-template) above.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: commit-status
  namespace: app-preview
spec:
  type: github
  address: https://github.com/org/app
  secretRef:
    name: git-auth
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: commit-status
  namespace: app-preview
spec:
  providerRef:
    name: commit-status
  eventSeverity: info
  eventSources:
    - kind: HelmRelease
      name: '*'
```

The commit status provider `spec.type` should match your Git provider: `github`, `gitlab`,
`azuredevops` or `gitea`.

Every time a commit is pushed to a feature branch, the Flux Operator will upgrade the Helm release
and will update the commit status with the latest deployment status.

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)

---
title: Ephemeral Environments for GitHub Pull Requests
description: Flux Operator preview environments integration with GitHub
---

# Ephemeral Environments for GitHub Pull Requests

This guide demonstrates how to use the Flux Operator ResourceSet API to automate the deployment of
applications changes made in GitHub Pull Requests to ephemeral environments for testing and validation.

## Development workflow

- A developer opens a Pull Request with changes to the app code and Helm chart.
- The CI builds and pushes the app container image to GitHub Container Registry. The image is tagged with the Git commit SHA.
- Another developer reviews the changes and labels the Pull Request with the `deploy/flux-preview` label.
- Flux Operator running in the preview cluster scans the repository and finds the new PR using the label filter.
- Flux Operator installs a Helm release using the PR number and the commit SHA inputs to deploy the app and chart changes in the cluster.
- The app is accessible at a preview URL composed of the PR number and the app name.
- The developers iterate over changes, with each push to the PR branch triggering a Helm release upgrade in the cluster.
- The developers are notified of the Helm release status in the Slack channel and on the PR page.
- Once the PR is approved and merged, the Flux Operator uninstalls the Helm release from the cluster.

## GitOps workflow

To enable the development workflow, we'll define a series of Flux Operator custom resources in the preview cluster.
Note that the preview cluster must be provisioned with a [Flux Instance](fluxinstance.md) and the Kubernetes
manifests part of the GitOps workflow should be stored in the Git repository used by the Flux Instance.

### Preview namespace

First we'll create a dedicated namespace called `app-preview` where all the app instances generated
from GitHub Pull Requests will be deployed. We'll also create a service account for Flux that limits
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

### GitHub authentication

In the `app-preview` namespace, we'll create a Kubernetes Secret
containing a GitHub PAT that grants read access to the app repository and PRs.

```shell
echo $GITHUB_TOKEN | flux-operator -n app-preview create secret basic-auth github-auth \
  --username=flux \
  --password-stdin
```

Alternatively, we can use a GitHub App token for authentication:

```shell
flux-operator -n app-preview create secret githubapp github-auth \
  --app-id=1 \
  --app-installation-id=2 \
  --app-private-key-file=./private-key-file.pem
```

### ResourceSet input provider

In the `app-preview` namespace, we'll create a [ResourceSetInputProvider](resourcesetinputprovider.md)
that tells Flux Operator to scan the repository for PRs labeled with `deploy/flux-preview`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-pull-requests
  namespace: app-preview
  annotations:
    fluxcd.controlplane.io/reconcileEvery: "10m"
spec:
  type: GitHubPullRequest
  url: https://github.com/org/app
  secretRef:
    name: github-auth
  filter:
    labels:
      - "deploy/flux-preview"
  defaultValues:
    chart: "charts/app"
```

### GitHub Webhook

Optionally, we can create a Flux [Webhook Receiver](https://fluxcd.io/flux/components/notification/receivers/)
that GitHub will call to notify the Flux Operator when a new PR is opened or updated: 

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: github-receiver
  namespace: app-preview
spec:
  type: github
  secretRef:
    name: receiver-token
  resources:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: app-pull-requests
```

### ResourceSet template

Finally, to deploy the app from PRs we'll create a [ResourceSet](resourceset.md)
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
      name: app-pull-requests
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: GitRepository
      metadata:
        name: app-<< inputs.id >>
        namespace: app-preview
      spec:
        provider: generic # or 'github' if using GitHub App
        interval: 1h
        url: https://github.com/org/app
        ref:
          commit: << inputs.sha >>
        secretRef:
          name: github-auth
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

The above `ResouceSet` will generate a Flux `GitRepository` and a `HelmRelease` for each opened PR.
The PR number passed as `<< inputs.id >>` is used as the name suffix for the Flux objects,
and is also used to compose the Ingress host name where the app can be accessed.

The latest commit SHA pushed to the PR HEAD is passed as `<< inputs.sha >>`,
the SHA is used to set the app image tag in the Helm release values.

The `change_request` and `commit` annotations are used by the Flux notification providers
to post comments and commit statuses on the Pull Request. The preview URL, branch name
and author are set as extra metadata to enrich the notifications that the dev team receives.

To verify the ResourceSet templates are valid, we can use the
[Flux Operator CLI](cli.md) and build them locally:

```shell
flux-operator build resourceset -f app-resourceset.yaml \
  --inputs-from test-inputs.yaml
```

The `test-inputs.yaml` file should contain mock PR data e.g.:

```yaml
   - author: test
     branch: feat/test
     id: "1"
     sha: bf5d6e01cf802734853f6f3417b237e3ad0ba35d
     title: 'testing'
```

### Notifications

To receive notifications when a PR triggers a Helm release install,
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

### Status reporting on Pull Requests

To notify the developers of the preview deployment status directly on the Pull Request page,
we can use the Flux notification providers for
[GitHub PR comments](https://fluxcd.io/flux/components/notification/providers/#github-pull-request-comment)
and [GitHub commit statuses](https://fluxcd.io/flux/components/notification/providers/#github).

The `githubpullrequestcomment` provider posts a comment on the PR page with the deployment status
and metadata. The comment is automatically updated on subsequent events, so it doesn't spam the PR
with multiple comments. The `github` commit status provider posts a status check on the commit
that triggered the deployment.

The PR comment provider requires the `change_request` annotation and the commit status provider
requires the `commit` annotation, both set on the HelmRelease as shown in the
[ResourceSet template](#resourceset-template) above.

```yaml
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: github-pr-comment
  namespace: app-preview
spec:
  type: githubpullrequestcomment
  address: https://github.com/org/app
  secretRef:
    name: github-auth
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: github-pr-comment
  namespace: app-preview
spec:
  providerRef:
    name: github-pr-comment
  eventSeverity: info
  eventSources:
    - kind: HelmRelease
      name: '*'
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: github-commit-status
  namespace: app-preview
spec:
  type: github
  address: https://github.com/org/app
  secretRef:
    name: github-auth
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: github-commit-status
  namespace: app-preview
spec:
  providerRef:
    name: github-commit-status
  eventSeverity: info
  eventSources:
    - kind: HelmRelease
      name: '*'
```

Every time a commit is pushed to the PR branch, the Flux Operator will upgrade the Helm release
and will update the PR comment and commit status with the latest deployment status.

## GitHub Workflow

To automate the build and push of the app container image to GitHub Container Registry,
the GitHub Actions workflow should include the following steps:

```yaml
name: push-image-preview
on:
  pull_request:
    branches: ['main']
jobs:
  docker:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Generate image metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: |
            ghcr.io/${{ github.repository }}
          tags: |
            type=raw,value=${{ github.event.pull_request.head.sha }}
      - name: Build and push image
        uses: docker/build-push-action@v6
        with:
          push: true
          context: .
          file: ./Dockerfile
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

Note that we tag the container image with `${{ github.event.pull_request.head.sha }}`.
This ensures that the image tag matches the commit SHA of the PR HEAD that the ResourceSet
uses to deploy the app.

### Delay updates if the build takes too long

If your GitHub Workflow takes too long to build artifacts, e.g. more than 10 minutes,
you may want to keep the previous commit SHA in the ResourceSet until the new SHA is
completely built by your workflow. In order to do that you can use a Flux `Receiver`
of the type `generic` instead of `github` to trigger the reconciliation of the
`ResourceSetInputProvider`:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: github-receiver
  namespace: app-preview
spec:
  type: generic
  secretRef:
    name: receiver-token
  resources:
    - apiVersion: fluxcd.controlplane.io/v1
      kind: ResourceSetInputProvider
      name: app-pull-requests
```

This is necessary because the Flux webhook must be called only at the end of the
GitHub Workflow, so make sure to store the webhook URL as a secret in your GitHub
repository, e.g. `FLUX_RECEIVER_WEBHOOK`.

You also need to create a label in your GitHub repository to tell the
`ResourceSetInputProvider` to skip updating the exported inputs for the
pull request when this label is present, e.g. `deploy/flux-preview-pause`.
This label will be dynamically added and removed by the GitHub Workflow
that builds the artifacts. In your `ResourceSetInputProvider` add the
following configuration:

```yaml
...
spec:
  skip:
    labels:
      - "deploy/flux-preview-pause"
...
```

Finally, add the following parts to the job of your [GitHub Workflow](#github-workflow):

```yaml
...
    permissions:
      pull-requests: write # for adding/removing labels to the pull request
...
      # Add the following immediately after the checkout step (checkout must always be the first):
      - name: Add label to prevent ResourceSetInputProvider from updating
        run: gh pr edit $PR_NUMBER --add-label deploy/flux-preview-pause
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          PR_NUMBER: ${{ github.event.pull_request.number }}
...
      # Add the following at the end of the job:
      - name: Remove label to allow ResourceSetInputProvider to update
        run: gh pr edit $PR_NUMBER --remove-label deploy/flux-preview-pause
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          PR_NUMBER: ${{ github.event.pull_request.number }}
      - name: Trigger Flux Receiver
        run: curl -X POST $FLUX_RECEIVER_WEBHOOK
        env:
          FLUX_RECEIVER_WEBHOOK: ${{ secrets.FLUX_RECEIVER_WEBHOOK }}
```

There's still a chance that Flux Operator will reconcile your `ResourceSetInputProvider`
between the moment when you make a Git push and the moment when the GitHub Workflow adds
the pause label, this will cause your ephemeral environment to be updated with the new
SHA before the artifacts are built, but this is unlikely to happen if your GitHub Workflows
are quickly scheduled on runners.

## Further reading

To learn more about ResourceSets and the various configuration options, see the following docs:

- [ResourceSet API reference](resourceset.md)
- [ResourceSetInputProvider API reference](resourcesetinputprovider.md)

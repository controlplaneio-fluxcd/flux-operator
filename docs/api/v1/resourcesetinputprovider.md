---
title: ResourceSetInputProvider CRD
description: Flux Operator API for generating input values for ResourceSets
---

# ResourceSetInputProvider CRD

**ResourceSetInputProvider** is a declarative API for generating a set of input values
for use within [ResourceSet](resourceset.md) definitions. The input values are fetched from external
services such as GitHub or GitLab, and can be used to parameterize the resources templates
defined in ResourceSets.

## Example

The following example shows a provider that fetches input values from
GitHub Pull Requests labeled with `deploy/flux-preview`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: flux-appx-prs
  namespace: default
  annotations:
    fluxcd.controlplane.io/reconcile: "enabled"
    fluxcd.controlplane.io/reconcileEvery: "5m"
spec:
  type: GitHubPullRequest
  url: https://github.com/controlplaneio-fluxcd/flux-appx
  filter:
    labels:
      - "deploy/flux-preview"
  defaultValues:
    chart: "charts/flux-appx"
```

You can run this example by saving the manifest into `flux-appx-prs.yaml`.

**1.** Apply the ResourceSetInputProvider on the cluster:

```shell
kubectl apply -f flux-appx-prs.yaml.yaml
```

**2.** Wait for the ResourceSetInputProvider to reconcile:

```shell
kubectl wait rsip/flux-appx-prs --for=condition=ready --timeout=5m
```

**3.** Run `kubectl get -o yaml` to see the exported inputs generated in the ResourceSetInputProvider status:

```console
$ kubectl get rsip/flux-appx-prs -o yaml | yq .status.exportedInputs
- author: stefanprodan
  branch: kubernetes/helm-set-limits
  chart: charts/flux-appx
  id: "4"
  sha: bf5d6e01cf802734853f6f3417b237e3ad0ba35d
  title: 'kubernetes(helm): Add default resources limits'
- author: stefanprodan
  branch: feat/ui-footer
  chart: charts/flux-appx
  id: "3"
  sha: 8492c0b5b2094fe720776c8ace1b9690ff258f53
  title: 'feat(ui): Add footer'
- author: stefanprodan
  branch: feat/ui-color-scheme
  chart: charts/flux-appx
  id: "2"
  sha: 8166bdecd6b078b9e5dd14fa3b7b67a847f76893
  title: 'feat(ui): Default color scheme'
```

**4.** Run `kubectl delete` to remove the provider from the cluster:

```shell
kubectl delete rsip/flux-appx-prs
```

## Writing a ResourceSetInputProvider spec

As with all other Kubernetes config, a ResourceSet needs `apiVersion`,
`kind`, `metadata.name` and `metadata.namespace` fields.
The name of a ResourceSet object must be a valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).
A ResourceSet also needs a [`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Type

The `.spec.type` field is required and specifies the type of the provider.

The following types are supported:

- `Static`: exports a single input map with the values from the field `.spec.defaultValues`.
- `GitHubPullRequest`: fetches input values from opened GitHub Pull Requests.
- `GitHubBranch`: fetches input values from GitHub repository branches.
- `GitLabMergeRequest`: fetches input values from opened GitLab Merge Requests.
- `GitLabBranch`: fetches input values from GitLab project branches.

For the `Static` type, the flux-operator will export in `.status.exportedInputs` a
single input map with the values from the field `.spec.defaultValues` and the
additional value:

- `id`: the Adler-32 checksum of the ResourceSetInputProvider UID (type string).

For all non-static types, the flux-operator will export in `.status.exportedInputs` a
set of input values for each Pull/Merge Request or Branch
that matches the [filter](#filter) criteria.

For Pull/Merge Requests the [exported inputs](#exported-inputs-status) structure is:

- `id`: the ID number of the PR/MR (type string).
- `sha`: the commit SHA of the PR/MR (type string).
- `branch`: the branch name of the PR/MR (type string).
- `author`: the author username of the PR/MR (type string).
- `title`: the title of the PR/MR (type string).

For Git Branches the [exported inputs](#exported-inputs-status) structure is:

- `id`: the Adler-32 checksum of the branch name (type string).
- `branch`: the branch name (type string).
- `sha`: the commit SHA corresponding to the branch HEAD (type string).

### URL

The `.spec.url` field is required and specifies the HTTP/S URL of the provider.
For Git services, the URL should contain the GitHub repository or the GitLab project address.

### Filter

The `.spec.filter` field is optional and specifies the filter criteria for the input values.

The following filters are supported:

- `limit`: limit the number of input values fetched (default is 100).
- `labels`: filter GitHub Pull Requests or GitLab Merge Requests by labels.
- `includeBranch`: regular expression to include branches by name.
- `excludeBranch`: regular expression to exclude branches by name.

Example of a filter configuration for GitLab Merge Requests:

```yaml
spec:
  filter:
    limit: 10
    labels:
      - "deploy::flux-preview"
    includeBranch: "^feat/.*"
    excludeBranch: "^feat/not-this-one$"
```

### Skip

The `.spec.skip` field is optional and specifies the skip criteria for skipping input updates.
This field can be used to wait until certain PR/MR is ready.

The following skips are supported:

- `labels`: skip input update by labels if one of the label matched. When the label starts with `!` it will skip if the label is not present.

Example of a skip configuration:

```yaml
spec:
  filter:
    labels:
      - "deploy:flux-preview"
  skip:
    labels:
      - "deploy/flux-preview-pause"
      - "!test-build-push/passed"
```

### Default values

The `.spec.defaultValues` field is optional and specifies the default values for the exported inputs.
This field can be used to set values that are common to all the exported inputs.

Example:

```yaml
spec:
  defaultValues:
    env: "staging"
    tenants:
      - "tenant1"
      - "tenant2"
```

### Authentication configuration

The `.spec.secretRef` field is optional and specifies the Kubernetes Secret containing
the authentication credentials used for connecting to the external service.
Note that the secret must be created in the same namespace as the ResourceSetInputProvider.

For Git services, the secret should contain the `username` and `password` keys, with the password
set to a personal access token that grants access for listing Pull Requests or Merge Requests
and Git branches.

Example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-pat
  namespace: default
stringData:
  username: flux
  password: <GITHUB PAT>
```

Example secret reference:

```yaml
spec:
  secretRef:
    name: github-pat
```

#### GitHub App authentication

For GitHub, GitHub App authentication is also supported. Instead of adding the basic
auth keys `username` and `password`, you can add the following GitHub App keys to the
secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-app
  namespace: default
stringData:
  githubAppID: "<GITHUB APP ID>"
  githubAppInstallationID: "<GITHUB APP INSTALLATION ID>"
  githubAppBaseURL: <github-enterprise-api-url> # optional, for self-hosted GitHub Enterprise
  githubAppPrivateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
```

Example secret reference:

```yaml
spec:
  secretRef:
    name: github-app
```

The GitHub App ID and Installation ID are integer numbers, so remember to quote them in the secret
if using the `stringData` field as all values in this field must be strings.

A simpler alternative is creating the secret using the Flux CLI command `flux create secret githubapp`.

### TLS certificate configuration

The `.spec.certSecretRef` field is optional and specifies the Kubernetes Secret containing the
TLS certificate used for connecting to the external service.
Note that the secret must be created in the same namespace as the ResourceSetInputProvider.

For Git services that use self-signed certificates, the secret should contain the `ca.crt` key.

Example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitlab-ca
  namespace: default
stringData:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    MIIDpDCCAoygAwIBAgIUI7z
    ...
    -----END CERTIFICATE-----
```

Example certificate reference:

```yaml
spec:
  certSecretRef:
    name: gitlab-ca
```

### Reconciliation configuration

The reconciliation of behaviour of a ResourceSet can be configured using the following annotations:

- `fluxcd.controlplane.io/reconcile`: Enable or disable the reconciliation loop. Default is `enabled`, set to `disabled` to pause the reconciliation.
- `fluxcd.controlplane.io/reconcileEvery`: Set the reconciliation interval used for calling external services. Default is `10m`.
- `fluxcd.controlplane.io/reconcileTimeout`: Set the timeout for calling external services. Default is `1m`.

## ResourceSetInputProvider Status

### Conditions

A ResourceSetInputProvider enters various states during its lifecycle, reflected as Kubernetes Conditions.
It can be [reconciling](#reconciling-fluxinstance) while fetching data from external services,
it can be [ready](#ready-fluxinstance), or it can [fail during reconciliation](#failed-fluxinstance).

The ResourceSetInputProvider API is compatible with the **kstatus** specification,
and reports `Reconciling` and `Stalled` conditions where applicable to
provide better (timeout) support to solutions polling the ResourceSetInputProvider to
become `Ready`.

#### Reconciling ResourceSetInputProvider

The flux-operator marks a ResourceSetInputProvider as _reconciling_ when it starts
the reconciliation of the same. The Condition added to the ResourceSetInputProvider's
`.status.conditions` has the following attributes:

- `type: Reconciling`
- `status: "True"`
- `reason: Progressing` | `reason: ProgressingWithRetry`

The Condition `message` is updated during the course of the reconciliation to
report the action being performed at any particular moment such as
fetching data from external services.

The `Ready` Condition's `status` is also marked as `Unknown`.

#### Ready ResourceSetInputProvider

The flux-operator marks a ResourceSetInputProvider as _ready_ when the
data fetching from external services is successful.

When the ResourceSet is "ready", the flux-operator sets a Condition with the
following attributes in the ResourceSet’s `.status.conditions`:

- `type: Ready`
- `status: "True"`
- `reason: ReconciliationSucceeded`

#### Failed ResourceSetInputProvider

The flux-operator may get stuck trying to reconcile and apply a
ResourceSetInputProvider without completing. This can occur due to some of the following factors:

- The authentication to the external service fails.
- The external service is unreachable.
- The data fetched from the external service is invalid.

When this happens, the flux-operator sets the `Ready` Condition status to False
and adds a Condition with the following attributes to the ResourceSet’s
`.status.conditions`:

- `type: Ready`
- `status: "False"`
- `reason: ReconciliationFailed`

The `message` field of the Condition will contain more information about why
the reconciliation failed.

While the ResourceSetInputProvider has one or more of these Conditions, the flux-operator
will continue to attempt a reconciliation with an
exponential backoff, until it succeeds and the ResourceSetInputProvider is marked as [ready](#ready-fluxinstance).

### Exported inputs status

After a successful reconciliation, the ResourceSetInputProvider status contains a list of exported inputs
that can be used in the ResourceSet templates.

Example:

```yaml
status:
  exportedInputs:
  - author: stefanprodan
    branch: kubernetes/helm-set-limits
    id: "4"
    sha: bf5d6e01cf802734853f6f3417b237e3ad0ba35d
    title: 'kubernetes(helm): Add default resources limits'
  - author: stefanprodan
    branch: feat/ui-footer
    id: "3"
    sha: 8492c0b5b2094fe720776c8ace1b9690ff258f53
    title: 'feat(ui): Add footer'
  - author: stefanprodan
    branch: feat/ui-color-scheme
    id: "2"
    sha: 8166bdecd6b078b9e5dd14fa3b7b67a847f76893
    title: 'feat(ui): Default color scheme'
```

## ResourceSetInputProvider Metrics

The Flux Operator exports Prometheus metrics for the ResourceSetInputProvider objects
that can be used to monitor the reconciliation status.

Metrics:

```text
flux_resourcesetinputprovider_info{uid, kind, name, exported_namespace, ready, suspended, url}
```

Labels:

- `uid`: The Kubernetes unique identifier of the resource.
- `kind`: The kind of the resource (e.g. `ResourceSetInputProvider`).
- `name`: The name of the resource (e.g. `podinfo-prs`).
- `exported_namespace`: The namespace where the resource is deployed (e.g. `podinfo-review`).
- `ready`: The readiness status of the resource (e.g. `True`, `False` or `Unkown`).
- `reason`: The reason for the readiness status (e.g. `ReconciliationSucceeded` or `ReconciliationFailed`).
- `suspended`: The suspended status of the resource (e.g. `True` or `False`).
- `url`: The provider address (e.g. `https://github.com/stefanprodan/podinfo`).

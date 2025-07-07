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
- `GitHubTag`: fetches input values from GitHub repository tags.
- `GitLabMergeRequest`: fetches input values from opened GitLab Merge Requests.
- `GitLabBranch`: fetches input values from GitLab project branches.
- `GitLabTag`: fetches input values from GitLab project tags.
- `AzureDevOpsPullRequest`: fetches input values from opened Azure DevOps Pull Requests.
- `AzureDevOpsBranch`: fetches input values from Azure DevOps repository branches.
- `AzureDevOpsTag`: fetches input values from AzureDevOps project tags.
- `OCIArtifactTag`: fetches input values from OCI artifact tags from generic container registries.
- `ACRArtifactTag`: fetches input values from Azure Container Registry OCI artifact tags.
- `ECRArtifactTag`: fetches input values from Elastic Container Registry OCI artifact tags.
- `GARArtifactTag`: fetches input values from Google Artifact Registry OCI artifact tags.

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

For Git Tags the [exported inputs](#exported-inputs-status) structure is:

- `id`: the Adler-32 checksum of the tag name (type string).
- `tag`: the tag name (type string).
- `sha`: the commit SHA corresponding to the tag in the format `<hash>` (type string).

For OCI Artifact Tags the [exported inputs](#exported-inputs-status) structure is:

- `id`: the Adler-32 checksum of the tag name (type string).
- `tag`: the tag name (type string).
- `digest`: the SHA256 digest corresponding to the tag in the format `sha256:<hash>` (type string).

The ACR, ECR and GAR Artifact Tag providers export the same inputs as the OCI Artifact Tag provider,
with the difference on the authentication method used to connect to the registry. For these providers,
[secret-less](#secret-less) authentication is used.

### URL

The `.spec.url` field is required for external providers.
For Git services, the URL should contain GitHub repository or the GitLab project address,
including the HTTP/S scheme (`(http|https)://`).
For OCI services, the URL should contain the OCI repository address,
including the OCI scheme (`oci://`).

### Filter

The `.spec.filter` field is optional and specifies the filter criteria for the input values.

The following filters are supported:

- `limit`: limit the number of input values fetched (default is 100).
- `labels`: filter GitHub Pull Requests or GitLab Merge Requests by labels.
- `includeBranch`: regular expression to include branches by name.
- `excludeBranch`: regular expression to exclude branches by name.
- `includeTag`: regular expression to include tags by name.
- `excludeTag`: regular expression to exclude tags by name.
- `semver`: sematic version range to filter and sort tags.

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

Example of a filter configuration for filtering tags by inclusion and exclusion:

```yaml
spec:
  filter:
    includeTag: "^v[0-9]+\\.[0-9]+\\.[0-9]+$" # include tags like v1.2.3
    excludeTag: "^v0" # exclude tags like v0.1.0
```

Example of a filter configuration for fetching only the latest tag according to semver:

```yaml
spec:
  filter:
    limit: 1
    semver: ">=1.0.0"
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

#### Secret-based

The `.spec.secretRef` field is optional and specifies the Kubernetes Secret containing
the authentication credentials used for connecting to the external service.
Note that the secret must be created in the same namespace as the ResourceSetInputProvider.

This field is not supported by the following provider [types](#type):

- `Static`
- `ACRArtifactTag`
- `ECRArtifactTag`
- `GARArtifactTag`

For Git services, the secret should contain the `username` and `password` keys, with the password
set to a personal access token that grants access for listing Pull Requests or Merge Requests
and Git branches.

For the `OCIArtifactTag` provider [type](#type), the secret should contain a Kubernetes Docker
config JSON secret, i.e. as if created by the `kubectl create secret docker-registry` command.
If the `.spec.serviceAccountName` field is specified, all the image pull secrets configured on
the ServiceAccount are also included in the registry keychain used for the reconciliation,
alongside the secret specified in `.spec.secretRef`. All of them have to be on the format
produced by the `kubectl create secret docker-registry` command.

Example of Git secret:

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

Example of `OCIArtifactTag` Docker config secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-config
  namespace: default
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: |
    {
      "auths": {
        "<REGISTRY HOST>": {
          "username": "flux",
          "password": "<PASSWORD>"
        }
      }
    }
```

Example secret reference:

```yaml
spec:
  serviceAccountName: oci-pull-secrets-sa # optional, for OCIArtifactTag provider type only
  secretRef:
    name: github-pat # or oci-pull-secret for OCIArtifactTag provider type
```

#### Secret-less

The `.spec.serviceAccountName` field is optional and specifies the name of the Kubernetes
ServiceAccount in the same namespace configured with workload identity to access a cloud
provider service (this is called *object-level workload identity*). This field can only
be used with the following provider [types](#type) for workload identity:

- `AzureDevOpsPullRequest`
- `AzureDevOpsBranch`
- `AzureDevOpsTag`
- `ACRArtifactTag`
- `ECRArtifactTag`
- `GARArtifactTag`

When this field is not present and one of the types above is specified (and in the case of Azure
DevOps, [`.spec.secretRef`](#secret-reference) is also not specified), the operator will attempt
to authenticate using the environment credentials, i.e. either the identity of the node or the
operator ServiceAccount. This is called *controller-level workload identity*.

For configuring a Kubernetes ServiceAccount with workload identity, see the following documentation:

- [Azure](https://fluxcd.io/flux/integrations/azure/#with-workload-identity-federation)
- [AWS (controller-level)](https://fluxcd.io/flux/integrations/aws/#with-eks-pod-identity)
- [AWS (object-level)](https://fluxcd.io/flux/integrations/aws/#with-oidc-federation)
- [GCP](https://fluxcd.io/flux/integrations/gcp/#with-workload-identity-federation)

For configuring the required permissions to access the cloud services, see the following documentation:

- [Azure DevOps](https://fluxcd.io/flux/integrations/azure/#for-azure-devops) (the `Readers` ADO group is sufficient)
- [Azure Container Registry](https://fluxcd.io/flux/integrations/azure/#for-azure-container-registry)
- [Amazon Elastic Container Registry](https://fluxcd.io/flux/integrations/aws/#for-amazon-elastic-container-registry)
- [Amazon Elastic Container Registry Public](https://fluxcd.io/flux/integrations/aws/#for-amazon-public-elastic-container-registry)
- [Google Artifact Registry](https://fluxcd.io/flux/integrations/gcp/#for-google-cloud-artifact-registry)

For configuring the operator to use controller-level workload identity, patches like
the ones described in the documentation below can be applied to the operator deployment:

- [Azure](https://fluxcd.io/flux/integrations/azure/#at-the-controller-level)
- [AWS](https://fluxcd.io/flux/integrations/aws/#at-the-controller-level)
- [GCP](https://fluxcd.io/flux/integrations/gcp/#at-the-controller-level)

For configuring the identity of your nodes to access container registry services, see the
following documentation:

- [Authenticate with ACR from AKS](https://learn.microsoft.com/en-us/azure/aks/cluster-container-registry-integration?tabs=azure-cli)
- Authenticate with ECR from EKS:
  - [Create IAM Roles for EKS worker nodes](https://docs.aws.amazon.com/eks/latest/userguide/create-node-role.html#create-worker-node-role)
  - [Allow the EKS worker IAM Roles to pull images from ECR](https://docs.aws.amazon.com/AmazonECR/latest/userguide/ECR_on_EKS.html)
- [Authenticate with GAR from GKE](https://cloud.google.com/artifact-registry/docs/integrate-gke)

See also the cross-cloud documentation:

- [Cross-cloud support](https://fluxcd.io/flux/integrations/cross-cloud/)

#### GitHub App authentication

For GitHub, GitHub App authentication is also supported. Instead of adding the basic
auth keys `username` and `password` to the referenced Secret, you can add the following
GitHub App keys:

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

This field is not supported by the following provider [types](#type):

- `Static`
- `ACRArtifactTag`
- `ECRArtifactTag`
- `GARArtifactTag`

For Git services that use self-signed certificates, the secret should contain the `ca.crt` key.

For the `OCIArtifactTag` provider [type](#type), the secret should contain either or both of
the `ca.crt` key with a CA certificate, and the pair `tls.crt` and `tls.key` keys with an
mTLS client certificate and key.

Example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: provider-certs
  namespace: default
stringData:
  ca.crt: | # supported for both Git and OCIArtifactTag providers
    -----BEGIN CERTIFICATE-----
    MIIDpDCCAoygAwIBAgIUI7z
    ...
    -----END CERTIFICATE-----
  tls.crt: | # supported only for the OCIArtifactTag provider
    -----BEGIN CERTIFICATE-----
    MIIDpDCCAoygIUI7zgAwIBA
    ...
    -----END CERTIFICATE-----
  tls.key: | # supported only for the OCIArtifactTag provider
    -----BEGIN PRIVATE KEY-----
    MIIEvQIBADABAQCv1qlHtnk
    ...
    -----END PRIVATE KEY-----
```

Example certificate reference:

```yaml
spec:
  certSecretRef:
    name: provider-certs
```

### Schedule

The `.spec.schedule` field is optional and can be used to specify a list of `Schedule` objects.

Each `Schedule` object has the following fields:

- `.cron`: a required string representing the cron schedule in the format accepted by
  [cron](https://crontab.guru/).
- `.timeZone`: a string representing the [time zone](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)
  in which the cron schedule should be interpreted. This field is optional and defaults to `UTC`.
- `.window`: an optional string representing the time window duration in which reconciliations are
  allowed to run. The format is a Go duration string, such as `1h30m` or `2h45m`. Defaults to `0s`,
  meaning no window is applied. The duration must be either zero or at least twice the
  [timeout](#reconciliation-configuration).

Example:

```yaml
spec:
  schedule:
    # Every day-of-week from Monday through Thursday
    # between 10:00 to 16:00
    - cron: "0 10 * * 1-4"
      timeZone: "Europe/London"
      window: "6h"
    # Every Friday from 10:00 to 13:00
    - cron: "0 10 * * 5"
      timeZone: "Europe/London"
      window: "3h"
```

When multiple schedules are specified, flux-operator will:

- Reconcile the ResourceSetInputProvider if at least one of the schedules matches the current time.
- Use the earliest next scheduled time across all schedules to determine the next scheduled time.

When a schedule is specified with `window: 0s`, flux-operator will make the best effort to reconcile
the ResourceSetInputProvider at the scheduled time; but it may not be able to guarantee that the
reconciliation will start exactly at that time, especially if the operator is too busy or if
the cluster is under heavy load.

If multiple schedules are specified and at least one has a zero-duration window,
flux-operator will always reconcile the ResourceSetInputProvider upon any requests, e.g.,
updating the `.spec`, when the CLI is used to trigger a reconciliation, or when the
operator is restarted.

When a schedule is specified with a non-zero duration window, flux-operator will only reconcile
the ResourceSetInputProvider when the time point `time.Now().Add(obj.GetTimeout())` falls within
the time window defined by the schedule. This check is performed not only at the start of the
reconciliation, but also when scheduling the next reconciliation according to the interval defined
by the `fluxcd.controlplane.io/reconcileEvery` [annotation](#reconciliation-configuration).

To force a one-off out-of-schedule reconciliation, annotate the ResourceSetInputProvider with
`reconcile.fluxcd.io/requestedAt: "<current timestamp>"` and
`reconcile.fluxcd.io/forceAt: "<current timestamp>"`. The value of both annotations must be
the exact same string, otherwise the reconciliation will not be triggered. This can be done
easily using the `flux-operator` CLI:

```shell
flux-operator reconcile inputprovider <name> --force
```

If an out-of-schedule reconciliation is triggered without being forced, the
flux-operator will only print an info log and and emit a `Normal` event with
the reason `SkippedDueToSchedule` to indicate that the reconciliation was
skipped due to the schedule configuration. The [`.status.conditions`](#conditions)
will not be updated in this case, and the ResourceSetInputProvider will not be
reconciled until the next scheduled time.

### Reconciliation configuration

The reconciliation of behaviour of a ResourceSet can be configured using the following annotations:

- `fluxcd.controlplane.io/reconcile`: Enable or disable the reconciliation loop. Default is `enabled`, set to `disabled` to pause the reconciliation.
- `fluxcd.controlplane.io/reconcileEvery`: Set the reconciliation interval used for calling external services. Default is `10m`.
- `fluxcd.controlplane.io/reconcileTimeout`: Set the timeout for calling external services. Default is `2m`.

## ResourceSetInputProvider Status

### Conditions

A ResourceSetInputProvider enters various states during its lifecycle, reflected as Kubernetes Conditions.
It can be [reconciling](#reconciling-resourcesetinputprovider) while fetching data from external services,
it can be [ready](#ready-resourcesetinputprovider),
it can [fail during reconciliation](#failed-resourcesetinputprovider),
or it can [fail due to misconfiguration](#stalled-resourcesetinputprovider).

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
exponential backoff, until it succeeds and the ResourceSetInputProvider is marked as
[ready](#ready-resourcesetinputprovider).

#### Stalled ResourceSetInputProvider

The flux-operator may fail the reconciliation of a ResourceSetInputProvider object terminally due
to a misconfiguration. When this happens, the flux-operator adds the `Stalled` Condition to the
ResourceSetInputProvider’s `.status.conditions` with the following attributes:

- `type: Stalled`
- `status: "True"`
- `reason: InvalidDefaultValues | InvalidSchedule | InvalidExportedInputs`

Misconfigurations can include:

- The `.spec.defaultValues` has invalid values. In this case the condition reason is `InvalidDefaultValues`.
- The `.spec.schedule` has invalid configuration. In this case the condition reason is `InvalidSchedule`.
- For the `Static` provider type only, the default values can be parsed but cannot be exported as inputs. In this case the condition reason is `InvalidExportedInputs`.

When this happens, the flux-operator will not attempt to reconcile the ResourceSetInputProvider
until the misconfiguration is fixed. The `Ready` Condition status is also set to `False`.

### Exported inputs status

After a successful reconciliation, the ResourceSetInputProvider status contains a list of exported inputs
that can be used in the ResourceSet templates.

Example for GitHub Pull Request:

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

Example for GitHub latest semver tag:

```yaml
status:
  exportedInputs:
  - id: "48955639"
    tag: "6.0.4"
    sha: 11cf36d83818e64aaa60d523ab6438258ebb6009
```

Example for OCI latest semver tag:

```yaml
status:
  exportedInputs:
  - id: "48955639"
    tag: "6.0.4"
    sha: sha256:d4ec9861522d4961b2acac5a070ef4f92d732480dff2062c2f3a1dcf9a5d1e91
```

### Schedule status

When the next reconciliation of the ResourceSetInputProvider is due to a schedule
the field `.status.nextSchedule` holds information about the next scheduled
reconciliation. For example:

```yaml
status:
  nextSchedule:
    cron: 0 8 * * 1-5
    timeZone: Europe/London
    when: "2025-06-29T00:00:00Z"
    window: 8h
```

During a window, the flux-operator will schedule reconciliations based on the
interval defined by the `fluxcd.controlplane.io/reconcileEvery` annotation
(or its default value). During this time, the `.status.nextSchedule` field
will not be present.

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

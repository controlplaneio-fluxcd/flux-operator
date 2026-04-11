# Install Flux with Terraform

This example demonstrates how to deploy Flux on a Kubernetes cluster using Terraform
and the [flux-operator-bootstrap](https://github.com/controlplaneio-fluxcd/terraform-kubernetes-flux-operator-bootstrap)
module.

## Usage

Create a Kubernetes cluster using KinD:

```shell
kind create cluster --name staging
```

Install the Flux Operator and deploy the Flux instance on the cluster 
set as the default context in the `~/.kube/config` file:

```shell
terraform apply \
  -var cluster_name="staging" \
  -var cluster_region="eu-west-2"
```

To authenticate with a private GitHub repository using a GitHub App:

```shell
export GITHUB_APP_PEM=`cat path/to/app.private-key.pem`

terraform apply \
  -var cluster_name="staging" \
  -var cluster_region="eu-west-2" \
  -var github_app_id="1" \
  -var github_app_installation_owner="org" \
  -var github_app_pem="$GITHUB_APP_PEM"
```

To authenticate with a private repository using a Git PAT (e.g. for GitLab):

```shell
terraform apply \
  -var cluster_name="staging" \
  -var cluster_region="eu-west-2" \
  -var git_token="${GIT_TOKEN}"
```

Verify the Flux components are running:

```shell
kubectl -n flux-system get pods
```

Verify the Flux instance is syncing the cluster state from the Git repository:

```shell
kubectl -n flux-system get fluxreport/flux -o yaml
```

The output should show the sync status:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxReport
metadata:
  name: flux
  namespace: flux-system
spec:
  # Distribution status omitted for brevity
  sync:
    id: kustomization/flux-system
    path: config/terraform/clusters/staging
    ready: true
    source: https://github.com/controlplaneio-fluxcd/flux-operator.git
    status: 'Applied revision: refs/heads/main@sha1:21486401be9bcdc37e6ebda48a3b68f8350777c9'
```

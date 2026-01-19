# Install Flux with Terraform

This example demonstrates how to deploy Flux on a Kubernetes cluster using Terraform
and the `flux-operator` and `flux-instance` Helm charts.

## Usage

Create a Kubernetes cluster using KinD:

```shell
kind create cluster --name flux
```

Install the Flux Operator and deploy the Flux instance on the cluster 
set as the default context in the `~/.kube/config` file:

```shell
terraform apply \
  -var flux_version="2.x" \
  -var flux_registry="ghcr.io/fluxcd" \
  -var git_token="${GITHUB_TOKEN}" \
  -var git_url="https://github.com/fluxcd/flux2-kustomize-helm-example.git" \
  -var git_ref="refs/heads/main" \
  -var git_path="clusters/production"
```

Note that the `GITHUB_TOKEN` env var must be set to a GitHub personal access token.
The `git_token` variable is used to create a Kubernetes secret in the `flux-system` namespace for
Flux to authenticate with the Git repository over HTTPS.
If the repository is public, the token variable can be omitted.

Alternatively, you can use a GitHub App to authenticate with a GitHub repository:

```shell
export GITHUB_APP_PEM=`cat path/to/app.private-key.pem`

terraform apply \
  -var flux_version="2.x" \
  -var flux_registry="ghcr.io/fluxcd" \
  -var github_app_id="1" \
  -var github_app_installation_owner="org" \
  -var github_app_pem="$GITHUB_APP_PEM" \
  -var git_url="https://github.com/org/repo.git" \
  -var git_ref="refs/heads/main" \
  -var git_path="clusters/production"
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
    path: clusters/production
    ready: true
    source: https://github.com/fluxcd/flux2-kustomize-helm-example.git
    status: 'Applied revision: refs/heads/main@sha1:21486401be9bcdc37e6ebda48a3b68f8350777c9'
```

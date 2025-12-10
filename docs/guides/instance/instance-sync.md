---
title: Flux Cluster Sync Configuration
description: Flux Operator configuration guide for cluster synchronization
---

# Flux Cluster Sync Configuration

The `FluxInstance` resource can be configured to instruct the operator to generate
a Flux source (`GitRepository`, `OCIRepository` or `Bucket`) and a Flux `Kustomization`
to sync the cluster state with the source repository.

The Flux objects are created in the same namespace where the `FluxInstance` is deployed
using the namespace name as the Flux source and Kustomization name. The naming convention
matches the one used by `flux bootstrap` to ensure compatibility with upstream, and
to allow transitioning a bootstrapped cluster to a `FluxInstance` managed one.

## Sync from a Git Repository

To sync the cluster state from a Git repository, add the following configuration to the `FluxInstance`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
  sync:
    kind: GitRepository
    url: "https://gitlab.com/my-org/my-fleet.git"
    ref: "refs/heads/main"
    path: "clusters/my-cluster"
    pullSecret: "flux-system"
```

If the source repository is private, the Kubernetes secret must be created in the `flux-system` namespace
and should contain the credentials to clone the repository:

```shell
flux create secret git flux-system \
  --url=https://gitlab.com/my-org/my-fleet.git \
  --username=git \
  --password=$GITLAB_TOKEN
```

## Sync from a Git Repository using GitHub App auth

To sync the cluster state from a GitHub repository using GitHub App authentication:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - image-reflector-controller
    - image-automation-controller
  sync:
    kind: GitRepository
    provider: github
    url: "https://github.com/my-org/my-fleet.git"
    ref: "refs/heads/main"
    path: "clusters/my-cluster"
    pullSecret: "flux-system"
```

The Kubernetes secret must be created in the `flux-system` namespace
and should contain the GitHub App private key:

```shell
flux create secret githubapp flux-system \
  --app-id=1 \
  --app-installation-id=2 \
  --app-private-key=./path/to/private-key-file.pem
```

!!! tip "GitHub App Support"

    Note that GitHub App support was added in Flux v2.5.0 and Flux Operator v0.16.0.
    For more information on how to create a GitHub App see the
    Flux [GitRepository API reference](https://fluxcd.io/flux/components/source/gitrepositories/#github). 


## Sync from an Azure DevOps Repository using AKS Workload Identity

To sync the cluster state from Azure DevOps using AKS Workload Identity:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.7.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
  sync:
    kind: GitRepository
    provider: azure
    url: "https://dev.azure.com/my-org/_git/my-fleet"
    ref: "refs/heads/main"
    path: "clusters/my-cluster"
  kustomize:
    patches:
    - patch: |-
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: source-controller
          annotations:
            azure.workload.identity/client-id: <AZURE_CLIENT_ID>
            azure.workload.identity/tenant-id: <AZURE_TENANT_ID>
      target:
        kind: ServiceAccount
        name: source-controller
    - patch: |-
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: source-controller
        spec:
          template:
            metadata:
              labels:
                azure.workload.identity/use: "true" 
      target:
        kind: Deployment
        name: source-controller
```

!!! tip "Workload Identity Support"

    Note that Azure DevOps Workload Identity support was added in Flux v2.5.0 and Flux Operator v0.18.0.
    For more information on how to configure Azure DevOps Workload Identity see the
    Flux [GitRepository API reference](https://fluxcd.io/flux/components/source/gitrepositories/#azure). 

## Sync from a Container Registry

To sync the cluster state from a container registry where the Kubernetes manifests
are pushed as OCI artifacts using `flux push artifact`:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.x"
    registry: "ghcr.io/fluxcd"
  sync:
    kind: OCIRepository
    url: "oci://ghcr.io/my-org/my-fleet-manifests"
    ref: "latest"
    path: "clusters/my-cluster"
    pullSecret: "flux-system"
```

If the container registry is private, the Kubernetes secret must be created
in the same namespace where the `FluxInstance` is deployed,
and be of type `kubernetes.io/dockerconfigjson`:

```shell
flux create secret oci flux-system \
  --namespace flux-system \
  --url=ghcr.io \
  --username=flux \
  --password=$GITHUB_TOKEN
```

## Sync from a Container Registry using Workload Identity

To sync the cluster state from a managed container registry, for example, AWS ECR:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.x"
    registry: "ghcr.io/fluxcd"
  sync:
    kind: OCIRepository
    provider: aws
    url: "oci://<account>.dkr.ecr.<region>.amazonaws.com/fleet-manifests"
    ref: "latest"
    path: "clusters/my-cluster"
```

Note that you need to create an EKS Pod Identity association for the `source-controller`
Service Account to allow it to pull images from the ECR repository.

## Sync from a Bucket

To sync the cluster state from an S3 bucket where the Kubernetes manifests
are stored as YAML files:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: flux-system
spec:
  distribution:
    version: "2.x"
    registry: "ghcr.io/fluxcd"
  sync:
    kind: Bucket
    url: "minio.my-org.com"
    ref: "my-bucket-fleet"
    path: "clusters/my-cluster"
    pullSecret: "bucket-auth"
```

The Kubernetes secret must be created
in the same namespace where the FluxInstance is deployed, with the following keys:

```shell
kubectl create secret generic bucket-auth \
  --namespace flux-system \
  --from-literal=accesskey=my-accesskey \
  --from-literal=secretkey=my-secretkey
```

To find out more about the available configuration options, refer to the
[FluxInstance API reference](fluxinstance.md).

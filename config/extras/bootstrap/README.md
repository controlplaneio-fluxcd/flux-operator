# FluxInstance Bootstrap Job

This directory contains Kubernetes manifests for bootstrapping a [FluxInstance](https://fluxoperator.dev/docs/crd/fluxinstance/)
resource using a one-time Job. The Kubernetes Job is intended to run after the Flux Operator
has been installed, creating the initial instance that Flux will then manage itself.

## Contents

- `bootstrap.sh` - Shell script that applies the FluxInstance
- `bootstrap-job.yaml` - Kubernetes Job that runs the bootstrap script
- `flux-instance.yaml` - FluxInstance resource manifest
- `kustomization.yaml` - Kustomize configuration

Note that after the cluster is bootstrapped, the `flux-instance.yaml` file should be place under
Flux's source repository so that changes to the instance can be managed via GitOps.

## Usage

1. Edit `flux-instance.yaml` to configure your FluxInstance (source URL, path, etc.)

2. If using a private repository, create the credentials secret:

   ```bash
   kubectl -n flux-system create secret generic flux-git-auth \
     --from-literal=username=<git-username> \
     --from-literal=password=<git-token>
   ```

   Then add `pullSecret: flux-git-auth` in `flux-instance.yaml` under `spec.source`.

3. Apply the manifests:

   ```bash
   kubectl apply -k config/extras/bootstrap/
   ```

4. Monitor the Job:

   ```bash
   kubectl -n flux-system logs -f job/flux-bootstrap
   ```

## Behavior

- If the FluxInstance already exists, the Job exits successfully without changes.
- If the FluxInstance CRD is not yet available, the script retries applying the manifest.
- After applying, the script waits for the FluxInstance to become ready.
- The Job is automatically deleted 5 minutes after completion.

Note that the Kubernetes Job runs under the `flux-operator` Service Account.

## Script Options

The bootstrap script supports the following variables (edit `bootstrap.sh`):

| Variable        | Default | Description                                      |
|-----------------|---------|--------------------------------------------------|
| `MAX_RETRIES`   | 5       | Number of apply retry attempts                   |
| `RETRY_DELAY`   | 10      | Seconds between retries                          |
| `READY_TIMEOUT` | 10m     | Timeout waiting for FluxInstance to become ready |

## Runner Image

The bootstrap Job uses the [Flux CLI](https://github.com/fluxcd/flux2) image (`ghcr.io/fluxcd/flux-cli`),
which is based on Alpine Linux and includes both `kubectl` and a shell (`/bin/sh`).

To change the image tag, modify `kustomization.yaml`:

```yaml
images:
  - name: ghcr.io/fluxcd/flux-cli
    newTag: v2.7.5
```

To use a different kubectl image, you can replace it with any image that includes
both `kubectl` and a shell (`/bin/sh`). Update the image reference in `kustomization.yaml`:

```yaml
images:
  - name: ghcr.io/fluxcd/flux-cli
    newName: my.registry/kubectl
    newTag: 1.35.0
```

Note: The official `registry.k8s.io/kubectl` image is distroless and does not include a shell,
so it cannot be used with this bootstrap script.

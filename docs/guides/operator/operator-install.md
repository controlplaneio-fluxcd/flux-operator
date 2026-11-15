---
title: Installation
description: Installing Flux Operator on Kubernetes and OpenShift clusters
---

# Flux Operator Installation

The Flux Operator is designed to run in a Kubernetes cluster on Linux nodes (AMD64 or ARM64)
and is compatible with all major Kubernetes distributions. The operator is written in Go and
statically compiled as a single binary with no external dependencies.

## Install methods

The Flux Operator can be installed with Helm, Terraform, Operator Lifecycle Manager (OLM),
the `flux-operator` CLI, or `kubectl`.
It is recommended to install the operator in a dedicated namespace, such as `flux-system`.

### Helm

The Flux Operator can be installed using the
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator)
available in GitHub Container Registry:

```shell
helm install flux-operator oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator \
  --namespace flux-system \
  --create-namespace
```

### Terraform

Installing the Flux Operator with Terraform is possible using the
[flux-operator-bootstrap](https://github.com/controlplaneio-fluxcd/terraform-kubernetes-flux-operator-bootstrap)
module:

```hcl
module "flux_operator_bootstrap" {
  source  = "controlplaneio-fluxcd/flux-operator-bootstrap/kubernetes"

  revision = var.bootstrap_revision

  gitops_resources = {
    instance_yaml = file("${path.root}/clusters/staging/flux-system/flux-instance.yaml")
  }
}
```

For more information on how to configure the Flux instance with Terraform,
see the Flux Operator
[terraform example](https://github.com/controlplaneio-fluxcd/flux-operator/tree/main/config/terraform).

### Operator Lifecycle Manager

The Flux Operator can be installed on OpenShift using the bundle published on OperatorHub
at [operatorhub.io/operator/flux-operator](https://operatorhub.io/operator/flux-operator).

Example subscription manifest:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  channel: stable
  name: flux-operator
  source: operatorhubio-catalog
  sourceNamespace: olm
  config:
    env:
      - name: DEFAULT_SERVICE_ACCOUNT
        value: "flux-operator"
      - name: REPORTING_INTERVAL
        value: "30s"
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
            - matchExpressions:
                - key: role
                  operator: In
                  values:
                    - flux
    tolerations:
      - effect: NoSchedule
        key: role
        operator: Equal
        value: flux
```

The Flux Operator is also available in the OpenShift and OKD
[production-ready catalog](https://github.com/redhat-openshift-ecosystem/community-operators-prod).

!!! note "Environment Variables"
    
    Flux Operator supports various environment variables to customize its behavior on OpenShift.
    Please see the operator [configuration guide](operator-config.md#environment-variables) for more details.

### Flux Operator CLI

The Flux Operator can be installed using the
[flux-operator CLI](https://fluxoperator.dev/docs/guides/cli/),
which bootstraps a Kubernetes cluster with the Flux Operator and a Flux instance.

Install the CLI with Homebrew:

```shell
brew install controlplaneio-fluxcd/tap/flux-operator
```

Install the Flux Operator and a Flux instance from a configuration file:

```shell
flux-operator install -f flux-instance.yaml
```

### Kubectl

The Flux Operator can be installed with `kubectl` by
applying the Kubernetes manifests published on the releases page:

```shell
kubectl apply -f https://github.com/controlplaneio-fluxcd/flux-operator/releases/latest/download/install.yaml
```

!!! warning "Development and testing"

    This method is intended for development and testing purposes. On production environments,
    it is recommended to use [Helm](#helm) or [Terraform](#terraform).

## SPIFFE configuration

When the FluxInstance enables the SPIFFE Workload API or Broker API, the
flux-operator pod must be configured with the same endpoints. The operator
uses these APIs to download artifacts and send events.

These settings configure the flux-operator pod only. The Flux controllers are
configured through the FluxInstance `.spec.spiffe` field.

The operator does not need the `--authorized-*` inter-controller flags. It
reads the Flux controller SPIFFE identities from the FluxInstance.

### Helm

The Flux Operator Helm chart exposes the SPIFFE settings under the `spiffe`
value:

```yaml
spiffe:
  workloadAPI:
    endpoint: "unix:///spiffe-workload-api/agent.sock"
    csiDriver: "csi.spiffe.io"
  brokerAPI:
    endpoint: "dns:///spiffe-broker.example.com:443"
    ids:
      - "spiffe://example.org/broker/"
```

The chart configures the `SPIFFE_ENDPOINT_SOCKET` environment variable, the
`--spiffe-broker-endpoint` and `--spiffe-broker-id` flags, and, when
`csiDriver` is set, the SPIFFE CSI volume and mount.

The `csiDriver` field is optional and opt-in. Omit it when the Workload API
socket is mounted into the operator pod by other means.

To require SPIFFE mTLS from Ingress or Gateway proxies that access the Web UI,
set the Helm chart value `web.authorizedClientSPIFFEIDs` to the list of
authorized client IDs. This renders the repeatable
`--authorized-web-ui-client-spiffe-id` flag. Setting at least one client ID
makes the web server accept only clients presenting an authorized SPIFFE
identity.

### OLM

For OLM installations, set the equivalent environment variables in the
`Subscription`:

```yaml
spec:
  config:
    env:
      - name: SPIFFE_ENDPOINT_SOCKET
        value: "unix:///spiffe-workload-api/agent.sock"
      - name: SPIFFE_BROKER_ENDPOINT
        value: "dns:///spiffe-broker.example.com:443"
      - name: SPIFFE_BROKER_IDS
        value: "spiffe://example.org/broker/,spiffe://example.org/backup-broker/"
      - name: AUTHORIZED_WEB_UI_CLIENT_SPIFFE_IDS
        value: "spiffe://example.org/my-gateway/"
    volumes:
      - name: spiffe-workload-api
        csi:
          driver: csi.spiffe.io
          readOnly: true
    volumeMounts:
      - name: spiffe-workload-api
        mountPath: /spiffe-workload-api
        readOnly: true
```

`SPIFFE_BROKER_IDS` is a comma-separated list of SPIFFE IDs or prefixes.
The `--spiffe-broker-id` flag is repeatable, while the OLM environment
variable accepts the same values in a single comma-separated string.

The `SPIFFE_ENDPOINT_SOCKET` environment variable is the standard go-spiffe
variable read by the operator.

## Upgrade

The recommended way to upgrade the Flux Operator is using GitOps, by managing the operator's
installation with a Flux `HelmRelease`. 

For a complete example, see the [Automating Flux Operator upgrades](operator-migration.md#automating-flux-operator-upgrades)
section in the migration guide.

## Uninstall

The recommended way to uninstall the Flux Operator and Flux instance is using the
[flux-operator CLI](https://fluxoperator.dev/docs/guides/cli/):

```shell
flux-operator -n flux-system uninstall --keep-namespace
```

The `uninstall` command safely removes the Flux Operator and Flux controllers
without affecting the Kubernetes objects or Helm releases reconciled by Flux.
It is safe to re-install the Flux Operator later to resume managing the existing resources.

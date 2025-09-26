// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"strings"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	defaultServiceAccount = "default"
)

// GetProfileClusterSize returns a patch that configures the
// Flux controllers for a specific cluster size.
func GetProfileClusterSize(size string) string {
	switch size {
	case "small":
		return profileClusterSizeSmall
	case "medium":
		return profileClusterSizeMedium
	case "large":
		return profileClusterSizeLarge
	default:
		return profileClusterSizeDefault
	}
}

// profileClusterSizeSmall sets concurrency to 5 and limits to 1CPU/512Mi.
const profileClusterSizeSmall = `
- target:
    kind: Deployment
  patch: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: all
      annotations:
        fluxcd.controlplane.io/profile: "small"
    spec:
      template:
        metadata:
          labels:
            app.kubernetes.io/part-of: flux
          annotations:
            cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
- target:
    kind: Deployment
  patch: |
    - op: replace
      path: /spec/template/spec/containers/0/resources
      value:
        requests:
          cpu: 100m
          memory: 64Mi
        limits:
          cpu: 1000m
          memory: 512Mi
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --concurrent=5
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --requeue-dependency=10s
- target:
    kind: Deployment
    name: "(source-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-max-size=10
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-ttl=720m
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-purge-interval=60m
`

// profileClusterSizeMedium sets concurrency to 10 and limits to 2CPU/1Gi.
const profileClusterSizeMedium = `
- target:
    kind: Deployment
  patch: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: all
      annotations:
        fluxcd.controlplane.io/profile: "medium"
    spec:
      template:
        metadata:
          labels:
            app.kubernetes.io/part-of: flux
          annotations:
            cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller|source-controller)"
  patch: |
    - op: replace
      path: /spec/template/spec/containers/0/resources
      value:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 2000m
          memory: 1Gi
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --concurrent=10
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --requeue-dependency=5s
- target:
    kind: Deployment
    name: "(source-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-max-size=50
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-ttl=720m
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-purge-interval=60m
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --concurrent=5
`

// profileClusterSizeLarge is a Flux performance profile for ~3000 apps.
const profileClusterSizeLarge = `
- target:
    kind: Deployment
  patch: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: all
      annotations:
        fluxcd.controlplane.io/profile: "large"
    spec:
      template:
        metadata:
          labels:
            app.kubernetes.io/part-of: flux
          annotations:
            cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --concurrent=20
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --requeue-dependency=5s
    - op: replace
      path: /spec/template/spec/containers/0/resources
      value:
        requests:
          cpu: 100m
          memory: 256Mi
        limits:
          cpu: 3000m
          memory: 3Gi
- target:
    kind: Deployment
    name: "(source-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-max-size=100
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-ttl=720m
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --helm-cache-purge-interval=60m
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --concurrent=10
    - op: replace
      path: /spec/template/spec/containers/0/resources
      value:
        requests:
          cpu: 100m
          memory: 256Mi
        limits:
          cpu: 2000m
          memory: 2Gi
`

const profileClusterSizeDefault = `
- target:
    kind: Deployment
  patch: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: all
    spec:
      template:
        metadata:
          labels:
            app.kubernetes.io/part-of: flux
          annotations:
            cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
`

// GetProfileClusterType returns a patch that configures the
// Flux controllers for a specific Kubernetes distribution.
func GetProfileClusterType(clusterType string) string {
	switch clusterType {
	case "openshift":
		return profileClusterTypeOpenShift
	default:
		return ""
	}
}

const profileClusterTypeOpenShift = `
- target:
    kind: Deployment
  patch: |-
    - op: remove
      path: /spec/template/spec/securityContext
    - op: remove
      path: /spec/template/spec/containers/0/securityContext/seccompProfile
    - op: remove
      path: /spec/template/spec/containers/0/securityContext/runAsNonRoot
- target:
    kind: Namespace
  patch: |-
    - op: remove
      path: /metadata/labels/pod-security.kubernetes.io~1warn
    - op: remove
      path: /metadata/labels/pod-security.kubernetes.io~1warn-version
`

// GetProfileMultitenant returns a patch to enable multitenancy in the Flux controllers.
func GetProfileMultitenant(defaultSA string) string {
	if defaultSA == "" {
		defaultSA = defaultServiceAccount
	}

	return fmt.Sprintf(profileClusterMultitenant, defaultSA)
}

const profileClusterMultitenant = `
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller|notification-controller|image-reflector-controller|image-automation-controller|source-watcher)"
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --no-cross-namespace-refs=true
- target:
    kind: Deployment
    name: "(kustomize-controller)"
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --no-remote-bases=true
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller)"
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --default-service-account=%s
- target:
    kind: Kustomization
  patch: |-
    - op: add
      path: /spec/serviceAccountName
      value: kustomize-controller
`

// GetProfileMultitenantWorkloadIdentity returns a patch to enable
// multitenant workload identity in the Flux controllers through
// the feature gate.
func GetProfileMultitenantWorkloadIdentity(cluster fluxcdv1.Cluster) string {
	defaultSA := cluster.TenantDefaultServiceAccount
	if defaultSA == "" {
		defaultSA = defaultServiceAccount
	}

	defaultDecryptionSA := cluster.TenantDefaultDecryptionServiceAccount
	if defaultDecryptionSA == "" {
		defaultDecryptionSA = defaultServiceAccount
	}

	defaultKubeConfigSA := cluster.TenantDefaultKubeConfigServiceAccount
	if defaultKubeConfigSA == "" {
		defaultKubeConfigSA = defaultServiceAccount
	}

	return fmt.Sprintf(profileClusterMultitenantWorkloadIdentity,
		defaultSA, defaultDecryptionSA, defaultKubeConfigSA)
}

const profileClusterMultitenantWorkloadIdentity = `
- target:
    kind: Deployment
    name: "(source-controller|notification-controller|image-reflector-controller|image-automation-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --default-service-account=%[1]s
- target:
    kind: Deployment
    name: "(kustomize-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --default-decryption-service-account=%[2]s
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --default-kubeconfig-service-account=%[3]s
`

// GetProfileObjectLevelWorkloadIdentity returns a patch to enable
// object level workload identity in the Flux controllers through
// the feature gate.
func GetProfileObjectLevelWorkloadIdentity(controllers []string, enabled bool) string {
	c := strings.Join(controllers, "|")

	return fmt.Sprintf(profileClusterObjectLevelWorkloadIdentity, c, enabled)
}

const profileClusterObjectLevelWorkloadIdentity = `
- target:
    kind: Deployment
    name: "(%[1]s)"
  patch: |
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --feature-gates=ObjectLevelWorkloadIdentity=%[2]v
`

const profileExternalArtifactFeatureGate = `
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller)"
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: --feature-gates=ExternalArtifact=true
`

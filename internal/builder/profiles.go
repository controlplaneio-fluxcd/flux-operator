// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import "fmt"

const ProfileOpenShift = `
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

const profileMultitenant = `
- target:
    kind: Deployment
    name: "(kustomize-controller|helm-controller|notification-controller|image-reflector-controller|image-automation-controller)"
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

func GetMultitenantProfile(defaultSA string) string {
	if defaultSA == "" {
		defaultSA = "default"
	}

	return fmt.Sprintf(profileMultitenant, defaultSA)
}

const tmpNotificationPatch = `
- target:
    kind: CustomResourceDefinition
    name: alerts.notification.toolkit.fluxcd.io
  patch: |-
    - op: add
      path: /spec/versions/0/schema/openAPIV3Schema/properties/spec/properties/eventSources/items/properties/kind/enum/-
      value: FluxInstance
    - op: add
      path: /spec/versions/1/schema/openAPIV3Schema/properties/spec/properties/eventSources/items/properties/kind/enum/-
      value: FluxInstance
    - op: add
      path: /spec/versions/2/schema/openAPIV3Schema/properties/spec/properties/eventSources/items/properties/kind/enum/-
      value: FluxInstance
- target:
    kind: CustomResourceDefinition
    name: receivers.notification.toolkit.fluxcd.io
  patch: |-
    - op: add
      path: /spec/versions/0/schema/openAPIV3Schema/properties/spec/properties/resources/items/properties/kind/enum/-
      value: FluxInstance
    - op: add
      path: /spec/versions/1/schema/openAPIV3Schema/properties/spec/properties/resources/items/properties/kind/enum/-
      value: FluxInstance
    - op: add
      path: /spec/versions/2/schema/openAPIV3Schema/properties/spec/properties/resources/items/properties/kind/enum/-
      value: FluxInstance
- target:
    kind: ClusterRole
    name: crd-controller-%s
  patch: |-
    - op: add
      path: /rules/-
      value:
       apiGroups: [ 'fluxcd.controlplane.io' ]
       resources: [ '*' ]
       verbs: [ '*' ]
`

func GetNotificationPatch(namespace string) string {
	if namespace == "" {
		namespace = "flux-system"
	}

	return fmt.Sprintf(tmpNotificationPatch, namespace)
}

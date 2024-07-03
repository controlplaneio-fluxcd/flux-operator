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

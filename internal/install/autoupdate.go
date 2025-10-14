// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
)

// ApplyAutoUpdate configures automatic updates of the Flux Operator from the distribution artifact.
func (in *Installer) ApplyAutoUpdate(ctx context.Context, multitenant bool) (*ssa.ChangeSet, error) {
	// Strip tag from artifact URL (e.g., "oci://registry/image:tag" -> "oci://registry/image")
	artifactURL := in.options.artifactURL
	if idx := strings.LastIndex(artifactURL, ":"); idx > 6 {
		artifactURL = artifactURL[:idx]
	}

	// Build template data
	data := struct {
		Namespace   string
		ArtifactURL string
		Multitenant bool
	}{
		Namespace:   in.options.namespace,
		ArtifactURL: artifactURL,
		Multitenant: multitenant,
	}

	// Execute template
	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	if err != nil {
		return nil, fmt.Errorf("unable to parse auto-update template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("unable to execute auto-update template: %w", err)
	}
	autoUpdateYAML := buf.String()

	autoUpdateObjects, err := ssautil.ReadObjects(bytes.NewReader([]byte(autoUpdateYAML)))
	if err != nil {
		return nil, fmt.Errorf("unable to parse auto-update manifest: %w", err)
	}

	return in.kubeClient.Manager.ApplyAllStaged(ctx, autoUpdateObjects, ssa.DefaultApplyOptions())
}

const autoUpdateTmpl = `
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-operator
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/name: flux-operator
    app.kubernetes.io/instance: flux-operator
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: "5m"
spec:
  wait: true
  inputs:
    - url: {{.ArtifactURL}}
      interval: "1h"
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: << inputs.provider.name >>
        namespace: << inputs.provider.namespace >>
      spec:
        interval: << inputs.interval | quote >>
        url: << inputs.url | quote >>
        ref:
          tag: latest
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: << inputs.provider.name >>
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 24h
        retryInterval: 5m
        timeout: 5m
        wait: true
        prune: true
        force: true
        deletionPolicy: Orphan
        serviceAccountName: << inputs.provider.name >>
        sourceRef:
          kind: OCIRepository
          name: << inputs.provider.name >>
        path: ./flux-operator
        commonMetadata:
          labels:
            app.kubernetes.io/name: flux-operator
            app.kubernetes.io/instance: flux-operator
        patches:
          - patch: |-
              - op: replace
                path: "/spec/selector/matchLabels"
                value:
                  app.kubernetes.io/name: flux-operator
                  app.kubernetes.io/instance: flux-operator
              - op: replace
                path: "/spec/template/metadata/labels"
                value:
                  app.kubernetes.io/name: flux-operator
                  app.kubernetes.io/instance: flux-operator
              - op: add
                path: "/spec/template/spec/containers/0/env/-"
                value:
                  name: REPORTING_INTERVAL
                  value: "30s"
{{- if .Multitenant }}
              - op: add
                path: "/spec/template/spec/containers/0/env/-"
                value:
                  name: DEFAULT_SERVICE_ACCOUNT
                  value: "flux-operator"
{{- end }}
            target:
              kind: Deployment
`

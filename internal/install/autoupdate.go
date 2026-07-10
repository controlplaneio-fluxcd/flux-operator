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

// ApplyAutoUpdate configures automatic updates of the Flux Operator from the configured OCIRepository.
func (in *Installer) ApplyAutoUpdate(ctx context.Context, multitenant bool) (*ssa.ChangeSet, error) {
	artifactURL := artifactRepositoryURL(in.options.artifactURL)

	ociRepository, err := renderAutoUpdateOCIRepository(artifactURL, in.options.autoUpdateOCIRepository)
	if err != nil {
		return nil, err
	}

	data := struct {
		Namespace     string
		ArtifactURL   string
		OCIRepository string
		Multitenant   bool
	}{
		Namespace:     in.options.namespace,
		ArtifactURL:   artifactURL,
		OCIRepository: yamlListItem(ociRepository),
		Multitenant:   multitenant,
	}

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

func renderAutoUpdateOCIRepository(artifactURL, customOCIRepository string) (string, error) {
	if strings.TrimSpace(customOCIRepository) != "" {
		return strings.TrimSpace(customOCIRepository), nil
	}

	data := struct {
		ArtifactURL string
	}{
		ArtifactURL: artifactURL,
	}

	tmpl, err := template.New("autoUpdateOCIRepository").Parse(autoUpdateOCIRepositoryTmpl)
	if err != nil {
		return "", fmt.Errorf("unable to parse auto-update OCIRepository template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("unable to execute auto-update OCIRepository template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func artifactRepositoryURL(artifactURL string) string {
	repositoryURL, _, _ := strings.Cut(artifactURL, "@")
	if idx := strings.LastIndex(repositoryURL, ":"); idx > strings.LastIndex(repositoryURL, "/") {
		return repositoryURL[:idx]
	}
	return repositoryURL
}

func yamlListItem(manifest string) string {
	const indent = 4

	lines := strings.Split(strings.TrimSpace(manifest), "\n")
	if len(lines) == 0 {
		return ""
	}

	prefix := strings.Repeat(" ", indent)
	childPrefix := strings.Repeat(" ", indent+2)
	lines[0] = prefix + "- " + strings.TrimLeft(lines[0], " ")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = childPrefix + strings.TrimRight(lines[i], " ")
	}

	return strings.Join(lines, "\n")
}

const autoUpdateOCIRepositoryTmpl = `apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: << inputs.provider.name >>
  namespace: << inputs.provider.namespace >>
spec:
  interval: << inputs.interval | quote >>
  url: << inputs.url | quote >>
  ref:
    tag: latest
`

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
{{.OCIRepository}}
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

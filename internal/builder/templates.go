// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"text/template"
)

var kustomizationTmpl = `---
{{- $eventsAddr := .EventsAddr }}
{{- $watchAllNamespaces := .WatchAllNamespaces }}
{{- $registry := .Registry }}
{{- $logLevel := .LogLevel }}
{{- $clusterDomain := "" }}
{{- if .ClusterDomain }}
{{- $clusterDomain = .ClusterDomain }}
{{- end }}
{{- $artifactStorage := .ArtifactStorage }}
{{- $sync := .Sync }}
{{- $namespace := .Namespace }}
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{$namespace}}
transformers:
  - annotations.yaml
  - labels.yaml
resources:
  - namespace.yaml
{{- if .NetworkPolicy }}
  - policies.yaml
{{- end }}
  - roles
{{- range .Components }}
  - {{.}}.yaml
{{- end }}
{{- if $artifactStorage }}
  - pvc.yaml
{{- end }}
{{- if $sync }}
  - sync.yaml
{{- end }}
{{- if $registry }}
images:
{{- range .ComponentImages }}
  - name: fluxcd/{{.Name}}
    newName: {{.Repository}}
    newTag: {{.Tag}}
{{- if .Digest }}
    digest: {{.Digest}}
{{- end }}
{{- end }}
{{- end }}
patches:
- path: node-selector.yaml
  target:
    kind: Deployment
{{- range $i, $component := .Components }}
{{- if eq $component "notification-controller" }}
- target:
    group: apps
    version: v1
    kind: Deployment
    name: {{$component}}
  patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/args/0
      value: --watch-all-namespaces={{$watchAllNamespaces}}
    - op: replace
      path: /spec/template/spec/containers/0/args/1
      value: --log-level={{$logLevel}}
{{- else if eq $component "source-controller" }}
- target:
    group: apps
    version: v1
    kind: Deployment
    name: {{$component}}
  patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/args/0
      value: --events-addr={{$eventsAddr}}
    - op: replace
      path: /spec/template/spec/containers/0/args/1
      value: --watch-all-namespaces={{$watchAllNamespaces}}
    - op: replace
      path: /spec/template/spec/containers/0/args/2
      value: --log-level={{$logLevel}}
    - op: replace
      path: /spec/template/spec/containers/0/args/6
      {{- if $clusterDomain }}
      value: --storage-adv-addr=source-controller.$(RUNTIME_NAMESPACE).svc.{{$clusterDomain}}.
      {{- else }}
      value: --storage-adv-addr=source-controller.$(RUNTIME_NAMESPACE).svc
      {{- end }}
{{- if $artifactStorage }}
    - op: add
      path: '/spec/template/spec/volumes/-'
      value:
        name: persistent-data
        persistentVolumeClaim:
          claimName: source-controller
    - op: replace
      path: '/spec/template/spec/containers/0/volumeMounts/0'
      value:
        name: persistent-data
        mountPath: /data
{{- end }}
{{- else }}
- target:
    group: apps
    version: v1
    kind: Deployment
    name: {{$component}}
  patch: |-
    - op: replace
      path: /spec/template/spec/containers/0/args/0
      value: --events-addr={{$eventsAddr}}
    - op: replace
      path: /spec/template/spec/containers/0/args/1
      value: --watch-all-namespaces={{$watchAllNamespaces}}
    - op: replace
      path: /spec/template/spec/containers/0/args/2
      value: --log-level={{$logLevel}}
{{- end }}
{{- end }}
{{ .Patches }}
`

var kustomizationRolesTmpl = `---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{.Namespace}}
resources:
  - rbac.yaml
nameSuffix: -{{.Namespace}}
`

var nodeSelectorTmpl = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: all
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/os: linux
{{- if .ImagePullSecret }}
      imagePullSecrets:
       - name: {{.ImagePullSecret}}
{{- end }}
{{ if gt (len .TolerationKeys) 0 }}
      tolerations:
{{- range $i, $key := .TolerationKeys }}
       - key: "{{$key}}"
         operator: "Exists"
{{- end }}
{{- end }}
`

var labelsTmpl = `---
apiVersion: builtin
kind: LabelTransformer
metadata:
  name: labels
labels:
  app.kubernetes.io/managed-by: flux-operator
  app.kubernetes.io/instance: {{.Namespace}}
  app.kubernetes.io/version: "{{.Version}}"
  app.kubernetes.io/part-of: flux
fieldSpecs:
  - path: metadata/labels
    create: true
`

var annotationsTmpl = `---
apiVersion: builtin
kind: AnnotationsTransformer
metadata:
  name: annotations
annotations:
  kustomize.toolkit.fluxcd.io/ssa: Ignore
fieldSpecs:
  - path: metadata/annotations
    create: true
`

var namespaceTmpl = `---
apiVersion: v1
kind: Namespace
metadata:
  name: {{.Namespace}}
  labels:
    pod-security.kubernetes.io/warn: restricted
    pod-security.kubernetes.io/warn-version: latest
  annotations:
    fluxcd.controlplane.io/prune: disabled
`

var pvcTmpl = `---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: source-controller
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: {{.ArtifactStorage.Class}}
  resources:
    requests:
      storage: {{.ArtifactStorage.Size}}
`

var syncTmpl = `---
{{- $sync := .Sync }}
{{- $namespace := .Namespace }}
{{- if eq $sync.Kind "GitRepository" }}
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
{{- else if eq $sync.Kind "OCIRepository" }}
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
{{- else if eq $sync.Kind "Bucket" }}
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: Bucket
{{- end }}
metadata:
  name: {{$namespace}}
  namespace: {{$namespace}}
spec:
  interval: {{$sync.Interval}}
{{- if eq $sync.Kind "GitRepository" }}
  ref:
    name: {{$sync.Ref}}
{{- else if eq $sync.Kind "OCIRepository" }}
  ref:
    tag: {{$sync.Ref}}
{{- else if eq $sync.Kind "Bucket" }}
  bucketName: {{$sync.Ref}}
{{- end }}
{{- if $sync.PullSecret }}
  secretRef:
    name: {{$sync.PullSecret}}
{{- end }}
  url: {{$sync.URL}}
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{$namespace}}
  namespace: {{$namespace}}
spec:
  interval: 10m0s
  path: {{$sync.Path}}
  prune: true
  sourceRef:
    kind: {{$sync.Kind}}
    name: {{$namespace}}
`

func execTemplate(obj interface{}, tmpl, filename string) (err error) {
	t, err := template.New("tmpl").Parse(tmpl)
	if err != nil {
		return err
	}

	var data bytes.Buffer
	writer := bufio.NewWriter(&data)
	if err := t.Execute(writer, obj); err != nil {
		return err
	}

	if err := writer.Flush(); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err = file.Close()
	}(file)

	_, err = io.WriteString(file, data.String())
	if err != nil {
		return err
	}

	return file.Sync()
}

func containsItemString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

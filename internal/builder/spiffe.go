// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/semver/v3"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	internalspiffe "github.com/controlplaneio-fluxcd/flux-operator/internal/spiffe"
)

const (
	spiffeVolumeName      = "spiffe-workload-api"
	spiffeVolumeMountPath = "/spiffe-workload-api"
)

// spiffePatchData is the input for the SPIFFE kustomize patches template.
type spiffePatchData struct {
	WorkloadAPI       spiffeWorkloadAPIData
	DeploymentPatches []spiffeDeploymentPatchData
}

type spiffeWorkloadAPIData struct {
	Components string
	Endpoint   string
	CSIDriver  string
}

type spiffeDeploymentPatchData struct {
	Name string
	Args []string
}

const spiffeTmpl = `
{{- if .WorkloadAPI.Components }}
- target:
    group: apps
    version: v1
    kind: Deployment
    name: "{{ .WorkloadAPI.Components }}"
  patch: |-
    - op: add
      path: /spec/template/spec/containers/0/env/-
      value:
        name: SPIFFE_ENDPOINT_SOCKET
        value: {{ .WorkloadAPI.Endpoint }}
{{- if .WorkloadAPI.CSIDriver }}
    - op: add
      path: /spec/template/spec/volumes/-
      value:
        name: spiffe-workload-api
        csi:
          driver: {{ .WorkloadAPI.CSIDriver }}
          readOnly: true
    - op: add
      path: /spec/template/spec/containers/0/volumeMounts/-
      value:
        name: spiffe-workload-api
        mountPath: /spiffe-workload-api
        readOnly: true
{{- end }}
{{- end }}
{{- range .DeploymentPatches }}
- target:
    group: apps
    version: v1
    kind: Deployment
    name: "{{ .Name }}"
  patch: |-
{{- range .Args }}
    - op: add
      path: /spec/template/spec/containers/0/args/-
      value: {{ . }}
{{- end }}
{{- end }}
`

// ValidateAndApplySPIFFEConfig validates and applies the SPIFFE configuration
// based on the Flux version in the Options struct.
func (o *Options) ValidateAndApplySPIFFEConfig(instance *fluxcdv1.FluxInstance) error {
	spiffe := instance.Spec.SPIFFE
	if spiffe == nil {
		return nil
	}

	version, err := semver.NewVersion(o.Version)
	if err != nil {
		return fmt.Errorf("failed to parse Flux version '%s': %w", o.Version, err)
	}

	supported, err := checkVersionAgainstConstraint(version, ">= 2.10.0")
	if err != nil {
		return err
	}
	if !supported {
		return fmt.Errorf(".spec.spiffe requires Flux >= 2.10.0, please remove .spec.spiffe before using an older Flux version")
	}

	data := spiffePatchData{
		WorkloadAPI: spiffeWorkloadAPIData{
			Components: strings.Join(o.Components, "|"),
			Endpoint:   spiffe.WorkloadAPI.Endpoint,
			CSIDriver:  spiffe.WorkloadAPI.CSIDriver,
		},
	}
	if spiffe.BrokerAPI != nil {
		data.DeploymentPatches = append(data.DeploymentPatches, spiffeBrokerAPIData(o.Components, *spiffe.BrokerAPI)...)
	}
	if spiffe.MTLS != nil && spiffe.MTLS.Enabled {
		data.DeploymentPatches = append(data.DeploymentPatches, spiffeMTLSData(o.Components, *spiffe.MTLS, o.OperatorSPIFFEID)...)
	}

	// Skip patches without args, as kustomize rejects an empty patch.
	data.DeploymentPatches = slices.DeleteFunc(data.DeploymentPatches, func(p spiffeDeploymentPatchData) bool {
		return len(p.Args) == 0
	})

	t, err := template.New("spiffe").Parse(spiffeTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse SPIFFE template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render SPIFFE patches: %w", err)
	}
	o.Patches += buf.String()
	return nil
}

// spiffeBrokerAPIData returns the patches that configure the SPIFFE Broker API
// flags on the controllers that support object identity.
func spiffeBrokerAPIData(components []string, api fluxcdv1.BrokerAPI) []spiffeDeploymentPatchData {
	var patches []spiffeDeploymentPatchData
	for _, component := range components {
		if component == fluxcdv1.FluxSourceWatcher {
			continue
		}
		args := make([]string, 0, 1+len(api.IDs))
		args = append(args, fmt.Sprintf("--spiffe-broker-endpoint=%s", api.Endpoint))
		for _, id := range api.IDs {
			args = append(args, fmt.Sprintf("--spiffe-broker-id=%s", id))
		}
		patches = append(patches, spiffeDeploymentPatchData{
			Name: component,
			Args: args,
		})
	}
	return patches
}

// spiffeMTLSData returns the patches for the inter-controller SPIFFE mTLS
// configuration. The operator is authorized with operatorID, or with the
// template-derived flux-operator ID when operatorID is empty.
func spiffeMTLSData(components []string, mtls fluxcdv1.MTLS, operatorID string) []spiffeDeploymentPatchData {
	var patches []spiffeDeploymentPatchData
	componentIDTemplate := mtls.ComponentIDTemplate
	if operatorID == "" {
		operatorID = internalspiffe.ComponentID(componentIDTemplate, fluxcdv1.FluxOperator)
	}
	has := func(component string) bool {
		return containsComponent(components, component)
	}
	ids := func(names ...string) []string {
		return internalspiffe.ComponentIDs(componentIDTemplate, names...)
	}

	if has(fluxcdv1.FluxSourceController) {
		clients := ids(installed(components, fluxcdv1.FluxKustomizeController, fluxcdv1.FluxHelmController, fluxcdv1.FluxSourceWatcher)...)
		clients = append(clients, mtls.ExtraArtifactClients...)
		clients = append(clients, operatorID)
		args := spiffeFlagArgs("--authorized-artifact-client-spiffe-id", clients)
		if has(fluxcdv1.FluxNotificationController) {
			args = append(args, spiffeFlagArgs("--authorized-event-server-spiffe-id", ids(fluxcdv1.FluxNotificationController))...)
		}
		patches = append(patches, spiffeDeploymentPatchData{
			Name: fluxcdv1.FluxSourceController,
			Args: args,
		})
	}

	// Source-watcher must authorize itself as both an artifact client and an
	// artifact server. ArtifactGenerator sources can be ExternalArtifacts,
	// including ones produced by source-watcher itself, and artifact fetches
	// use the same authorized artifact server list.
	if has(fluxcdv1.FluxSourceWatcher) {
		clients := ids(installed(components, fluxcdv1.FluxKustomizeController, fluxcdv1.FluxHelmController, fluxcdv1.FluxSourceWatcher)...)
		clients = append(clients, mtls.ExtraArtifactClients...)
		clients = append(clients, operatorID)
		args := spiffeFlagArgs("--authorized-artifact-client-spiffe-id", clients)

		servers := internalspiffe.ArtifactServers(componentIDTemplate, components, mtls.ExtraArtifactServers)
		args = append(args, spiffeFlagArgs("--authorized-artifact-server-spiffe-id", servers)...)
		if has(fluxcdv1.FluxNotificationController) {
			args = append(args, spiffeFlagArgs("--authorized-event-server-spiffe-id", ids(fluxcdv1.FluxNotificationController))...)
		}
		patches = append(patches, spiffeDeploymentPatchData{
			Name: fluxcdv1.FluxSourceWatcher,
			Args: args,
		})
	}

	for _, component := range []string{fluxcdv1.FluxKustomizeController, fluxcdv1.FluxHelmController} {
		if !has(component) {
			continue
		}
		servers := internalspiffe.ArtifactServers(componentIDTemplate, components, mtls.ExtraArtifactServers)
		args := spiffeFlagArgs("--authorized-artifact-server-spiffe-id", servers)
		if has(fluxcdv1.FluxNotificationController) {
			args = append(args, spiffeFlagArgs("--authorized-event-server-spiffe-id", ids(fluxcdv1.FluxNotificationController))...)
		}
		patches = append(patches, spiffeDeploymentPatchData{
			Name: component,
			Args: args,
		})
	}

	if has(fluxcdv1.FluxNotificationController) {
		notifiers := ids(components...)
		notifiers = append(notifiers, operatorID)
		notifiers = append(notifiers, mtls.ExtraNotifiers...)
		args := spiffeFlagArgs("--authorized-notifier-spiffe-id", notifiers)
		args = append(args, spiffeFlagArgs("--authorized-receiver-spiffe-id", mtls.Receivers)...)
		patches = append(patches, spiffeDeploymentPatchData{
			Name: fluxcdv1.FluxNotificationController,
			Args: args,
		})
	}

	for _, component := range []string{fluxcdv1.FluxImageReflectorController, fluxcdv1.FluxImageAutomationController} {
		if !has(component) {
			continue
		}
		if has(fluxcdv1.FluxNotificationController) {
			patches = append(patches, spiffeDeploymentPatchData{
				Name: component,
				Args: spiffeFlagArgs("--authorized-event-server-spiffe-id", ids(fluxcdv1.FluxNotificationController)),
			})
		}
	}

	return patches
}

// spiffeFlagArgs returns the repeatable flag arguments with duplicate values
// removed.
func spiffeFlagArgs(flag string, values []string) []string {
	seen := make(map[string]struct{}, len(values))
	args := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		args = append(args, fmt.Sprintf("%s=%s", flag, value))
	}
	return args
}

func containsComponent(components []string, component string) bool {
	for _, c := range components {
		if c == component {
			return true
		}
	}
	return false
}

// installed returns the subset of candidates that are installed.
func installed(components []string, candidates ...string) []string {
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if containsComponent(components, candidate) {
			result = append(result, candidate)
		}
	}
	return result
}

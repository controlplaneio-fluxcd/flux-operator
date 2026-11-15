// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package spiffe provides helpers for deriving the SPIFFE configuration of
// the Flux controllers from a FluxInstance. The mTLS helpers read the
// applied settings from the FluxInstance status.
package spiffe

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	fluxspiffe "github.com/fluxcd/pkg/spiffe"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const componentPlaceholder = "{component}"

// ErrNoFluxInstance is returned when no FluxInstance is found.
var ErrNoFluxInstance = errors.New("no FluxInstances found")

// GetFluxInstance returns the given FluxInstance or fetches the single
// FluxInstance managed by the operator when instance is nil.
func GetFluxInstance(ctx context.Context, c client.Client,
	instance *fluxcdv1.FluxInstance) (*fluxcdv1.FluxInstance, error) {
	if instance != nil {
		return instance, nil
	}
	if c == nil {
		return nil, errors.New("client is required to fetch the FluxInstance")
	}

	var list fluxcdv1.FluxInstanceList
	if err := c.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("failed to list FluxInstances: %w", err)
	}
	switch len(list.Items) {
	case 0:
		return nil, ErrNoFluxInstance
	case 1:
		return &list.Items[0], nil
	default:
		return nil, errors.New("multiple FluxInstances found, only one is supported")
	}
}

// AppliedMTLSEnabled reports whether .status.spiffe.mtls has mTLS enabled.
func AppliedMTLSEnabled(instance *fluxcdv1.FluxInstance) bool {
	return instance != nil &&
		instance.Status.SPIFFE != nil &&
		instance.Status.SPIFFE.MTLS != nil &&
		instance.Status.SPIFFE.MTLS.Enabled
}

// AppliedComponentID returns the SPIFFE ID of the component from the applied
// ID template, or false when mTLS is not applied.
func AppliedComponentID(instance *fluxcdv1.FluxInstance, component string) (string, bool) {
	if !AppliedMTLSEnabled(instance) {
		return "", false
	}
	template := instance.Status.SPIFFE.MTLS.ComponentIDTemplate
	if template == "" {
		return "", false
	}
	return ComponentID(template, component), true
}

// ComponentID returns the SPIFFE ID of the component from the ID template.
func ComponentID(template, component string) string {
	return strings.ReplaceAll(template, componentPlaceholder, component)
}

// ComponentIDs returns the SPIFFE IDs of the components from the ID template.
func ComponentIDs(template string, components ...string) []string {
	ids := make([]string, 0, len(components))
	for _, component := range components {
		ids = append(ids, ComponentID(template, component))
	}
	return ids
}

// ArtifactServers returns the SPIFFE IDs authorized as artifact servers:
// the installed source-controller and source-watcher, then the extra servers.
func ArtifactServers(template string, components, extras []string) []string {
	var ids []string
	if template != "" {
		for _, component := range []string{fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher} {
			if slices.Contains(components, component) {
				ids = append(ids, ComponentID(template, component))
			}
		}
	}
	return append(ids, extras...)
}

// AppliedComponents returns the component names in .status.components.
func AppliedComponents(instance *fluxcdv1.FluxInstance) []string {
	components := make([]string, 0, len(instance.Status.Components))
	for _, c := range instance.Status.Components {
		components = append(components, c.Name)
	}
	return components
}

// ArtifactServerIDs returns the SPIFFE IDs of the applied artifact servers.
func ArtifactServerIDs(instance *fluxcdv1.FluxInstance) []string {
	if !AppliedMTLSEnabled(instance) {
		return nil
	}
	mtls := instance.Status.SPIFFE.MTLS
	return ArtifactServers(mtls.ComponentIDTemplate, AppliedComponents(instance), mtls.ExtraArtifactServers)
}

// NotificationServerID returns the SPIFFE ID of the applied
// notification-controller.
func NotificationServerID(instance *fluxcdv1.FluxInstance) (string, bool) {
	if !slices.Contains(AppliedComponents(instance), fluxcdv1.FluxNotificationController) {
		return "", false
	}
	return AppliedComponentID(instance, fluxcdv1.FluxNotificationController)
}

// IsSelfTrustDomainPeer reports whether the peer is a spiffe://self form,
// which refers to the local trust domain.
func IsSelfTrustDomainPeer(peer string) bool {
	return peer == "spiffe://self" || strings.HasPrefix(peer, "spiffe://self/")
}

// ArtifactServerPeers returns the artifact server SPIFFE IDs when mTLS is
// applied, or nil when it isn't or no FluxInstance exists.
// The FluxInstance is fetched when instance is nil.
func ArtifactServerPeers(ctx context.Context, c client.Client, source fluxspiffe.TLSSource,
	instance *fluxcdv1.FluxInstance) ([]string, error) {
	instance, err := GetFluxInstance(ctx, c, instance)
	if errors.Is(err, ErrNoFluxInstance) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !AppliedMTLSEnabled(instance) {
		return nil, nil
	}
	if source == nil {
		return nil, errors.New("SPIFFE mTLS is enabled but the SPIFFE Workload API source is not configured")
	}

	peers := ArtifactServerIDs(instance)
	if len(peers) == 0 {
		return nil, errors.New("SPIFFE mTLS is enabled but no artifact server SPIFFE IDs were found")
	}
	return peers, nil
}

// EventServerPeer returns the notification-controller SPIFFE ID when mTLS
// is applied, or an empty string when it isn't.
func EventServerPeer(source fluxspiffe.TLSSource, instance *fluxcdv1.FluxInstance) (string, error) {
	if !AppliedMTLSEnabled(instance) {
		return "", nil
	}
	if source == nil {
		return "", errors.New("SPIFFE mTLS is enabled but the SPIFFE Workload API source is not configured")
	}

	id, ok := NotificationServerID(instance)
	if !ok {
		return "", errors.New("SPIFFE mTLS is enabled but notification-controller is not installed")
	}
	return id, nil
}

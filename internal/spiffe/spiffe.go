// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package spiffe provides helpers for deriving the SPIFFE configuration of
// the Flux controllers from a FluxInstance.
package spiffe

import (
	"context"
	"crypto/tls"
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

// MTLSEnabled reports whether SPIFFE mTLS is enabled for the given
// FluxInstance.
func MTLSEnabled(instance *fluxcdv1.FluxInstance) bool {
	return instance != nil &&
		instance.Spec.SPIFFE != nil &&
		instance.Spec.SPIFFE.MTLS != nil &&
		instance.Spec.SPIFFE.MTLS.Enabled
}

// ComponentID returns the SPIFFE ID of the given Flux component.
func ComponentID(instance *fluxcdv1.FluxInstance, component string) (string, bool) {
	if !MTLSEnabled(instance) {
		return "", false
	}
	template := instance.Spec.SPIFFE.MTLS.ComponentIDTemplate
	if template == "" {
		return "", false
	}
	return strings.ReplaceAll(template, componentPlaceholder, component), true
}

// ArtifactServerIDs returns the SPIFFE IDs of the source-controller and
// source-watcher artifact servers.
func ArtifactServerIDs(instance *fluxcdv1.FluxInstance) []string {
	if !MTLSEnabled(instance) {
		return nil
	}

	components := instance.GetComponents()
	var ids []string
	for _, component := range []string{fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher} {
		if !slices.Contains(components, component) {
			continue
		}
		if id, ok := ComponentID(instance, component); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// NotificationServerID returns the SPIFFE ID of the notification-controller
// event server.
func NotificationServerID(instance *fluxcdv1.FluxInstance) (string, bool) {
	if !slices.Contains(instance.GetComponents(), fluxcdv1.FluxNotificationController) {
		return "", false
	}
	return ComponentID(instance, fluxcdv1.FluxNotificationController)
}

// ArtifactServerPeers returns the SPIFFE IDs authorized to serve artifacts
// when mTLS is enabled on the FluxInstance. It returns nil when mTLS is
// disabled or no FluxInstance exists. The FluxInstance is fetched when
// instance is nil.
func ArtifactServerPeers(ctx context.Context, c client.Client, source fluxspiffe.TLSSource,
	instance *fluxcdv1.FluxInstance) ([]string, error) {
	instance, err := GetFluxInstance(ctx, c, instance)
	if errors.Is(err, ErrNoFluxInstance) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !MTLSEnabled(instance) {
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

// EventTLSConfig returns the TLS configuration for sending events to the
// notification-controller event server. It returns nil when mTLS is disabled.
func EventTLSConfig(source fluxspiffe.TLSSource, instance *fluxcdv1.FluxInstance) (*tls.Config, error) {
	if !MTLSEnabled(instance) {
		return nil, nil
	}
	if source == nil {
		return nil, errors.New("SPIFFE mTLS is enabled but the SPIFFE Workload API source is not configured")
	}

	id, ok := NotificationServerID(instance)
	if !ok {
		return nil, errors.New("SPIFFE mTLS is enabled but notification-controller is not installed")
	}
	return fluxspiffe.BuildTLSConfig(source, id)
}

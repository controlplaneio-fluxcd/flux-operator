// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package notifier

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"

	"github.com/fluxcd/pkg/runtime/events"
	fluxspiffe "github.com/fluxcd/pkg/spiffe"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	internalspiffe "github.com/controlplaneio-fluxcd/flux-operator/internal/spiffe"
)

// spiffeDisabledWarning ensures that a missing SPIFFE Workload API source is
// logged only once instead of on every event.
var spiffeDisabledWarning sync.Once

// Address returns the address of the notification-controller for
// the given namespace and clusterDomain for sending events.
func Address(namespace, clusterDomain string) string {
	return fmt.Sprintf("http://notification-controller.%s.svc.%s./", namespace, clusterDomain)
}

// options holds the settings of New.
type options struct {
	fluxInstance *fluxcdv1.FluxInstance
	client       client.Client
	spiffeSource fluxspiffe.TLSSource
}

// Option configures New.
type Option func(*options)

// WithFluxInstance sets the FluxInstance to read the notification-controller
// settings from. Without it, the FluxInstance is fetched with the client.
func WithFluxInstance(obj *fluxcdv1.FluxInstance) Option {
	return func(o *options) {
		o.fluxInstance = obj
	}
}

// WithClient sets the client used to fetch the FluxInstance.
func WithClient(c client.Client) Option {
	return func(o *options) {
		o.client = c
	}
}

// WithSPIFFESource sets the SPIFFE source used for mTLS with
// notification-controller.
func WithSPIFFESource(source fluxspiffe.TLSSource) Option {
	return func(o *options) {
		o.spiffeSource = source
	}
}

// recorderKey holds the settings a recorder was built with.
type recorderKey struct {
	scheme     *runtime.Scheme
	eventsAddr string
	// mtls is empty for plain HTTP, otherwise the notification-controller
	// SPIFFE ID, prefixed with the local trust domain for spiffe://self IDs.
	mtls string
}

// recorderCacheEntry holds the events.Recorder built for a base recorder.
type recorderCacheEntry struct {
	key      recorderKey
	recorder events.Recorder
}

// recorderCache holds one recorder per base recorder, so events reuse
// its HTTP connections.
var recorderCache = struct {
	mu      sync.Mutex
	entries map[record.EventRecorder]recorderCacheEntry
}{entries: map[record.EventRecorder]recorderCacheEntry{}}

// cacheable reports whether the base recorder can be used as a map key.
func cacheable(base record.EventRecorder) bool {
	return base != nil && reflect.TypeOf(base).Comparable()
}

// New returns an events.Recorder that records Kubernetes events with the
// base recorder and forwards them to notification-controller, over SPIFFE
// mTLS when the FluxInstance status reports it as applied. If the mTLS
// settings can't be built, events are only recorded in Kubernetes. The
// recorder is reused while the address and mTLS settings are unchanged.
func New(ctx context.Context, base record.EventRecorder, scheme *runtime.Scheme, opts ...Option) events.Recorder {
	log := ctrl.LoggerFrom(ctx)

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	fluxInstance, eventsAddr := eventsAddress(ctx, o)

	var peer, mtlsKey string
	if fluxInstance != nil && eventsAddr != "" {
		var err error
		peer, mtlsKey, err = eventMTLS(o.spiffeSource, fluxInstance)
		if err != nil {
			spiffeDisabledWarning.Do(func() {
				log.Error(err, "external events are disabled until the SPIFFE configuration is fixed")
			})
			eventsAddr, peer, mtlsKey = "", "", ""
		}
	}

	build := func() (events.Recorder, bool) {
		return buildRecorder(log, base, scheme, eventsAddr, peer, o.spiffeSource)
	}
	if !cacheable(base) {
		er, _ := build()
		return er
	}

	// Building does no network I/O, so hold the lock to give concurrent
	// callers the same recorder.
	key := recorderKey{scheme: scheme, eventsAddr: eventsAddr, mtls: mtlsKey}
	recorderCache.mu.Lock()
	defer recorderCache.mu.Unlock()
	if entry, ok := recorderCache.entries[base]; ok && entry.key == key {
		return entry.recorder
	}
	er, ok := build()
	if ok {
		recorderCache.entries[base] = recorderCacheEntry{key: key, recorder: er}
	}
	return er
}

// buildRecorder creates a recorder for the address, using mTLS when peer is
// set. It returns false when it falls back to Kubernetes-only events, which
// must not be cached.
func buildRecorder(log logr.Logger, base record.EventRecorder, scheme *runtime.Scheme,
	eventsAddr, peer string, source fluxspiffe.TLSSource) (events.Recorder, bool) {
	ok := true
	var tlsConfig *tls.Config
	if peer != "" {
		var err error
		tlsConfig, err = fluxspiffe.BuildTLSConfig(source, peer)
		if err != nil {
			spiffeDisabledWarning.Do(func() {
				log.Error(err, "external events are disabled until the SPIFFE configuration is fixed")
			})
			eventsAddr, tlsConfig, ok = "", nil, false
		}
	}

	// The recorder outlives the request, so it can't use the request logger.
	recorderLog := ctrl.Log.WithName("event-recorder")
	er, err := newRecorder(recorderLog, base, scheme, eventsAddr, tlsConfig)
	if err != nil {
		log.Error(err, "failed to create event recorder")
		// Fall back to recording the events only in Kubernetes.
		if er, err = newRecorder(recorderLog, base, scheme, "", nil); err != nil {
			log.Error(err, "failed to create Kubernetes event recorder")
			return events.NewNopRecorder(), false
		}
		return er, false
	}
	return er, ok
}

// eventMTLS returns the notification-controller SPIFFE ID and the mTLS part
// of the recorder key, both empty when mTLS is not applied.
func eventMTLS(source fluxspiffe.TLSSource, fluxInstance *fluxcdv1.FluxInstance) (string, string, error) {
	peer, err := internalspiffe.EventServerPeer(source, fluxInstance)
	if err != nil || peer == "" {
		return "", "", err
	}
	if !internalspiffe.IsSelfTrustDomainPeer(peer) {
		return peer, peer, nil
	}
	// spiffe://self IDs are bound to the local trust domain at build time.
	svid, err := source.GetX509SVID()
	if err != nil {
		return "", "", fmt.Errorf("failed to get SPIFFE X.509-SVID: %w", err)
	}
	return peer, svid.ID.TrustDomain().String() + "|" + peer, nil
}

// newRecorder creates an events.Recorder that records core/v1 events with the
// base recorder and posts them to the given notification-controller address.
func newRecorder(log logr.Logger, base record.EventRecorder, scheme *runtime.Scheme,
	eventsAddr string, tlsConfig *tls.Config) (events.Recorder, error) {
	const reportingController = "flux-operator"
	opts := []events.RecorderOption{
		events.WithScheme(scheme),
		events.WithLegacyEventRecorder(base),
	}
	if tlsConfig != nil {
		opts = append(opts, events.WithTLSConfig(tlsConfig))
	}
	return events.NewRecorder(log, eventsAddr, reportingController, opts...)
}

// installedComponents returns the applied components, or the spec
// components before the first apply.
func installedComponents(fluxInstance *fluxcdv1.FluxInstance) []string {
	if components := internalspiffe.AppliedComponents(fluxInstance); len(components) > 0 {
		return components
	}
	return fluxInstance.GetComponents()
}

// eventsAddress returns the FluxInstance and the notification-controller
// address, taken from NOTIFICATION_CONTROLLER_ADDRESS or derived from the
// FluxInstance. The address is empty when notifications are disabled or
// notification-controller isn't installed.
func eventsAddress(ctx context.Context, o options) (*fluxcdv1.FluxInstance, string) {
	log := ctrl.LoggerFrom(ctx)

	if os.Getenv("NOTIFICATIONS_DISABLED") != "" {
		return nil, ""
	}

	fluxInstance, err := internalspiffe.GetFluxInstance(ctx, o.client, o.fluxInstance)
	if err != nil && !errors.Is(err, internalspiffe.ErrNoFluxInstance) {
		log.Error(err, "failed to get flux instance")
	}

	eventsAddr := os.Getenv("NOTIFICATION_CONTROLLER_ADDRESS")
	if eventsAddr != "" {
		return fluxInstance, eventsAddr
	}
	if fluxInstance == nil {
		return nil, ""
	}
	if slices.Contains(installedComponents(fluxInstance), fluxcdv1.FluxNotificationController) {
		return fluxInstance, Address(fluxInstance.GetNamespace(), fluxInstance.GetCluster().Domain)
	}
	return fluxInstance, ""
}

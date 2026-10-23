// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package notifier

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
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

type options struct {
	fluxInstance *fluxcdv1.FluxInstance
	client       client.Client
	spiffeSource fluxspiffe.TLSSource
}

type Option func(*options)

func WithFluxInstance(obj *fluxcdv1.FluxInstance) Option {
	return func(o *options) {
		o.fluxInstance = obj
	}
}

func WithClient(c client.Client) Option {
	return func(o *options) {
		o.client = c
	}
}

// WithSPIFFESource sets the SPIFFE Workload API source used to build the
// TLS configuration for the notification-controller when FluxInstance mTLS
// is enabled.
func WithSPIFFESource(source fluxspiffe.TLSSource) Option {
	return func(o *options) {
		o.spiffeSource = source
	}
}

// New returns an events.Recorder that records Kubernetes events with the
// base recorder and forwards them to notification-controller when the
// controller address can be resolved from the FluxInstance. If the address
// can't be resolved, the events are only recorded in Kubernetes.
func New(ctx context.Context, base record.EventRecorder, scheme *runtime.Scheme, opts ...Option) events.Recorder {
	log := ctrl.LoggerFrom(ctx)

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	fluxInstance, eventsAddr := eventsAddress(ctx, o)

	var tlsConfig *tls.Config
	if fluxInstance != nil && eventsAddr != "" && internalspiffe.MTLSEnabled(fluxInstance) {
		var err error
		tlsConfig, err = internalspiffe.EventTLSConfig(o.spiffeSource, fluxInstance)
		if err != nil {
			spiffeDisabledWarning.Do(func() {
				log.Error(err, "external events are disabled until the SPIFFE configuration is fixed")
			})
			eventsAddr = ""
		}
	}

	er, err := newRecorder(log, base, scheme, eventsAddr, tlsConfig)
	if err != nil {
		log.Error(err, "failed to create event recorder")
		// Fall back to recording the events only in Kubernetes.
		if er, err = newRecorder(log, base, scheme, "", nil); err != nil {
			log.Error(err, "failed to create Kubernetes event recorder")
			return events.NewNopRecorder()
		}
	}

	return er
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

// eventsAddress returns the notification-controller address from the
// environment or from the FluxInstance. It returns an empty string if
// notifications are disabled or the address can't be resolved.
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
	if slices.Contains(fluxInstance.GetComponents(), fluxcdv1.FluxNotificationController) {
		return fluxInstance, Address(fluxInstance.GetNamespace(), fluxInstance.GetCluster().Domain)
	}
	return fluxInstance, ""
}

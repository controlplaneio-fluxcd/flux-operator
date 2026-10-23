// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package notifier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/fluxcd/pkg/runtime/events"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Address returns the address of the notification-controller for
// the given namespace and clusterDomain for sending events.
func Address(namespace, clusterDomain string) string {
	return fmt.Sprintf("http://notification-controller.%s.svc.%s./", namespace, clusterDomain)
}

type options struct {
	fluxInstance *fluxcdv1.FluxInstance
	client       client.Client
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

	eventsAddr := eventsAddress(ctx, o)
	er, err := newRecorder(log, base, scheme, eventsAddr)
	if err != nil {
		log.Error(err, "failed to create event recorder")
		// Fall back to recording the events only in Kubernetes.
		if er, err = newRecorder(log, base, scheme, ""); err != nil {
			log.Error(err, "failed to create Kubernetes event recorder")
			return events.NewNopRecorder()
		}
	}

	return er
}

// newRecorder creates an events.Recorder that records core/v1 events with the
// base recorder and posts them to the given notification-controller address.
func newRecorder(log logr.Logger, base record.EventRecorder, scheme *runtime.Scheme, eventsAddr string) (events.Recorder, error) {
	const reportingController = "flux-operator"
	return events.NewRecorder(log, eventsAddr, reportingController,
		events.WithScheme(scheme), events.WithLegacyEventRecorder(base))
}

// eventsAddress returns the notification-controller address from the
// environment or from the FluxInstance. It returns an empty string if
// notifications are disabled or the address can't be resolved.
func eventsAddress(ctx context.Context, o options) string {
	log := ctrl.LoggerFrom(ctx)

	eventsAddr := os.Getenv("NOTIFICATION_CONTROLLER_ADDRESS")
	if eventsAddr != "" || os.Getenv("NOTIFICATIONS_DISABLED") != "" {
		return eventsAddr
	}

	fluxInstance := o.fluxInstance
	if fluxInstance == nil {
		if o.client == nil {
			const msg = "flux instance object or client is required"
			log.Error(errors.New(msg), msg)
			return ""
		}
		var instanceList fluxcdv1.FluxInstanceList
		if err := o.client.List(ctx, &instanceList); err != nil {
			log.Error(err, "failed to list flux instances")
			return ""
		}
		if len(instanceList.Items) == 0 {
			const msg = "no flux instances found"
			log.Error(errors.New(msg), msg)
			return ""
		}
		if len(instanceList.Items) > 1 {
			const msg = "multiple flux instances found, only one is supported"
			log.Error(errors.New(msg), msg)
			return ""
		}
		fluxInstance = &instanceList.Items[0]
	}
	if slices.Contains(fluxInstance.GetComponents(), "notification-controller") {
		return Address(fluxInstance.GetNamespace(), fluxInstance.GetCluster().Domain)
	}
	return ""
}

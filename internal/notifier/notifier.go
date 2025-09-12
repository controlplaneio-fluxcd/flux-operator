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

func New(ctx context.Context, base record.EventRecorder, scheme *runtime.Scheme, opts ...Option) record.EventRecorder {
	log := ctrl.LoggerFrom(ctx)

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	// Figure out the notification-controller address from the flux instance.
	var eventsAddr string
	if os.Getenv("NOTIFICATIONS_DISABLED") == "" {
		fluxInstance := o.fluxInstance
		if fluxInstance == nil {
			if o.client == nil {
				const msg = "flux instance object or client is required"
				log.Error(errors.New(msg), msg)
				return nilNotifier{}
			}
			var instanceList fluxcdv1.FluxInstanceList
			if err := o.client.List(ctx, &instanceList); err != nil {
				log.Error(err, "failed to list flux instances")
				return nilNotifier{}
			}
			if len(instanceList.Items) == 0 {
				const msg = "no flux instances found"
				log.Error(errors.New(msg), msg)
				return nilNotifier{}
			}
			if len(instanceList.Items) > 1 {
				const msg = "multiple flux instances found, only one is supported"
				log.Error(errors.New(msg), msg)
				return nilNotifier{}
			}
			fluxInstance = &instanceList.Items[0]
		}
		if slices.Contains(fluxInstance.GetComponents(), "notification-controller") {
			eventsAddr = Address(fluxInstance.GetNamespace(), fluxInstance.GetCluster().Domain)
		}
	}

	// Create the event recorder.
	const reportingController = "flux-operator"
	er, err := events.NewRecorderForScheme(scheme, base, log, eventsAddr, reportingController)
	if err != nil {
		log.Error(err, "failed to create event recorder")
		return nilNotifier{}
	}

	return er
}

type nilNotifier struct{}

// AnnotatedEventf implements record.EventRecorder.
func (nilNotifier) AnnotatedEventf(runtime.Object, map[string]string, string, string, string, ...any) {
}

// Event implements record.EventRecorder.
func (nilNotifier) Event(runtime.Object, string, string, string) {
}

// Eventf implements record.EventRecorder.
func (nilNotifier) Eventf(runtime.Object, string, string, string, ...any) {
}

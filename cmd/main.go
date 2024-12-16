// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	runtimeCtrl "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/logger"
	"github.com/fluxcd/pkg/runtime/pprof"
	"github.com/fluxcd/pkg/runtime/probes"
	flag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcfg "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/controller"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	// +kubebuilder:scaffold:imports
)

const controllerName = "flux-operator"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(fluxcdv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		concurrent           int
		metricsAddr          string
		healthAddr           string
		enableLeaderElection bool
		logOptions           logger.Options
		rateLimiterOptions   runtimeCtrl.RateLimiterOptions
		storagePath          string
	)

	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent kustomize reconciles.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":8081", "The address the health endpoint binds to.")
	flag.StringVar(&storagePath, "storage-path", "/data", "The local storage path.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	logOptions.BindFlags(flag.CommandLine)
	rateLimiterOptions.BindFlags(flag.CommandLine)

	flag.Parse()

	logger.SetLogger(logger.NewLogger(logOptions))

	runtimeNamespace := os.Getenv("RUNTIME_NAMESPACE")
	if runtimeNamespace == "" {
		runtimeNamespace = fluxcdv1.DefaultNamespace
		setupLog.Info("RUNTIME_NAMESPACE env var not set, defaulting to " + fluxcdv1.DefaultNamespace)
	}

	reporter.MustRegisterMetrics()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			ExtraHandlers: pprof.GetHandlers(),
		},
		HealthProbeBindAddress:        healthAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              controllerName,
		LeaderElectionReleaseOnCancel: true,
		Controller: ctrlcfg.Controller{
			MaxConcurrentReconciles: concurrent,
			RecoverPanic:            ptr.To(true),
		},
		Client: ctrlclient.Options{
			Cache: &ctrlclient.CacheOptions{
				DisableFor: []ctrlclient.Object{&corev1.Secret{}, &corev1.ConfigMap{}},
			},
		},
		Cache: ctrlcache.Options{
			ByObject: map[ctrlclient.Object]ctrlcache.ByObject{
				&fluxcdv1.FluxInstance{}: {
					// Only the FluxInstance with the name 'flux' can be reconciled.
					Field: fields.SelectorFromSet(fields.Set{
						"metadata.name":      fluxcdv1.DefaultInstanceName,
						"metadata.namespace": runtimeNamespace,
					}),
				},
				&fluxcdv1.FluxReport{}: {
					// Only the FluxReport with the name 'flux' can be reconciled.
					Field: fields.SelectorFromSet(fields.Set{
						"metadata.name":      fluxcdv1.DefaultInstanceName,
						"metadata.namespace": runtimeNamespace,
					}),
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	entitlementClient, err := entitlement.NewClient()
	if err != nil {
		setupLog.Error(err, "unable to create entitlement client")
		os.Exit(1)
	}

	if err = (&controller.EntitlementReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		StatusPoller:      polling.NewStatusPoller(mgr.GetClient(), mgr.GetRESTMapper(), polling.Options{}),
		StatusManager:     controllerName,
		EventRecorder:     mgr.GetEventRecorderFor(controllerName),
		WatchNamespace:    runtimeNamespace,
		EntitlementClient: entitlementClient,
	}).SetupWithManager(mgr,
		controller.EntitlementReconcilerOptions{
			RateLimiter: runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Entitlement")
		os.Exit(1)
	}

	if err = (&controller.FluxInstanceReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		StatusPoller:  polling.NewStatusPoller(mgr.GetClient(), mgr.GetRESTMapper(), polling.Options{}),
		StoragePath:   storagePath,
		StatusManager: controllerName,
		EventRecorder: mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManager(mgr,
		controller.FluxInstanceReconcilerOptions{
			RateLimiter: runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", fluxcdv1.FluxInstanceKind)
		os.Exit(1)
	}

	if err = (&controller.FluxInstanceArtifactReconciler{
		Client:        mgr.GetClient(),
		StatusManager: controllerName,
		EventRecorder: mgr.GetEventRecorderFor(controllerName),
	}).SetupWithManager(mgr,
		controller.FluxInstanceArtifactReconcilerOptions{
			RateLimiter: runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", fluxcdv1.FluxInstanceKind+"Artifact")
		os.Exit(1)
	}

	if err = (&controller.FluxReportReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		StatusManager:  controllerName,
		EventRecorder:  mgr.GetEventRecorderFor(controllerName),
		WatchNamespace: runtimeNamespace,
	}).SetupWithManager(mgr,
		controller.FluxReportReconcilerOptions{
			RateLimiter: runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", fluxcdv1.FluxReportKind)
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	probes.SetupChecks(mgr, setupLog)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

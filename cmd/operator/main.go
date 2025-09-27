// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"crypto/fips140"
	"errors"
	"os"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/clusterreader"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
	runtimeCtrl "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/jitter"
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

var (
	VERSION  = "0.0.0-dev.0"
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
	const (
		controllerName                              = "flux-operator"
		defaultServiceAccountEnvKey                 = "DEFAULT_SERVICE_ACCOUNT"
		defaultWorkloadIdentityServiceAccountEnvKey = "DEFAULT_WORKLOAD_IDENTITY_SERVICE_ACCOUNT"
		reportingIntervalEnvKey                     = "REPORTING_INTERVAL"
		runtimeNamespaceEnvKey                      = "RUNTIME_NAMESPACE"
		tokenCacheDefaultMaxSize                    = 100
	)

	var (
		concurrent                            int
		reportingInterval                     time.Duration
		requeueDependency                     time.Duration
		tokenCacheOptions                     cache.TokenFlags
		metricsAddr                           string
		healthAddr                            string
		enableLeaderElection                  bool
		logOptions                            logger.Options
		rateLimiterOptions                    runtimeCtrl.RateLimiterOptions
		intervalJitterOptions                 jitter.IntervalOptions
		storagePath                           string
		defaultServiceAccount                 string
		defaultWorkloadIdentityServiceAccount string
		watchOptions                          runtimeCtrl.WatchOptions
	)

	flag.IntVar(&concurrent, "concurrent", 10,
		"The number of concurrent resource reconciles.")
	flag.DurationVar(&reportingInterval, "reporting-interval", 5*time.Minute,
		"The interval at which the report is computed.")
	flag.DurationVar(&requeueDependency, "requeue-dependency", 5*time.Second,
		"The interval at which failing dependencies are reevaluated.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080",
		"The address the metric endpoint binds to.")
	flag.StringVar(&healthAddr, "health-addr", ":8081",
		"The address the health endpoint binds to.")
	flag.StringVar(&storagePath, "storage-path", "/data",
		"The local storage path.")
	flag.StringVar(&defaultServiceAccount, "default-service-account", "",
		"Default service account used for impersonation.")
	flag.StringVar(&defaultWorkloadIdentityServiceAccount, "default-workload-identity-service-account", "",
		"Default service account to use for workload identity when not specified in resources.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.CommandLine.StringVar(&watchOptions.ConfigsLabelSelector, "watch-configs-label-selector", meta.LabelKeyWatch+"="+meta.LabelValueWatchEnabled,
		"Watch for ConfigMaps and Secrets with matching labels.")

	tokenCacheOptions.BindFlags(flag.CommandLine, tokenCacheDefaultMaxSize)
	logOptions.BindFlags(flag.CommandLine)
	rateLimiterOptions.BindFlags(flag.CommandLine)
	intervalJitterOptions.BindFlags(flag.CommandLine)

	flag.Parse()

	logger.SetLogger(logger.NewLogger(logOptions))

	// Perform FIPS 140-3 check, will panic if integrity check fails.
	if fips140.Enabled() {
		setupLog.Info("Operating in FIPS 140-3 mode, integrity check passed")
	} else {
		setupLog.Error(errors.New("FIPS 140-3 mode disabled"), "Operating in non-FIPS mode is not supported")
		os.Exit(1)
	}

	runtimeNamespace := os.Getenv(runtimeNamespaceEnvKey)
	if runtimeNamespace == "" {
		runtimeNamespace = fluxcdv1.DefaultNamespace
		setupLog.Info("RUNTIME_NAMESPACE env var not set, defaulting to " + fluxcdv1.DefaultNamespace)
	}

	if err := intervalJitterOptions.SetGlobalJitter(nil); err != nil {
		setupLog.Error(err, "unable to set global jitter")
		os.Exit(1)
	}

	watchConfigsPredicate, err := runtimeCtrl.GetWatchConfigsPredicate(watchOptions)
	if err != nil {
		setupLog.Error(err, "unable to configure watch configs label selector for controller")
		os.Exit(1)
	}

	// Allow the default service account names to be set by the environment variables.
	// Needed for the OLM Subscription that only allows env var configuration.
	if s := os.Getenv(defaultServiceAccountEnvKey); s != "" {
		defaultServiceAccount = s
	}
	if s := os.Getenv(defaultWorkloadIdentityServiceAccountEnvKey); s != "" {
		defaultWorkloadIdentityServiceAccount = s
	}

	// auth setup.
	auth.EnableObjectLevelWorkloadIdentity()
	if s := defaultWorkloadIdentityServiceAccount; s != "" {
		auth.SetDefaultServiceAccount(s)
	}

	// Allow the reporting interval to be set by an environment variable.
	reportingIntervalEnv := os.Getenv(reportingIntervalEnvKey)
	if reportingIntervalEnv != "" {
		d, err := time.ParseDuration(reportingIntervalEnv)
		if err != nil {
			setupLog.Error(err, "unable to parse reporting interval", "value", reportingIntervalEnv)
			os.Exit(1)
		}
		reportingInterval = d
	}

	// Disable the status poller cache to reduce memory usage.
	clusterReader := engine.ClusterReaderFactoryFunc(clusterreader.NewDirectClusterReader)

	reporter.MustRegisterMetrics()

	ctx := ctrl.SetupSignalHandler()
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

	var tokenCache *cache.TokenCache
	if tokenCacheOptions.MaxSize > 0 {
		tokenCache, err = cache.NewTokenCache(tokenCacheOptions.MaxSize,
			cache.WithMaxDuration(tokenCacheOptions.MaxDuration),
			cache.WithMetricsRegisterer(reporter.Registerer()),
			cache.WithMetricsPrefix("flux_token_"),
			cache.WithEventNamespaceLabel("exported_namespace"))
		if err != nil {
			setupLog.Error(err, "unable to create token cache")
			os.Exit(1)
		}
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
		ClusterReader: clusterReader,
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
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		StatusManager:     controllerName,
		EventRecorder:     mgr.GetEventRecorderFor(controllerName),
		WatchNamespace:    runtimeNamespace,
		ReportingInterval: reportingInterval,
		Version:           VERSION,
	}).SetupWithManager(mgr,
		controller.FluxReportReconcilerOptions{
			RateLimiter: runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", fluxcdv1.FluxReportKind)
		os.Exit(1)
	}

	if err = (&controller.ResourceSetReconciler{
		Client:                mgr.GetClient(),
		APIReader:             mgr.GetAPIReader(),
		Scheme:                mgr.GetScheme(),
		ClusterReader:         clusterReader,
		StatusManager:         controllerName,
		EventRecorder:         mgr.GetEventRecorderFor(controllerName),
		DefaultServiceAccount: defaultServiceAccount,
		RequeueDependency:     requeueDependency,
	}).SetupWithManager(ctx, mgr,
		controller.ResourceSetReconcilerOptions{
			RateLimiter:           runtimeCtrl.GetRateLimiter(rateLimiterOptions),
			WatchConfigsPredicate: watchConfigsPredicate,
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", fluxcdv1.ResourceSetKind)
		os.Exit(1)
	}

	if err = (&controller.ResourceSetInputProviderReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		StatusManager: controllerName,
		EventRecorder: mgr.GetEventRecorderFor(controllerName),
		TokenCache:    tokenCache,
	}).SetupWithManager(mgr,
		controller.ResourceSetInputProviderReconcilerOptions{
			RateLimiter: runtimeCtrl.GetRateLimiter(rateLimiterOptions),
		}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", fluxcdv1.ResourceSetInputProviderKind)
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	probes.SetupChecks(mgr, setupLog)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

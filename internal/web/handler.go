// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
)

// Handler provides HTTP handlers for the API endpoints and SPA static files.
type Handler struct {
	conf          *fluxcdv1.WebConfigSpec
	kubeClient    *kubeclient.Client
	eventRecorder record.EventRecorder
	version       string
	statusManager string
	namespace     string

	// Report cache
	reportCache                 *unstructured.Unstructured
	reportCacheStatsByNamespace []reporter.ReconcilerStatsByNamespace
	reportCacheMu               sync.RWMutex

	// Search index
	searchIndex *SearchIndex

	// Workload index
	workloadIndex *WorkloadIndex

	// Pod metrics ring buffer
	metrics *MetricsCollector
}

// NewHandler creates a new handler for the web server.
// It also fires off goroutines to perform background
// tasks required for handling requests, such as caching
// the report periodically. They run until the context
// is canceled. The returned channel is closed when all
// the goroutines have stopped.
func NewHandler(ctx context.Context, conf *fluxcdv1.WebConfigSpec, spaHandler http.Handler, kubeClient *kubeclient.Client,
	version, statusManager, namespace string, reportInterval time.Duration, eventRecorder record.EventRecorder,
	authMiddleware func(http.Handler) http.Handler, l logr.Logger) (http.Handler, <-chan struct{}) {

	// Build the Handler struct.
	h := &Handler{
		conf:          conf,
		kubeClient:    kubeClient,
		eventRecorder: eventRecorder,
		version:       version,
		statusManager: statusManager,
		namespace:     namespace,
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
	}

	// The pod metrics are read with the privileged client as they contain
	// no sensitive information, following the same reasoning as the Flux
	// controller metrics in the report. Access remains bounded because the
	// workload endpoint fetches the workload with the user's client before
	// attaching any metrics. When collection is disabled in the config,
	// the collector stays nil and all enrichment paths are skipped.
	if conf.MetricsEnabled() {
		// Request protobuf encoding: the cluster-wide list is large on
		// big clusters and protobuf cuts both the response size and the
		// decode allocations several-fold compared to JSON. GetConfig
		// returns a copy, so the mutation does not leak elsewhere. The
		// clientset is created once so its transport is reused across
		// scrapes.
		config := kubeClient.GetConfig(ctx, kubeclient.WithPrivileges())
		config.ContentType = runtime.ContentTypeProtobuf
		if clientset, err := metricsclientset.NewForConfig(config); err != nil {
			l.Error(err, "pod metrics collection disabled, failed to create metrics client")
		} else {
			h.metrics = NewMetricsCollector(func(ctx context.Context) (*metricsv1beta1api.PodMetricsList, error) {
				return clientset.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			}, conf.MetricsScrapeInterval())
		}
	}

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Handle SPA.
	mux.Handle("/", spaHandler)

	// Handle API.
	mux.HandleFunc("GET /api/v1/artifact/download", h.DownloadHandler)
	mux.HandleFunc("GET /api/v1/events", h.EventsHandler)
	mux.HandleFunc("POST /api/v1/favorites", h.FavoritesHandler)
	mux.HandleFunc("POST /api/v1/inventory/objects", h.InventoryObjectsHandler)
	mux.HandleFunc("GET /api/v1/report", h.ReportHandler)
	mux.HandleFunc("GET /api/v1/resource", h.ResourceHandler)
	mux.HandleFunc("POST /api/v1/resource/action", h.ActionHandler)
	mux.HandleFunc("GET /api/v1/resources", h.ResourcesHandler)
	mux.HandleFunc("GET /api/v1/search", h.SearchHandler)
	mux.HandleFunc("GET /api/v1/workload", h.WorkloadHandler)
	mux.HandleFunc("GET /api/v1/workload/logs", h.WorkloadLogsHandler)
	mux.HandleFunc("POST /api/v1/workload/action", h.WorkloadActionHandler)
	mux.HandleFunc("GET /api/v1/workloads", h.WorkloadsListHandler)
	mux.HandleFunc("GET /api/v1/workloads/search", h.WorkloadsSearchHandler)
	mux.HandleFunc("POST /api/v1/workloads", h.WorkloadsHandler)

	// Wrap the mux with middlewares to produce the final handler.
	// Limit request body size to 1MB to prevent abuse on POST endpoints.
	// The cross-origin checks wrap the auth middleware so that they also
	// cover the logout endpoint it serves.
	handler := LoggingMiddleware(l, SecurityHeadersMiddleware(
		GzipMiddleware(CacheControlMiddleware(MaxBodySizeMiddleware(1<<20)(
			CrossOriginMiddleware(authMiddleware(mux)))))))

	// Start the background goroutines and merge their stop channels.
	reportStopped := h.startReportCache(ctx, reportInterval)
	if h.metrics == nil {
		return handler, reportStopped
	}
	metricsStopped := h.metrics.Start(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-reportStopped
		<-metricsStopped
	}()

	return handler, stopped
}

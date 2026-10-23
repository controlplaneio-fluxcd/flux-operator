// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	fluxspiffe "github.com/fluxcd/pkg/spiffe"
	"github.com/go-logr/logr"
	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
)

// Handler provides HTTP handlers for the API endpoints and SPA static files.
type Handler struct {
	conf          *fluxcdv1.WebConfigSpec
	kubeClient    *kubeclient.Client
	eventRecorder record.EventRecorder
	spiffeSource  fluxspiffe.TLSSource
	version       string
	statusManager string
	namespace     string

	// Report cache
	reportCache                 *unstructured.Unstructured
	reportCacheStatsByNamespace []reporter.ReconcilerStatsByNamespace
	reportCacheMu               sync.RWMutex

	// reportBuildFlight shares one report build among the requests
	// that find the cache empty.
	reportBuildFlight singleflight.Group

	// buildReportHook replaces buildReport when set, used in tests.
	buildReportHook func(ctx context.Context) (*unstructured.Unstructured, *reporter.FluxStatusReport, *NodesSnapshot, error)

	// Search index
	searchIndex *SearchIndex

	// Workload index
	workloadIndex *WorkloadIndex

	// Pod metrics ring buffer
	metrics *MetricsCollector

	// Node Ready transitions tracker, may be nil.
	nodeTransitions *ReadyTransitionTracker

	// Nodes snapshot cache, refreshed with the report.
	nodesSnapshot *NodesSnapshot

	// nodeLeasesFailing is set while the node Leases can't be listed,
	// so that the error is logged once per failure streak.
	nodeLeasesFailing atomic.Bool
}

// HandlerOption configures the web server Handler.
type HandlerOption func(*Handler)

// withSPIFFESource sets the SPIFFE Workload API source used to download
// artifacts from Flux artifact servers when FluxInstance mTLS is enabled.
func withSPIFFESource(source fluxspiffe.TLSSource) HandlerOption {
	return func(h *Handler) {
		h.spiffeSource = source
	}
}

// NewHandler creates a new handler for the web server.
// It also fires off goroutines to perform background
// tasks required for handling requests, such as caching
// the report periodically. They run until the context
// is canceled. The returned channel is closed when all
// the goroutines have stopped.
// The metrics collector is owned by the caller and may be nil
// when pod metrics collection is disabled. The node Ready transitions
// tracker is owned by the caller and may be nil.
func NewHandler(ctx context.Context, conf *fluxcdv1.WebConfigSpec, spaHandler http.Handler, kubeClient *kubeclient.Client,
	metrics *MetricsCollector, nodeTransitions *ReadyTransitionTracker, version, statusManager, namespace string, reportInterval time.Duration,
	eventRecorder record.EventRecorder, authMiddleware func(http.Handler) http.Handler, l logr.Logger,
	opts ...HandlerOption) (http.Handler, <-chan struct{}) {

	// Build the Handler struct.
	h := &Handler{
		conf:            conf,
		kubeClient:      kubeClient,
		eventRecorder:   eventRecorder,
		version:         version,
		statusManager:   statusManager,
		namespace:       namespace,
		searchIndex:     &SearchIndex{},
		workloadIndex:   &WorkloadIndex{},
		metrics:         metrics,
		nodeTransitions: nodeTransitions,
	}
	for _, opt := range opts {
		opt(h)
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
	mux.HandleFunc("GET /api/v1/nodes", h.NodesHandler)
	mux.HandleFunc("GET /api/v1/nodes/workloads", h.NodesWorkloadsHandler)
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

	// The report cache is the only goroutine.
	stopped := h.startReportCache(ctx, reportInterval)

	return handler, stopped
}

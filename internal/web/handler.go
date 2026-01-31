// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
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
	version       string
	statusManager string
	namespace     string

	// Report cache
	reportCache                 *unstructured.Unstructured
	reportCacheStatsByNamespace []reporter.ReconcilerStatsByNamespace
	reportCacheMu               sync.RWMutex
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
	}

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Handle SPA.
	mux.Handle("/", spaHandler)

	// Handle API.
	mux.HandleFunc("POST /api/v1/action", h.ActionHandler)
	mux.HandleFunc("GET /api/v1/download", h.DownloadHandler)
	mux.HandleFunc("GET /api/v1/events", h.EventsHandler)
	mux.HandleFunc("POST /api/v1/favorites", h.FavoritesHandler)
	mux.HandleFunc("GET /api/v1/report", h.ReportHandler)
	mux.HandleFunc("GET /api/v1/resource", h.ResourceHandler)
	mux.HandleFunc("GET /api/v1/resources", h.ResourcesHandler)
	mux.HandleFunc("GET /api/v1/search", h.SearchHandler)
	mux.HandleFunc("GET /api/v1/workload", h.WorkloadHandler)
	mux.HandleFunc("POST /api/v1/workloads", h.WorkloadsHandler)

	// Wrap the mux with middlewares to produce the final handler.
	handler := LoggingMiddleware(l, SecurityHeadersMiddleware(
		GzipMiddleware(CacheControlMiddleware(authMiddleware(mux)))))

	// The report cache is the only goroutine.
	stopped := h.startReportCache(ctx, reportInterval)

	return handler, stopped
}

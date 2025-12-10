// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Router provides HTTP handlers for the API endpoints and static files.
type Router struct {
	mux            *http.ServeMux
	kubeClient     *Client
	log            logr.Logger
	webFS          fs.FS
	version        string
	statusManager  string
	namespace      string
	authMiddleware func(http.Handler) http.Handler

	// Report cache
	reportCache    *unstructured.Unstructured
	reportCacheMu  sync.RWMutex
	reportInterval time.Duration
}

// NewRouter creates a new router with the given Kubernetes client and embedded filesystem.
func NewRouter(mux *http.ServeMux, webFS fs.FS, kubeClient *Client, log logr.Logger, version, statusManager, namespace string, reportInterval time.Duration, authMiddleware func(http.Handler) http.Handler) *Router {
	return &Router{
		mux:            mux,
		kubeClient:     kubeClient,
		log:            log,
		webFS:          webFS,
		version:        version,
		statusManager:  statusManager,
		namespace:      namespace,
		authMiddleware: authMiddleware,
		reportInterval: reportInterval,
	}
}

// RegisterRoutes registers all API and static file routes on the given mux.
func (r *Router) RegisterRoutes() {
	// Static file server for the frontend assets
	spaHandler := NewFileSystem(r.webFS)
	r.mux.Handle("/", http.FileServer(http.FS(spaHandler)))

	// API routes for the frontend to consume
	r.mux.HandleFunc("GET /api/v1/events", r.EventsHandler)
	r.mux.HandleFunc("POST /api/v1/favorites", r.FavoritesHandler)
	r.mux.HandleFunc("GET /api/v1/report", r.ReportHandler)
	r.mux.HandleFunc("GET /api/v1/resource", r.ResourceHandler)
	r.mux.HandleFunc("GET /api/v1/resources", r.ResourcesHandler)
	r.mux.HandleFunc("GET /api/v1/search", r.SearchHandler)
	r.mux.HandleFunc("GET /api/v1/workload", r.WorkloadHandler)
	r.mux.HandleFunc("POST /api/v1/workloads", r.WorkloadsHandler)
}

// RegisterMiddleware wraps the mux with logging, gzip compression, and cache control middleware.
func (r *Router) RegisterMiddleware() http.Handler {
	return LoggingMiddleware(r.log, GzipMiddleware(CacheControlMiddleware(r.authMiddleware(r.mux))))
}

// StartReportCache starts a background goroutine that periodically refreshes the report cache.
func (r *Router) StartReportCache(ctx context.Context) {
	// Build initial report synchronously
	r.refreshReportCache(ctx)

	go func() {
		ticker := time.NewTicker(r.reportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.refreshReportCache(ctx)
			}
		}
	}()
}

// refreshReportCache builds a fresh report and updates the cache.
func (r *Router) refreshReportCache(ctx context.Context) {
	report, err := r.buildReport(ctx)
	if err != nil {
		r.log.Error(err, "failed to refresh report cache")
		return
	}

	r.reportCacheMu.Lock()
	r.reportCache = report
	r.reportCacheMu.Unlock()
}

// getCachedReport returns the cached report if available.
func (r *Router) getCachedReport() *unstructured.Unstructured {
	r.reportCacheMu.RLock()
	defer r.reportCacheMu.RUnlock()
	return r.reportCache
}

// isAPIRequest returns true if the request is for an API endpoint.
func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

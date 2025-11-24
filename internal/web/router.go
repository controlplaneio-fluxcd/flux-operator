// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Router provides HTTP handlers for the API endpoints and static files.
type Router struct {
	mux           *http.ServeMux
	kubeReader    client.Reader
	kubeClient    client.Client
	kubeConfig    *rest.Config
	log           logr.Logger
	webFS         fs.FS
	version       string
	statusManager string
	namespace     string

	// Report cache
	reportCache    *unstructured.Unstructured
	reportCacheMu  sync.RWMutex
	reportInterval time.Duration
}

// NewRouter creates a new router with the given Kubernetes client and embedded filesystem.
func NewRouter(mux *http.ServeMux, webFS fs.FS, kubeReader client.Reader, kubeClient client.Client, kubeConfig *rest.Config, log logr.Logger, version, statusManager, namespace string, reportInterval time.Duration) *Router {
	return &Router{
		mux:            mux,
		kubeReader:     kubeReader,
		kubeClient:     kubeClient,
		kubeConfig:     kubeConfig,
		log:            log,
		webFS:          webFS,
		version:        version,
		statusManager:  statusManager,
		namespace:      namespace,
		reportInterval: reportInterval,
	}
}

// RegisterRoutes registers all API and static file routes on the given mux.
func (r *Router) RegisterRoutes() {
	// Static file server for the frontend assets
	spaHandler := NewFileSystem(r.webFS)
	r.mux.Handle("/", http.FileServer(http.FS(spaHandler)))

	// API routes for the frontend to consume
	r.mux.HandleFunc("GET /api/v1/report", r.ReportHandler)
	r.mux.HandleFunc("GET /api/v1/events", r.EventsHandler)
	r.mux.HandleFunc("GET /api/v1/resources", r.ResourcesHandler)
	r.mux.HandleFunc("GET /api/v1/resource", r.ResourceHandler)
	r.mux.HandleFunc("GET /api/v1/search", r.SearchHandler)
	r.mux.HandleFunc("GET /api/v1/workload", r.WorkloadHandler)
}

// RegisterMiddleware wraps the mux with logging and gzip compression middleware.
func (r *Router) RegisterMiddleware() http.Handler {
	return LoggingMiddleware(r.log, GzipMiddleware(r.mux))
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

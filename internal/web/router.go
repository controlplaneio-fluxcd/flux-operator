// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
)

// Router provides HTTP handlers for the API endpoints and static files.
type Router struct {
	mux            *http.ServeMux
	kubeClient     *kubeclient.Client
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
func NewRouter(mux *http.ServeMux, webFS fs.FS, kubeClient *kubeclient.Client, version, statusManager, namespace string, reportInterval time.Duration, authMiddleware func(http.Handler) http.Handler) *Router {
	return &Router{
		mux:            mux,
		kubeClient:     kubeClient,
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

// RegisterMiddleware wraps the mux with security headers, logging, gzip compression, and cache control middleware.
func (r *Router) RegisterMiddleware(l logr.Logger) http.Handler {
	return LoggingMiddleware(l, SecurityHeadersMiddleware(GzipMiddleware(CacheControlMiddleware(r.authMiddleware(r.mux)))))
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
		log.FromContext(ctx).Error(err, "failed to refresh report cache")
		return
	}

	r.reportCacheMu.Lock()
	r.reportCache = report
	r.reportCacheMu.Unlock()
}

// getCachedReport returns the cached report if available.
func (r *Router) getCachedReport() *unstructured.Unstructured {
	r.reportCacheMu.RLock()
	if r.reportCache == nil {
		r.reportCacheMu.RUnlock()
		return nil
	}
	b, _ := json.Marshal(r.reportCache)
	r.reportCacheMu.RUnlock()

	var obj unstructured.Unstructured
	_ = json.Unmarshal(b, &obj)
	return &obj
}

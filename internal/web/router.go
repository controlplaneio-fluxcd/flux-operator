// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"io/fs"
	"net/http"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Router provides HTTP handlers for the API endpoints and static files.
type Router struct {
	mux        *http.ServeMux
	kubeReader client.Reader
	kubeClient client.Client
	kubeConfig *rest.Config
	log        logr.Logger
	webFS      fs.FS
}

// NewRouter creates a new router with the given Kubernetes client and embedded filesystem.
func NewRouter(mux *http.ServeMux, webFS fs.FS, kubeReader client.Reader, kubeClient client.Client, kubeConfig *rest.Config, log logr.Logger) *Router {
	return &Router{
		mux:        mux,
		kubeReader: kubeReader,
		kubeClient: kubeClient,
		kubeConfig: kubeConfig,
		log:        log,
		webFS:      webFS,
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
}

// RegisterMiddleware wraps the mux with logging and gzip compression middleware.
func (r *Router) RegisterMiddleware() http.Handler {
	return LoggingMiddleware(r.log, GzipMiddleware(r.mux))
}

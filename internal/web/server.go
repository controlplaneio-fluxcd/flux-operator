// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/web"
)

func StartServer(ctx context.Context,
	timeout time.Duration,
	port int,
	kubeClient *kubeclient.Client,
	l logr.Logger,
	version, statusManager, namespace string,
	reportInterval time.Duration,
	authMiddleware func(http.Handler) http.Handler) error {

	// Create HTTP request multiplexer
	mux := http.NewServeMux()

	// Create router with embedded filesystem and register routes
	router := NewRouter(mux, web.GetFS(), kubeClient, version, statusManager, namespace, reportInterval, authMiddleware)
	router.RegisterRoutes()

	// Start background report cache refresh
	router.StartReportCache(log.IntoContext(ctx, l.WithValues("background", true)))

	// Create HTTP server with timeouts
	addr := fmt.Sprintf(":%d", port)
	webServer := &http.Server{
		Addr:         addr,
		Handler:      router.RegisterMiddleware(l),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		IdleTimeout:  timeout,
	}

	// Start server in a goroutine
	go func() {
		l.Info("Starting web server", "port", port)
		if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Error(err, "Failed to start web server")
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	l.Info("Shutdown signal received, gracefully stopping web server")

	// Create a context with timeout for graceful shutdown
	ctxShutdown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Shutdown the web server
	if err := webServer.Shutdown(ctxShutdown); err != nil {
		l.Error(err, "Error during graceful shutdown")
		return err
	}

	l.Info("Web server stopped")
	return nil
}

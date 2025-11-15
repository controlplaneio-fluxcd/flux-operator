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
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/controlplaneio-fluxcd/flux-operator/web"
)

func StartServer(ctx context.Context,
	timeout time.Duration,
	port int,
	kubeReader client.Reader,
	kubeClient client.Client,
	kubeConfig *rest.Config,
	log logr.Logger) error {

	// Create HTTP request multiplexer
	mux := http.NewServeMux()

	// Create router with embedded filesystem and register routes
	router := NewRouter(mux, web.GetFS(), kubeReader, kubeClient, kubeConfig, log)
	router.RegisterRoutes()

	// Create HTTP server with timeouts
	addr := fmt.Sprintf(":%d", port)
	webServer := &http.Server{
		Addr:         addr,
		Handler:      router.RegisterMiddleware(),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		IdleTimeout:  timeout,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting web server", "port", port)
		if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(err, "Failed to start web server")
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info("Shutdown signal received, gracefully stopping web server")

	// Create a context with timeout for graceful shutdown
	ctxShutdown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Shutdown the web server
	if err := webServer.Shutdown(ctxShutdown); err != nil {
		log.Error(err, "Error during graceful shutdown")
		return err
	}

	log.Info("Web server stopped")
	return nil
}

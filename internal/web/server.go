// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/web"
)

// Cannot be read from config.
const (
	serverTimeout = time.Minute
)

// TODO: Could be read from config.
const (
	reportInterval         = 20 * time.Second
	namespaceCacheDuration = reportInterval
)

var (
	errGracefulShutdownDeadlineExceeded = errors.New("graceful shutdown deadline exceeded")
)

// RunServer starts the web server and blocks
// until the provided context is canceled.
// Whenever a new configuration is received on the confChannel,
// it updates the server settings accordingly without downtime.
// The error returned is either from http.Server.Shutdown() or
// graceful shutdown deadline exceeded.
func RunServer(ctx context.Context, c cluster.Cluster,
	confChannel <-chan *fluxcdv1.WebConfigSpec,
	version, statusManager, namespace string,
	gracefulShutdownTimeout time.Duration, port int) error {

	l := ctrl.Log.WithName("web-server").WithValues("port", port)

	// Get event recorder.
	eventRecorder := c.GetEventRecorderFor("flux-operator-web-ui")

	// Build SPA handler.
	spaHandler := http.FileServer(http.FS(NewFileSystem(web.GetFS())))

	// Initialize HTTP handler with one that serves the SPA but 503 on API requests.
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.SetAnonymousAuthProviderCookie(w) // Always set auth-provider cookie.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "server not initialized", http.StatusServiceUnavailable)
			return
		}
		spaHandler.ServeHTTP(w, r)
	})

	// Start server using the handler on a goroutine. Guard the handler
	// with a mutex so that it can be swapped on configuration updates.
	var handlerMu sync.RWMutex
	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
		IdleTimeout:  serverTimeout,

		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerMu.RLock()
			h := handler
			handlerMu.RUnlock()

			h.ServeHTTP(w, r)
		}),
	}
	serverStopped := make(chan struct{})
	go func() {
		defer close(serverStopped)
		l.Info("starting web server")
		if err := s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			l.Error(err, "unable to start web server")
			os.Exit(1)
		}
	}()

	// Create management variables for the current handler.
	var cancelHandlerCtx context.CancelFunc
	var handlerStopped <-chan struct{}
	var closeAuth func(context.Context) error

	// Initialize them for the initial handler which does not fire off any goroutines.
	_, cancelHandlerCtx = context.WithCancel(context.Background())
	ch := make(chan struct{})
	close(ch)
	handlerStopped = ch
	closeAuth = func(context.Context) error { return nil }

	// Configure graceful shutdown procedure.
	gracefulShutdown := func() error {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancelShutdown()

		// Shutdown the server.
		if err := s.Shutdown(shutdownCtx); err != nil {
			return err
		}
		select {
		case <-shutdownCtx.Done():
			return errGracefulShutdownDeadlineExceeded
		case <-serverStopped:
		}

		// Stop the auth provider background goroutines.
		if err := closeAuth(shutdownCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return errGracefulShutdownDeadlineExceeded
			}
			return err
		}

		// Wait for the current handler goroutines to stop.
		cancelHandlerCtx()
		select {
		case <-shutdownCtx.Done():
			return errGracefulShutdownDeadlineExceeded
		case <-handlerStopped:
			return nil
		}
	}

	// Listen for configuration updates.
	var conf *fluxcdv1.WebConfigSpec
	confVersion := "uninitialized"
	for {
		// If the context is done, initiate graceful shutdown.
		if ctx.Err() != nil {
			return gracefulShutdown()
		}

		// If there's no conf from the previous iteration, block until we get one.
		if conf == nil {
			select {
			case conf = <-confChannel:
			case <-ctx.Done(): // Context canceled while waiting for config, shutdown.
				return gracefulShutdown()
			}
		}

		// Log the configuration update.
		eventLog := l.WithValues("existingConfigVersion", confVersion, "newConfigVersion", conf.Version)
		eventLog.Info("web server configuration update received, will attempt to reconfigure the server")
		serverLog := l.WithValues("configVersion", conf.Version)

		// Create kubeclient.
		userCacheSize := 1 // Single cache entry if authentication is disabled or anonymous.
		if a := conf.Authentication; a != nil && a.Type != fluxcdv1.AuthenticationTypeAnonymous {
			userCacheSize = a.UserCacheSize
		}
		kubeClient, err := kubeclient.New(c.GetAPIReader(), c.GetClient(), c.GetConfig(), c.GetScheme(),
			userCacheSize, namespaceCacheDuration)
		if err != nil {
			eventLog.Error(err, "unable to create kubeclient with new configuration, keeping existing configuration")
			continue
		}

		// Create auth middleware.
		authMiddleware, newCloseAuth, err := auth.NewMiddleware(conf, kubeClient, serverLog)
		if err != nil {
			eventLog.Error(err, "unable to create auth middleware with new configuration, keeping existing configuration")
			continue
		}

		// Successfully created all components with the new configuration.
		confVersion = conf.Version

		// Create new handler.
		newHandlerCtx, cancelNewHandlerCtx := context.WithCancel(context.Background())
		newHandler, newHandlerStopped := NewHandler(newHandlerCtx, conf, spaHandler, kubeClient,
			version, statusManager, namespace, reportInterval, eventRecorder, authMiddleware, serverLog)

		conf = nil // Clear conf to receive a new one in the next iteration.

		// Route new requests to the new handler.
		handlerMu.Lock()
		handler = newHandler
		handlerMu.Unlock()
		eventLog.Info("web server reconfiguration successful, new configuration was applied")

		// Swap handler management variables.
		cancelOldHandlerCtx, oldHandlerStopped, closeOldAuth := cancelHandlerCtx, handlerStopped, closeAuth
		cancelHandlerCtx, handlerStopped, closeAuth = cancelNewHandlerCtx, newHandlerStopped, newCloseAuth

		// Stop the old auth provider and handler, then wait for any of the possible events.
		closeOldAuthCtx, cancelCloseOldAuth := context.WithTimeout(context.Background(), 10*time.Second)
		if err := closeOldAuth(closeOldAuthCtx); err != nil {
			eventLog.Error(err, "failed to close old auth provider")
		}
		cancelCloseOldAuth()
		cancelOldHandlerCtx()
		select {
		case <-oldHandlerStopped:
			// Old handler stopped successfully, continue to the next iteration.
		case conf = <-confChannel:
			// Another configuration update received while waiting, let the next iteration handle it.
		case <-ctx.Done():
			// Context canceled, let the next iteration handle graceful shutdown.
		}
	}
}

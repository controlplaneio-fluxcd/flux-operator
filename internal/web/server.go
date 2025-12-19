// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/web"
)

// TODO: Could be read from config.
const (
	serverTimeout           = time.Minute
	gracefulShutdownTimeout = 10 * time.Second

	reportInterval         = 20 * time.Second
	namespaceCacheDuration = reportInterval

	listenerQueueSize = 100
)

var (
	errGracefulShutdownDeadlineExceeded = errors.New("graceful shutdown deadline exceeded")
)

// serverComponents holds, in additional to the configuration,
// the components needed for running the web server that require
// initialization and can produce errors during their initialization.
type serverComponents struct {
	conf           *config.ConfigSpec
	kubeClient     *kubeclient.Client
	authMiddleware func(http.Handler) http.Handler
}

// InitializeServerComponents initializes the components required for running the web server.
func InitializeServerComponents(conf *config.ConfigSpec,
	c cluster.Cluster, initLog logr.Logger) (*serverComponents, error) {

	// Create kubeclient.
	userCacheSize := 1 // Single cache entry if authentication is disabled or anonymous.
	if a := conf.Authentication; a != nil && a.Type != config.AuthenticationTypeAnonymous {
		userCacheSize = a.UserCacheSize
	}
	kubeClient, err := kubeclient.New(c, userCacheSize, namespaceCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubeclient: %w", err)
	}

	// Create auth middleware.
	authMiddleware, err := auth.NewMiddleware(conf, kubeClient, initLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth middleware: %w", err)
	}

	return &serverComponents{
		conf:           conf,
		kubeClient:     kubeClient,
		authMiddleware: authMiddleware,
	}, nil
}

// queueListener is a net.Listener that accepts connections from a channel.
type queueListener struct {
	addr          net.Addr
	queue         <-chan net.Conn
	softCtx       context.Context
	cancelSoftCtx context.CancelFunc
	hardCtx       context.Context
	cancelHardCtx context.CancelFunc

	deadline   context.Context
	deadlineMu sync.RWMutex

	closed   bool
	closedMu sync.RWMutex
}

// newQueueListener creates a new queueListener. It returns the listener
// and a write-only channel for enqueuing connections.
func newQueueListener(addr net.Addr) (*queueListener, chan<- net.Conn) {
	queue := make(chan net.Conn, listenerQueueSize)
	hardCtx, cancelHardCtx := context.WithCancel(context.Background())
	softCtx, cancelSoftCtx := context.WithCancel(context.Background())
	return &queueListener{
		addr:          addr,
		queue:         queue,
		hardCtx:       hardCtx,
		cancelHardCtx: cancelHardCtx,
		softCtx:       softCtx,
		cancelSoftCtx: cancelSoftCtx,
	}, queue
}

// Accept implements net.Listener.
func (l *queueListener) Accept() (net.Conn, error) {
	// Check if it's already closed.
	l.closedMu.RLock()
	closed := l.closed
	l.closedMu.RUnlock()
	if closed {
		return nil, net.ErrClosed
	}

	// Not closed, call accept and close if it returns ErrClosed.
	conn, err := l.accept()
	if err != nil && errors.Is(err, net.ErrClosed) {
		l.closedMu.Lock()
		l.closed = true
		l.closedMu.Unlock()
	}
	return conn, err
}
func (l *queueListener) accept() (net.Conn, error) {
	switch {
	case l.hardCtx.Err() != nil:
		// Hard-closed. Check deadline and return ErrClosed if exceeded.
		l.deadlineMu.RLock()
		deadline := l.deadline
		l.deadlineMu.RUnlock()
		if deadline.Err() != nil {
			// Deadline exceeded. Close all connections.
			for {
				select {
				case conn := <-l.queue:
					_ = conn.Close()
				default:
					return nil, net.ErrClosed
				}
			}
		}

		// Deadline not exceeded yet. Return existing connection or ErrClosed if none are available.
		select {
		case conn := <-l.queue:
			return conn, nil
		default:
			return nil, net.ErrClosed
		}
	case l.softCtx.Err() != nil:
		// Soft-closed. Return an existing connection or ErrClosed if none are available.
		select {
		case conn := <-l.queue:
			return conn, nil
		default:
			return nil, net.ErrClosed
		}
	default:
		// Normal operation, block on all possible events.
		select {
		case conn := <-l.queue:
			return conn, nil
		case <-l.softCtx.Done():
			return nil, net.ErrClosed
		case <-l.hardCtx.Done():
			return nil, net.ErrClosed
		}
	}
}

// Addr implements net.Listener.
func (l *queueListener) Addr() net.Addr {
	return l.addr
}

// Close implements net.Listener by hard-closing the listener.
// This means setting a deadline (if not already set) for pending
// connections on the queue to be returned on Accept calls.
func (l *queueListener) Close() error {
	l.deadlineMu.Lock()
	if l.deadline == nil {
		var cancel context.CancelFunc
		l.deadline, cancel = context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		_ = cancel // Trick to make linter happy about lost cancel func. The process will exit soon anyway.
	}
	l.deadlineMu.Unlock()
	l.cancelHardCtx()
	return nil
}

// SoftClose soft-closes the listener, which means prioritizing
// returning existing connections from the queue on Accept calls
// instead of returning net.ErrClosed. If the queue is empty,
// Accept returns net.ErrClosed. Should be used when reconfiguring
// the server to allow connections that were already enqueued to
// be processed, ensuring no downtime.
func (l *queueListener) SoftClose() {
	l.cancelSoftCtx()
}

// RunServer starts the web server and blocks
// until the provided context is cancelled.
// Whenever a new configuration is received on the confChannel,
// it updates the server settings accordingly without downtime.
// It returns an error if the graceful shutdown deadline is
// exceeded.
func RunServer(ctx context.Context,
	c cluster.Cluster,
	confChannel <-chan *config.ConfigSpec,
	version, statusManager, namespace string,
	firstComponents *serverComponents,
	netListener net.Listener,
	l logr.Logger) error {

	lisAddr := netListener.Addr()
	confVersion := firstComponents.conf.Version

	// Start the web server with the initial configuration.
	serverQueue, serverListener, serverStopped := startServer(
		version, statusManager, namespace, firstComponents, lisAddr,
		l.WithValues("configVersion", confVersion))

	// Start goroutine to accept connections and enqueue them.
	var queueMu sync.Mutex
	netListenerStopped := make(chan struct{})
	go func() {
		defer close(netListenerStopped)

		for {
			// Accept incoming connection.
			conn, err := netListener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				l.Error(err, "failed to accept incoming connection")
				os.Exit(1)
			}

			// Enqueue the connection to the current server's queue.
			queueMu.Lock()
			q := serverQueue
			queueMu.Unlock()
			select {
			case q <- conn:
			case <-ctx.Done():
				_ = conn.Close()
				return
			}
		}
	}()

	// Listen for configuration updates.
	for {
		select {
		case <-ctx.Done():
			// Process is shutting down, close the listeners and wait for goroutines to stop.
			_ = netListener.Close()
			_ = serverListener.Close()
			gracefulShutdownDeadline := time.After(gracefulShutdownTimeout)
			select {
			case <-serverStopped:
				select {
				case <-netListenerStopped:
					return nil
				case <-gracefulShutdownDeadline:
					return errGracefulShutdownDeadlineExceeded
				}
			case <-gracefulShutdownDeadline:
				return errGracefulShutdownDeadlineExceeded
			}
		case conf := <-confChannel:
			// Log the configuration update.
			eventLog := l.WithValues("existingConfigVersion", confVersion, "newConfigVersion", conf.Version)
			eventLog.Info("web server configuration update received, reconfiguring server")

			// Attempt initializing the new server components.
			serverLog := l.WithValues("configVersion", conf.Version)
			components, err := InitializeServerComponents(conf, c, serverLog)
			if err != nil {
				eventLog.Error(err, "unable to initialize web server components "+
					"with new configuration, keeping existing configuration")
				continue
			}

			// Start a new server with the updated configuration.
			newServerQueue, newServerListener, newServerStopped := startServer(
				version, statusManager, namespace, components, lisAddr, serverLog)

			// Route new connections to the new server.
			queueMu.Lock()
			serverQueue = newServerQueue
			queueMu.Unlock()

			// Stop the existing server.
			serverListener.SoftClose()
			select {
			case <-serverStopped:
			case <-ctx.Done():
				// Process is shutting down, close the listeners and wait for goroutines to stop.
				_ = netListener.Close()
				_ = newServerListener.Close()
				_ = serverListener.Close()
				gracefulShutdownDeadline := time.After(gracefulShutdownTimeout)
				select {
				case <-serverStopped:
					select {
					case <-newServerStopped:
						select {
						case <-netListenerStopped:
							return nil
						case <-gracefulShutdownDeadline:
							return errGracefulShutdownDeadlineExceeded
						}
					case <-gracefulShutdownDeadline:
						return errGracefulShutdownDeadlineExceeded
					}
				case <-gracefulShutdownDeadline:
					return errGracefulShutdownDeadlineExceeded
				}
			}

			// Switch pointers to the new server.
			serverListener = newServerListener
			serverStopped = newServerStopped
		}
	}
}

// startServer starts the web server on a goroutine.
// It should never return errors as it assumes that
// all the serverComponents have been properly initialized.
func startServer(version, statusManager, namespace string,
	components *serverComponents, lisAddr net.Addr,
	l logr.Logger) (chan<- net.Conn, *queueListener, <-chan struct{}) {

	// Get initialized components.
	kubeClient := components.kubeClient
	authMiddleware := components.authMiddleware

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Create router with embedded filesystem and register routes
	router := NewRouter(mux, web.GetFS(), kubeClient, version, statusManager, namespace, reportInterval, authMiddleware)
	router.RegisterRoutes()

	// Start background report cache refresh.
	ctx, cancel := context.WithCancel(context.Background())
	stopped := router.StartReportCache(log.IntoContext(ctx, l.WithValues("background", true)))

	// Create HTTP server with timeouts.
	webServer := &http.Server{
		Handler:      router.RegisterMiddleware(l),
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
		IdleTimeout:  serverTimeout,
	}

	// Start server in a goroutine.
	listener, queue := newQueueListener(lisAddr)
	go func() {
		l.Info("web server started")
		if err := webServer.Serve(listener); err != nil &&
			!errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			l.Error(err, "failed to start web server")
			os.Exit(1)
		}
		l.Info("web server stopped")
		cancel()
	}()

	return queue, listener, stopped
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// gzipResponseWriter wraps http.ResponseWriter to compress responses with gzip.
type gzipResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if gzipWriter, ok := w.writer.(*gzip.Writer); ok {
		_ = gzipWriter.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// GzipMiddleware adds gzip compression to responses.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip compression
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Create gzip writer
		gzipWriter := gzip.NewWriter(w)
		defer gzipWriter.Close()

		// Set response headers
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")

		// Serve with gzip compression
		next.ServeHTTP(&gzipResponseWriter{
			ResponseWriter: w,
			writer:         gzipWriter,
		}, r)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// CacheControlMiddleware sets appropriate cache headers for static assets.
// It ensures index.html is never cached while hashed assets are cached forever.
func CacheControlMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// For hashed assets in /assets/ directory, cache forever
		// These files have content-based hashes in their names, so they're immutable
		if strings.HasPrefix(path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// For index.html and other files, always revalidate
			// This ensures users get the latest asset references
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests and responses.
func LoggingMiddleware(logger logr.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Log request details
		duration := time.Since(start)
		logger.Info("HTTP request completed",
			"uri", r.RequestURI,
			"method", r.Method,
			"status", wrapped.statusCode,
			"remote", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"latency_ms", duration.Round(time.Millisecond).Milliseconds(),
		)
	})
}

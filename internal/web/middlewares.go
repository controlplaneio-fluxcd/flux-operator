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
	"sigs.k8s.io/controller-runtime/pkg/log"
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

// SecurityHeadersMiddleware adds security headers to all responses.
// These headers help protect against common web vulnerabilities.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking by disallowing framing
		w.Header().Set("X-Frame-Options", "DENY")
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Enable XSS filter (legacy browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// Restrict referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Permissions policy (disable unnecessary browser features)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// Instruct search engines not to index or follow links
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests and responses.
func LoggingMiddleware(logger logr.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		reqFields := map[string]any{
			"method":     r.Method,
			"path":       r.URL.RequestURI(),
			"remote":     r.RemoteAddr,
			"user_agent": r.UserAgent(),
		}
		ctx := log.IntoContext(r.Context(), logger.WithValues("httpRequest", reqFields))
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Log request details
		duration := time.Since(start)
		logger.V(1).Info("HTTP request completed", "httpRequest", reqFields, "httpResponse", map[string]any{
			"status":     wrapped.statusCode,
			"latency_ms": duration.Round(time.Millisecond).Milliseconds(),
		})
	})
}

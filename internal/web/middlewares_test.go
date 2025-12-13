// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
)

func TestGzipMiddleware(t *testing.T) {
	// Test handler that returns a simple response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response content"))
	})

	for _, tt := range []struct {
		name             string
		acceptEncoding   string
		expectCompressed bool
		expectHeader     bool
		expectContent    string
	}{
		{
			name:             "compresses when gzip accepted",
			acceptEncoding:   "gzip",
			expectCompressed: true,
			expectHeader:     true,
			expectContent:    "test response content",
		},
		{
			name:             "compresses when gzip with other encodings",
			acceptEncoding:   "gzip, deflate, br",
			expectCompressed: true,
			expectHeader:     true,
			expectContent:    "test response content",
		},
		{
			name:             "does not compress when gzip not accepted",
			acceptEncoding:   "deflate, br",
			expectCompressed: false,
			expectHeader:     false,
			expectContent:    "test response content",
		},
		{
			name:             "does not compress when no accept-encoding",
			acceptEncoding:   "",
			expectCompressed: false,
			expectHeader:     false,
			expectContent:    "test response content",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Apply middleware
			middleware := GzipMiddleware(testHandler)
			middleware.ServeHTTP(rec, req)

			// Check status code
			g.Expect(rec.Code).To(Equal(http.StatusOK))

			// Check Content-Encoding header
			if tt.expectHeader {
				g.Expect(rec.Header().Get("Content-Encoding")).To(Equal("gzip"))
			} else {
				g.Expect(rec.Header().Get("Content-Encoding")).To(BeEmpty())
			}

			// Check response body
			if tt.expectCompressed {
				// Decompress and verify
				reader, err := gzip.NewReader(rec.Body)
				g.Expect(err).NotTo(HaveOccurred())
				defer reader.Close()

				decompressed, err := io.ReadAll(reader)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(decompressed)).To(Equal(tt.expectContent))
			} else {
				// Verify uncompressed content
				g.Expect(rec.Body.String()).To(Equal(tt.expectContent))
			}
		})
	}
}

func TestGzipMiddleware_Flush(t *testing.T) {
	g := NewWithT(t)

	// Test handler that uses Flush
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("chunk1"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("chunk2"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()

	middleware := GzipMiddleware(testHandler)
	middleware.ServeHTTP(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Encoding")).To(Equal("gzip"))

	// Decompress and verify both chunks are present
	reader, err := gzip.NewReader(rec.Body)
	g.Expect(err).NotTo(HaveOccurred())
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(decompressed)).To(Equal("chunk1chunk2"))
}

func TestCacheControlMiddleware(t *testing.T) {
	// Test handler that returns a simple response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	})

	for _, tt := range []struct {
		name          string
		path          string
		expectedCache string
	}{
		{
			name:          "immutable cache for assets",
			path:          "/assets/main.js",
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "immutable cache for assets with hash",
			path:          "/assets/main-abc123.js",
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "immutable cache for CSS in assets",
			path:          "/assets/styles.css",
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "immutable cache for nested assets",
			path:          "/assets/js/vendor/lib.js",
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "no-cache for index.html",
			path:          "/index.html",
			expectedCache: "no-cache, no-store, must-revalidate",
		},
		{
			name:          "no-cache for root",
			path:          "/",
			expectedCache: "no-cache, no-store, must-revalidate",
		},
		{
			name:          "no-cache for API endpoints",
			path:          "/api/v1/report",
			expectedCache: "no-cache, no-store, must-revalidate",
		},
		{
			name:          "no-cache for other paths",
			path:          "/favicon.ico",
			expectedCache: "no-cache, no-store, must-revalidate",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create request
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Apply middleware
			middleware := CacheControlMiddleware(testHandler)
			middleware.ServeHTTP(rec, req)

			// Check Cache-Control header
			g.Expect(rec.Header().Get("Cache-Control")).To(Equal(tt.expectedCache))

			// Check status code
			g.Expect(rec.Code).To(Equal(http.StatusOK))

			// Check body
			g.Expect(rec.Body.String()).To(Equal("content"))
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// Test handler that returns different status codes
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for status code in query param for testing
		statusStr := r.URL.Query().Get("status")
		switch statusStr {
		case "404":
			w.WriteHeader(http.StatusNotFound)
		case "500":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write([]byte("response"))
	})

	for _, tt := range []struct {
		name           string
		method         string
		path           string
		queryStatus    string
		remoteAddr     string
		userAgent      string
		expectedStatus int
	}{
		{
			name:           "logs successful GET request",
			method:         http.MethodGet,
			path:           "/api/v1/report",
			queryStatus:    "",
			remoteAddr:     "192.168.1.1:12345",
			userAgent:      "Mozilla/5.0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "logs POST request",
			method:         http.MethodPost,
			path:           "/api/v1/resource",
			queryStatus:    "",
			remoteAddr:     "192.168.1.2:54321",
			userAgent:      "curl/7.68.0",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "logs 404 response",
			method:         http.MethodGet,
			path:           "/api/v1/notfound?status=404",
			queryStatus:    "404",
			remoteAddr:     "192.168.1.3:11111",
			userAgent:      "wget",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "logs 500 response",
			method:         http.MethodGet,
			path:           "/api/v1/error?status=500",
			queryStatus:    "500",
			remoteAddr:     "192.168.1.4:22222",
			userAgent:      "Python/3.9",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "logs request without user agent",
			method:         http.MethodGet,
			path:           "/api/v1/test",
			queryStatus:    "",
			remoteAddr:     "10.0.0.1:33333",
			userAgent:      "",
			expectedStatus: http.StatusOK,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Capture log output
			var logBuffer bytes.Buffer
			logger := logr.New(newTestLogSink(&logBuffer))

			// Create request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Apply middleware
			middleware := LoggingMiddleware(logger, testHandler)
			middleware.ServeHTTP(rec, req)

			// Check status code
			g.Expect(rec.Code).To(Equal(tt.expectedStatus))

			// Check that log was written
			logOutput := logBuffer.String()
			g.Expect(logOutput).To(ContainSubstring("HTTP request completed"))
			g.Expect(logOutput).To(ContainSubstring(tt.path))
			g.Expect(logOutput).To(ContainSubstring(tt.method))
			g.Expect(logOutput).To(ContainSubstring(tt.remoteAddr))

			if tt.userAgent != "" {
				g.Expect(logOutput).To(ContainSubstring(tt.userAgent))
			}
		})
	}
}

func TestLoggingMiddleware_StatusCodeDefault(t *testing.T) {
	g := NewWithT(t)

	// Handler that doesn't explicitly set status code (defaults to 200)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	var logBuffer bytes.Buffer
	logger := logr.New(newTestLogSink(&logBuffer))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware := LoggingMiddleware(logger, testHandler)
	middleware.ServeHTTP(rec, req)

	// Should default to 200 OK
	g.Expect(rec.Code).To(Equal(http.StatusOK))
	logOutput := logBuffer.String()
	g.Expect(logOutput).To(ContainSubstring("200"))
}

// testLogSink is a simple logr.LogSink implementation for testing
type testLogSink struct {
	writer io.Writer
}

func newTestLogSink(w io.Writer) *testLogSink {
	return &testLogSink{writer: w}
}

func (t *testLogSink) Init(info logr.RuntimeInfo) {}

func (t *testLogSink) Enabled(level int) bool {
	return true
}

func (t *testLogSink) Info(level int, msg string, keysAndValues ...any) {
	var sb strings.Builder
	sb.WriteString(msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			sb.WriteString(" ")
			sb.WriteString(keysAndValues[i].(string))
			sb.WriteString("=")
			sb.WriteString(formatValue(keysAndValues[i+1]))
		}
	}
	sb.WriteString("\n")
	_, _ = t.writer.Write([]byte(sb.String()))
}

func (t *testLogSink) Error(err error, msg string, keysAndValues ...any) {
	var sb strings.Builder
	sb.WriteString("ERROR: ")
	sb.WriteString(msg)
	sb.WriteString(" error=")
	sb.WriteString(err.Error())
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			sb.WriteString(" ")
			sb.WriteString(keysAndValues[i].(string))
			sb.WriteString("=")
			sb.WriteString(formatValue(keysAndValues[i+1]))
		}
	}
	sb.WriteString("\n")
	_, _ = t.writer.Write([]byte(sb.String()))
}

func (t *testLogSink) WithValues(keysAndValues ...any) logr.LogSink {
	return t
}

func (t *testLogSink) WithName(name string) logr.LogSink {
	return t
}

func formatValue(v any) string {
	return fmt.Sprintf("%v", v)
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	// Test handler that returns a simple response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	})

	for _, tt := range []struct {
		name          string
		path          string
		expectedValue map[string]string
	}{
		{
			name: "sets all security headers for root",
			path: "/",
			expectedValue: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
			},
		},
		{
			name: "sets all security headers for API",
			path: "/api/v1/report",
			expectedValue: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
			},
		},
		{
			name: "sets all security headers for static assets",
			path: "/assets/app-abc123.js",
			expectedValue: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create request
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Apply middleware
			middleware := SecurityHeadersMiddleware(testHandler)
			middleware.ServeHTTP(rec, req)

			// Check all security headers
			for header, expected := range tt.expectedValue {
				g.Expect(rec.Header().Get(header)).To(Equal(expected),
					"Header %s should be %s", header, expected)
			}

			// Check status code
			g.Expect(rec.Code).To(Equal(http.StatusOK))

			// Check body
			g.Expect(rec.Body.String()).To(Equal("content"))
		})
	}
}

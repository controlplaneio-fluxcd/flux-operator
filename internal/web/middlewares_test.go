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

func TestCrossOriginMiddleware(t *testing.T) {
	// Test handler that returns a simple response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	})

	const jsonBody = `{"action":"suspend"}`

	for _, tt := range []struct {
		name         string
		method       string
		secFetchSite string
		contentType  string
		body         string
		expectedCode int
	}{
		{
			name:         "allows same-origin JSON post",
			method:       http.MethodPost,
			secFetchSite: "same-origin",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows JSON post with charset parameter",
			method:       http.MethodPost,
			secFetchSite: "same-origin",
			contentType:  "application/json; charset=utf-8",
			body:         jsonBody,
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows user initiated request",
			method:       http.MethodPost,
			secFetchSite: "none",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows client that sends no fetch metadata",
			method:       http.MethodPost,
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows bodyless same-origin post",
			method:       http.MethodPost,
			secFetchSite: "same-origin",
			expectedCode: http.StatusOK,
		},
		{
			name:         "allows bodyless post without fetch metadata",
			method:       http.MethodPost,
			expectedCode: http.StatusOK,
		},
		{
			name:         "denies cross-site post",
			method:       http.MethodPost,
			secFetchSite: "cross-site",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "denies same-site post",
			method:       http.MethodPost,
			secFetchSite: "same-site",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "denies bodyless cross-site post",
			method:       http.MethodPost,
			secFetchSite: "cross-site",
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "denies cross-site delete",
			method:       http.MethodDelete,
			secFetchSite: "cross-site",
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "allows delete carrying a JSON body",
			method:       http.MethodDelete,
			secFetchSite: "same-origin",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusOK,
		},
		{
			name:         "denies delete carrying a plain text body",
			method:       http.MethodDelete,
			contentType:  "text/plain",
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "allows unrecognized fetch site with a JSON body",
			method:       http.MethodPost,
			secFetchSite: "unrecognized",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusOK,
		},
		{
			name:         "denies unrecognized fetch site with a plain text body",
			method:       http.MethodPost,
			secFetchSite: "unrecognized",
			contentType:  "text/plain",
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "denies cross-site put",
			method:       http.MethodPut,
			secFetchSite: "cross-site",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "denies cross-site patch",
			method:       http.MethodPatch,
			secFetchSite: "cross-site",
			contentType:  "application/json",
			body:         jsonBody,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "denies plain text body",
			method:       http.MethodPost,
			contentType:  "text/plain",
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "denies form encoded body",
			method:       http.MethodPost,
			contentType:  "application/x-www-form-urlencoded",
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "denies multipart body",
			method:       http.MethodPost,
			contentType:  "multipart/form-data; boundary=x",
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "denies body without a media type",
			method:       http.MethodPost,
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "denies body with a malformed media type",
			method:       http.MethodPost,
			contentType:  "application/json;;",
			body:         jsonBody,
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "does not screen cross-site get",
			method:       http.MethodGet,
			secFetchSite: "cross-site",
			expectedCode: http.StatusOK,
		},
		{
			name:         "does not screen cross-site head",
			method:       http.MethodHead,
			secFetchSite: "cross-site",
			expectedCode: http.StatusOK,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create request, leaving the body nil when the case has none
			// so that ContentLength stays zero
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, "/api/v1/resource/action", body)
			if tt.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSite)
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Apply middleware
			middleware := CrossOriginMiddleware(testHandler)
			middleware.ServeHTTP(rec, req)

			// Check status code
			g.Expect(rec.Code).To(Equal(tt.expectedCode))

			// Check the request only reached the handler when allowed
			if tt.expectedCode == http.StatusOK {
				g.Expect(rec.Body.String()).To(Equal("content"))
			} else {
				g.Expect(rec.Body.String()).NotTo(ContainSubstring("content"))
			}
		})
	}
}

func TestCrossOriginMiddleware_UnknownBodyLength(t *testing.T) {
	// Test handler that returns a simple response
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	})

	for _, tt := range []struct {
		name         string
		contentType  string
		expectedCode int
	}{
		{
			name:         "allows JSON body of unknown length",
			contentType:  "application/json",
			expectedCode: http.StatusOK,
		},
		{
			name:         "denies plain text body of unknown length",
			contentType:  "text/plain",
			expectedCode: http.StatusUnsupportedMediaType,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// A reader of unknown size yields a negative ContentLength, which
			// must still be treated as a body that declares its media type.
			req := httptest.NewRequest(http.MethodPost, "/api/v1/resource/action",
				io.NopCloser(strings.NewReader(`{"action":"suspend"}`)))
			req.Header.Set("Content-Type", tt.contentType)
			g.Expect(req.ContentLength).To(Equal(int64(-1)))

			// Create response recorder
			rec := httptest.NewRecorder()

			// Apply middleware
			CrossOriginMiddleware(testHandler).ServeHTTP(rec, req)

			// Check status code
			g.Expect(rec.Code).To(Equal(tt.expectedCode))
		})
	}
}

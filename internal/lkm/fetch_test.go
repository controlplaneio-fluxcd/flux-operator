// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
)

func TestFetch(t *testing.T) {
	t.Run("uses default options", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(r.UserAgent()).To(Equal("flux-operator-lkm/1.0"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		ctx := context.Background()
		data, err := Fetch(ctx, server.URL)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(Equal("test response"))
	})

	t.Run("applies multiple options", func(t *testing.T) {
		g := NewWithT(t)
		expectedContentType := "application/json"
		customUserAgent := "multi-option-agent/3.0"
		expectedData := []byte(`{"test": "value"}`)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(r.Header.Get("Content-Type")).To(Equal(expectedContentType))
			g.Expect(r.UserAgent()).To(Equal(customUserAgent))
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", expectedContentType)
			_, _ = w.Write(expectedData)
		}))
		defer server.Close()

		ctx := context.Background()
		data, err := Fetch(ctx, server.URL,
			FetchOpt.WithRetries(1),
			FetchOpt.WithUserAgent(customUserAgent),
			FetchOpt.WithContentType(expectedContentType),
			FetchOpt.WithInsecureSkipVerify(true),
		)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(expectedData))
	})

	t.Run("fails with HTTP URL", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		_, err := Fetch(ctx, "http://example.com/data")

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("URL must use HTTPS scheme"))
	})

	t.Run("fails with invalid URL", func(t *testing.T) {
		g := NewWithT(t)

		ctx := context.Background()
		_, err := Fetch(ctx, "://invalid-url")

		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails with status code", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := Fetch(ctx, server.URL, FetchOpt.WithRetries(1), FetchOpt.WithInsecureSkipVerify(true))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("fetch failed with status: 403"))
	})

	t.Run("fails with empty body", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := Fetch(ctx, server.URL, FetchOpt.WithRetries(1), FetchOpt.WithInsecureSkipVerify(true))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("response body is empty"))
	})

	t.Run("fails with invalid content type", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := Fetch(ctx, server.URL,
			FetchOpt.WithContentType("application/json"),
			FetchOpt.WithInsecureSkipVerify(true))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid JSON"))
	})

	t.Run("fails on localhost", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := Fetch(ctx, server.URL, FetchOpt.WithLocalhost(false))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("URL must use HTTPS scheme"))
	})

	t.Run("fails after retrying", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := Fetch(ctx, server.URL, FetchOpt.WithRetries(1), FetchOpt.WithInsecureSkipVerify(true))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("giving up after 2 attempt(s)"))
	})
}

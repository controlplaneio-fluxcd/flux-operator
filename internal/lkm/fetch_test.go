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

		data, err := Fetch(context.TODO(), server.URL)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(Equal("test response"))
	})

	t.Run("applies multiple options", func(t *testing.T) {
		g := NewWithT(t)
		customUserAgent := "multi-option-agent/3.0"
		expectedData := []byte(`{"keys":[{"use":"sig","kty": "OKP","crv": "Ed25519","alg": "EdDSA"}]}`)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(r.Header.Get("Accept")).To(Equal("application/json, application/jwks"))
			g.Expect(r.UserAgent()).To(Equal(customUserAgent))
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/jwk-set+json")
			_, _ = w.Write(expectedData)
		}))
		defer server.Close()

		data, err := Fetch(
			context.TODO(),
			server.URL,
			FetchOpt.WithRetries(1),
			FetchOpt.WithUserAgent(customUserAgent),
			FetchOpt.WithContentType(ContentTypeKeySet),
			FetchOpt.WithInsecureSkipVerify(true),
		)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(expectedData))
	})

	t.Run("validates jwt", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(r.Header.Get("Accept")).To(Equal("application/jose, application/jwt"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("HEADER.PAYLOAD.SIGNATURE"))
		}))
		defer server.Close()

		body, err := Fetch(context.TODO(), server.URL, FetchOpt.WithContentType(ContentTypeToken))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(body)).To(Equal("HEADER.PAYLOAD.SIGNATURE"))
	})

	t.Run("validates Google jwks", func(t *testing.T) {
		g := NewWithT(t)

		data, err := Fetch(context.TODO(),
			"https://www.googleapis.com/oauth2/v3/certs",
			FetchOpt.WithContentType(ContentTypeKeySet))

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(ContainSubstring(`"use": "sig"`))
	})

	t.Run("fails with HTTP URL", func(t *testing.T) {
		g := NewWithT(t)

		_, err := Fetch(context.TODO(), "http://example.com/data")

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("HTTPS scheme is required"))
	})

	t.Run("fails with invalid URL", func(t *testing.T) {
		g := NewWithT(t)

		_, err := Fetch(context.TODO(), "://invalid-url")

		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails with status code", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		_, err := Fetch(context.TODO(), server.URL)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed with status: 403"))
	})

	t.Run("fails with empty body", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		_, err := Fetch(context.TODO(), server.URL)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("response body is empty"))
	})

	t.Run("fails with invalid content response", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(r.Header.Get("Accept")).To(Equal("application/jose, application/jwe"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("HEADER.PAYLOAD.SIGNATURE"))
		}))
		defer server.Close()

		_, err := Fetch(context.TODO(), server.URL, FetchOpt.WithContentType(ContentTypeEncryptedToken))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid"))
	})

	t.Run("fails when localhost is not allowed", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		_, err := Fetch(context.TODO(), server.URL, FetchOpt.WithLocalhost(false))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("HTTPS scheme is required"))
	})

	t.Run("fails with unknown authority", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))
		defer server.Close()

		_, err := Fetch(context.TODO(), server.URL)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to verify certificate"))
	})

	t.Run("fails after retrying", func(t *testing.T) {
		g := NewWithT(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := Fetch(context.TODO(), server.URL, FetchOpt.WithRetries(1))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("giving up after 2 attempt(s)"))
	})
}

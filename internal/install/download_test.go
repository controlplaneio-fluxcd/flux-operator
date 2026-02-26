// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
)

func TestDownloadManifestFromURL_PlainHTTP(t *testing.T) {
	g := NewWithT(t)

	expected := "apiVersion: v1\nkind: ConfigMap\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(expected))
	}))
	defer srv.Close()

	data, err := DownloadManifestFromURL(context.Background(), srv.URL+"/manifest.yaml", authn.DefaultKeychain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(data)).To(Equal(expected))
}

func TestDownloadManifestFromURL_NotFound(t *testing.T) {
	g := NewWithT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := DownloadManifestFromURL(context.Background(), srv.URL+"/manifest.yaml", authn.DefaultKeychain)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("404"))
}

func TestDownloadManifestFromURL_InvalidURL(t *testing.T) {
	g := NewWithT(t)

	_, err := DownloadManifestFromURL(context.Background(), "://invalid", authn.DefaultKeychain)
	g.Expect(err).To(HaveOccurred())
}

func TestDownloadManifestFromURL_GitHubRawParam(t *testing.T) {
	g := NewWithT(t)

	expected := "github-content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Query().Get("raw")).To(Equal("true"))
		_, _ = w.Write([]byte(expected))
	}))
	defer srv.Close()

	// The function checks if the host contains "github" to add ?raw=true.
	// We can't easily fake the host with httptest, so instead test that
	// a URL that already has raw=true passes through without error.
	url := srv.URL + "/org/repo/blob/main/file.yaml?raw=true"
	data, err := DownloadManifestFromURL(context.Background(), url, authn.DefaultKeychain)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(data)).To(Equal(expected))
}

func TestDownloadManifestFromURL_OCI_MissingFragment(t *testing.T) {
	g := NewWithT(t)

	_, err := DownloadManifestFromURL(context.Background(), "oci://ghcr.io/org/repo:latest", authn.DefaultKeychain)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("fragment with the file path"))
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestMakeOptions_Defaults(t *testing.T) {
	g := NewWithT(t)

	opts, err := MakeOptions(
		WithArtifactURL(DefaultArtifactURL),
		WithOwner(DefaultOwner),
		WithNamespace(DefaultNamespace),
		WithTerminationTimeout(DefaultTerminationTimeout),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.ArtifactURL()).To(Equal(DefaultArtifactURL))
	g.Expect(opts.Owner()).To(Equal(DefaultOwner))
	g.Expect(opts.Namespace()).To(Equal(DefaultNamespace))
	g.Expect(opts.TerminationTimeout()).To(Equal(DefaultTerminationTimeout))
	g.Expect(opts.Credentials()).To(BeEmpty())
}

func TestMakeOptions_CustomValues(t *testing.T) {
	g := NewWithT(t)

	url := "oci://ghcr.io/custom/manifests:v1.0.0"
	creds := "user:token123"
	owner := "my-cli"
	ns := "custom-ns"
	timeout := 60 * time.Second

	opts, err := MakeOptions(
		WithArtifactURL(url),
		WithCredentials(creds),
		WithOwner(owner),
		WithNamespace(ns),
		WithTerminationTimeout(timeout),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.ArtifactURL()).To(Equal(url))
	g.Expect(opts.Credentials()).To(Equal(creds))
	g.Expect(opts.Owner()).To(Equal(owner))
	g.Expect(opts.Namespace()).To(Equal(ns))
	g.Expect(opts.TerminationTimeout()).To(Equal(timeout))
}

func TestMakeOptions_OverrideOrder(t *testing.T) {
	g := NewWithT(t)

	opts, err := MakeOptions(
		WithArtifactURL("oci://first:v1"),
		WithArtifactURL("oci://second:v2"),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(opts.ArtifactURL()).To(Equal("oci://second:v2"))
}

func TestMakeOptions_ValidateArtifactURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid OCI URL",
			url:  "oci://ghcr.io/org/repo:latest",
		},
		{
			name: "valid OCI URL with digest",
			url:  "oci://ghcr.io/org/repo:v1.0.0@sha256:abc",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
			errMsg:  "must start with 'oci://'",
		},
		{
			name:    "HTTP URL",
			url:     "https://example.com/manifests",
			wantErr: true,
			errMsg:  "must start with 'oci://'",
		},
		{
			name:    "no scheme",
			url:     "ghcr.io/org/repo:latest",
			wantErr: true,
			errMsg:  "must start with 'oci://'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			_, err := MakeOptions(WithArtifactURL(tt.url))
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestMakeOptions_ValidateCredentials(t *testing.T) {
	tests := []struct {
		name    string
		creds   string
		wantErr bool
		errMsg  string
	}{
		{
			name:  "valid credentials",
			creds: "user:token",
		},
		{
			name:  "credentials with colon in token",
			creds: "user:tok:en:value",
		},
		{
			name:  "empty credentials",
			creds: "",
		},
		{
			name:    "no colon separator",
			creds:   "usertoken",
			wantErr: true,
			errMsg:  "must be in the format 'username:token'",
		},
		{
			name:    "empty username",
			creds:   ":token",
			wantErr: true,
			errMsg:  "must be in the format 'username:token'",
		},
		{
			name:    "empty token",
			creds:   "user:",
			wantErr: true,
			errMsg:  "must be in the format 'username:token'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			_, err := MakeOptions(
				WithArtifactURL(DefaultArtifactURL),
				WithCredentials(tt.creds),
			)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

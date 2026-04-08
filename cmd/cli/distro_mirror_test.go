// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

func TestDistroMirror_FlagValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError string
	}{
		{
			name:        "missing destination arg",
			args:        []string{"distro", "mirror"},
			expectError: "accepts 1 arg",
		},
		{
			name:        "unknown variant",
			args:        []string{"distro", "mirror", "localhost:5050", "--variant", "bogus"},
			expectError: "unsupported variant",
		},
		{
			name:        "pull-token and pull-token-stdin are mutually exclusive",
			args:        []string{"distro", "mirror", "localhost:5050", "--pull-token", "abc", "--pull-token-stdin"},
			expectError: "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := executeCommand(tt.args)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
		})
	}
}

func TestDistroMirror_VariantRegistryMap(t *testing.T) {
	g := NewWithT(t)

	// Every advertised variant must have a source registry mapping.
	for _, v := range []string{
		builder.UpstreamAlpine,
		builder.EnterpriseAlpine,
		builder.EnterpriseDistroless,
		builder.EnterpriseDistrolessFIPS,
	} {
		_, ok := distroMirrorVariantRegistry[v]
		g.Expect(ok).To(BeTrue(), "variant %q has no source registry mapping", v)
	}

	g.Expect(distroMirrorVariantRegistry[builder.UpstreamAlpine]).To(Equal("ghcr.io/fluxcd"))
	g.Expect(distroMirrorVariantRegistry[builder.EnterpriseAlpine]).To(Equal("ghcr.io/controlplaneio-fluxcd/alpine"))
	g.Expect(distroMirrorVariantRegistry[builder.EnterpriseDistroless]).To(Equal("ghcr.io/controlplaneio-fluxcd/distroless"))
	g.Expect(distroMirrorVariantRegistry[builder.EnterpriseDistrolessFIPS]).To(Equal("ghcr.io/controlplaneio-fluxcd/distroless-fips"))
}

func TestDistroMirror_ResolveComponents(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		version     string
		expect      []string
		expectError string
	}{
		{
			name:    "default for 2.6.x has no source-watcher",
			version: "v2.6.4",
			expect: []string{
				"source-controller",
				"kustomize-controller",
				"helm-controller",
				"notification-controller",
				"image-reflector-controller",
				"image-automation-controller",
			},
		},
		{
			name:    "default for 2.7.x includes source-watcher",
			version: "v2.7.0",
			expect: []string{
				"source-controller",
				"kustomize-controller",
				"helm-controller",
				"notification-controller",
				"image-reflector-controller",
				"image-automation-controller",
				"source-watcher",
			},
		},
		{
			name:    "explicit subset is preserved",
			input:   []string{"source-controller", "helm-controller"},
			version: "v2.8.5",
			expect:  []string{"source-controller", "helm-controller"},
		},
		{
			name:        "invalid component is rejected",
			input:       []string{"bogus-controller"},
			version:     "v2.8.5",
			expectError: "invalid component",
		},
		{
			name:        "source-watcher rejected on 2.6.x",
			input:       []string{"source-watcher"},
			version:     "v2.6.4",
			expectError: "source-watcher is only supported",
		},
		{
			name:        "invalid version is rejected",
			input:       nil,
			version:     "not-a-version",
			expectError: "failed to parse version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			out, err := resolveMirrorComponents(tt.input, tt.version)
			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(out).To(Equal(tt.expect))
		})
	}
}

func TestDistroMirror_BuildJobs(t *testing.T) {
	g := NewWithT(t)

	images := []builder.ComponentImage{
		{
			Name:       "source-controller",
			Repository: "ghcr.io/fluxcd/source-controller",
			Tag:        "v1.6.2",
			Digest:     "sha256:aaaa",
		},
		{
			Name:       "kustomize-controller",
			Repository: "ghcr.io/fluxcd/kustomize-controller",
			Tag:        "v1.6.1",
			Digest:     "sha256:bbbb",
		},
	}

	// Save and override VERSION so the operator/chart job tags are deterministic.
	origVersion := VERSION
	VERSION = "0.46.0"
	defer func() { VERSION = origVersion }()

	jobs := buildMirrorJobs(images, "registry.example.com/flux", true, true)
	g.Expect(jobs).To(HaveLen(4)) // 2 controllers + operator + chart

	// Controllers
	g.Expect(jobs[0].srcRepo).To(Equal("ghcr.io/fluxcd/source-controller"))
	g.Expect(jobs[0].dstRepo).To(Equal("registry.example.com/flux/source-controller"))
	g.Expect(jobs[0].tag).To(Equal("v1.6.2"))
	g.Expect(jobs[0].digest).To(Equal("sha256:aaaa"))
	g.Expect(jobs[0].src()).To(Equal("ghcr.io/fluxcd/source-controller:v1.6.2"))
	g.Expect(jobs[0].dst()).To(Equal("registry.example.com/flux/source-controller:v1.6.2"))

	g.Expect(jobs[1].src()).To(Equal("ghcr.io/fluxcd/kustomize-controller:v1.6.1"))
	g.Expect(jobs[1].dst()).To(Equal("registry.example.com/flux/kustomize-controller:v1.6.1"))

	// Operator
	g.Expect(jobs[2].src()).To(Equal("ghcr.io/controlplaneio-fluxcd/flux-operator:v0.46.0"))
	g.Expect(jobs[2].dst()).To(Equal("registry.example.com/flux/flux-operator:v0.46.0"))
	g.Expect(jobs[2].digest).To(BeEmpty())

	// Chart (published without the "v" prefix)
	g.Expect(jobs[3].src()).To(Equal("ghcr.io/controlplaneio-fluxcd/charts/flux-operator:0.46.0"))
	g.Expect(jobs[3].dst()).To(Equal("registry.example.com/flux/charts/flux-operator:0.46.0"))
	g.Expect(jobs[3].tag).To(Equal("0.46.0"))
	g.Expect(jobs[3].digest).To(BeEmpty())

	// Without chart
	jobs = buildMirrorJobs(images, "registry.example.com/flux", true, false)
	g.Expect(jobs).To(HaveLen(3))
	for _, j := range jobs {
		g.Expect(j.dst()).ToNot(ContainSubstring("/charts/"))
	}

	// Without operator image (chart only)
	jobs = buildMirrorJobs(images, "registry.example.com/flux", false, true)
	g.Expect(jobs).To(HaveLen(3)) // 2 controllers + chart
	for _, j := range jobs {
		g.Expect(j.dst()).ToNot(Equal("registry.example.com/flux/flux-operator:v0.46.0"))
	}
	g.Expect(jobs[2].dst()).To(Equal("registry.example.com/flux/charts/flux-operator:0.46.0"))

	// Controllers only
	jobs = buildMirrorJobs(images, "registry.example.com/flux", false, false)
	g.Expect(jobs).To(HaveLen(2)) // 2 controllers, no operator, no chart
}

func TestDistroMirror_BuildJobsHandlesNestedSourceRegistry(t *testing.T) {
	g := NewWithT(t)

	// Enterprise variants live under ghcr.io/controlplaneio-fluxcd/<variant>/<component>.
	// path.Base() must extract just the component name.
	images := []builder.ComponentImage{
		{
			Name:       "source-controller",
			Repository: "ghcr.io/controlplaneio-fluxcd/distroless/source-controller",
			Tag:        "v1.6.2",
			Digest:     "sha256:cccc",
		},
	}

	origVersion := VERSION
	VERSION = "0.46.0"
	defer func() { VERSION = origVersion }()

	jobs := buildMirrorJobs(images, "registry.example.com/flux", true, false)
	g.Expect(jobs).To(HaveLen(2)) // 1 controller + operator
	g.Expect(jobs[0].dst()).To(Equal("registry.example.com/flux/source-controller:v1.6.2"))
}

func TestDistroMirror_OperatorTagStripsVPrefix(t *testing.T) {
	g := NewWithT(t)

	origVersion := VERSION
	defer func() { VERSION = origVersion }()

	// Both with and without "v" prefix should produce the same tag.
	for _, v := range []string{"0.46.0", "v0.46.0"} {
		VERSION = v
		jobs := buildMirrorJobs(nil, "r.example.com", true, false)
		g.Expect(jobs).To(HaveLen(1)) // operator only
		g.Expect(jobs[0].src()).To(Equal("ghcr.io/controlplaneio-fluxcd/flux-operator:v0.46.0"))
		g.Expect(jobs[0].dst()).To(Equal("r.example.com/flux-operator:v0.46.0"))
	}
}

func TestDistroMirror_DryRunSkipsRegistry(t *testing.T) {
	g := NewWithT(t)

	job := mirrorJob{
		srcRepo: "ghcr.io/fluxcd/source-controller",
		dstRepo: "registry.example.com/flux/source-controller",
		tag:     "v1.6.2",
	}
	res, err := runMirrorJob(job, false, true, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.action).To(Equal(mirrorDryRun))
	g.Expect(res.src).To(Equal(job.src()))
	g.Expect(res.dst).To(Equal(job.dst()))
}

func TestDistroMirror_IsNotFound(t *testing.T) {
	g := NewWithT(t)

	g.Expect(isNotFound(nil)).To(BeFalse())
	g.Expect(isNotFound(errors.New("boom"))).To(BeFalse())

	g.Expect(isNotFound(&transport.Error{StatusCode: 404})).To(BeTrue())
	g.Expect(isNotFound(&transport.Error{StatusCode: 500})).To(BeFalse())

	// Wrapped error is also recognized.
	g.Expect(isNotFound(errors.Join(errors.New("ctx"), &transport.Error{StatusCode: 404}))).To(BeTrue())
}

func TestDistroMirror_BuildKeychain(t *testing.T) {
	g := NewWithT(t)

	// No token → DefaultKeychain.
	g.Expect(buildMirrorKeychain("ghcr.io", "")).To(Equal(authn.DefaultKeychain))

	// With token → multi-keychain that resolves the source registry to the
	// static authenticator and other registries to anonymous (chained with
	// DefaultKeychain).
	kc := buildMirrorKeychain("ghcr.io", "secret")
	g.Expect(kc).ToNot(BeNil())

	srcRef, err := name.NewRepository("ghcr.io/fluxcd/source-controller")
	g.Expect(err).ToNot(HaveOccurred())
	auth, err := kc.Resolve(srcRef)
	g.Expect(err).ToNot(HaveOccurred())
	cfg, err := auth.Authorization()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg.Username).To(Equal("flux"))
	g.Expect(cfg.Password).To(Equal("secret"))

	// A non-source registry resolves to anonymous (or whatever DefaultKeychain returns for it).
	otherRef, err := name.NewRepository("other.example.com/foo/bar")
	g.Expect(err).ToNot(HaveOccurred())
	otherAuth, err := kc.Resolve(otherRef)
	g.Expect(err).ToNot(HaveOccurred())
	otherCfg, err := otherAuth.Authorization()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(otherCfg.Password).ToNot(Equal("secret"))
}

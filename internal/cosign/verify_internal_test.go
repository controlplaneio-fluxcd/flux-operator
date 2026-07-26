// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package cosign

import (
	"bytes"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"
)

func TestValidateSigstoreBundleDescriptor(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{name: "rejects negative size", size: -1, wantErr: true},
		{name: "accepts empty layer", size: 0},
		{name: "accepts maximum size", size: maxSigstoreBundleSize},
		{name: "rejects oversized layer", size: maxSigstoreBundleSize + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateSigstoreBundleDescriptor(v1.Descriptor{Size: tt.size})
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestReadSigstoreBundleLayer(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{name: "accepts small bundle", size: 2},
		{name: "accepts maximum size", size: maxSigstoreBundleSize},
		{name: "rejects expanded bundle", size: maxSigstoreBundleSize + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			content := bytes.Repeat([]byte("x"), int(tt.size))
			layer := static.NewLayer(content, types.MediaType(sigstoreBundleMediaTypePrefix+".v0.3+json"))

			data, err := readSigstoreBundleLayer(layer)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("exceeds maximum size"))
				g.Expect(data).To(BeNil())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(data).To(HaveLen(int(tt.size)))
			}
		})
	}
}

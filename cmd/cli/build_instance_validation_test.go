// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateInstanceShardingStorage(t *testing.T) {
	newInstance := func(storage *fluxcdv1.Storage, shardingStorage string) *fluxcdv1.FluxInstance {
		return &fluxcdv1.FluxInstance{
			Spec: fluxcdv1.FluxInstanceSpec{
				Distribution: fluxcdv1.Distribution{
					Version:  "2.x",
					Registry: "ghcr.io/fluxcd",
				},
				Sharding: &fluxcdv1.Sharding{
					Shards:  []string{"shard1"},
					Storage: shardingStorage,
				},
				Storage: storage,
			},
		}
	}

	tests := []struct {
		name    string
		obj     *fluxcdv1.FluxInstance
		wantErr string
	}{
		{
			name: "allows ephemeral sharding storage without artifact storage",
			obj:  newInstance(nil, "ephemeral"),
		},
		{
			name: "allows persistent sharding storage with artifact storage",
			obj: newInstance(&fluxcdv1.Storage{
				Class: "standard",
				Size:  "10Gi",
			}, "persistent"),
		},
		{
			name:    "rejects persistent sharding storage without artifact storage",
			obj:     newInstance(nil, "persistent"),
			wantErr: ".spec.storage must be set when .spec.sharding.storage is 'persistent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateInstance(tt.obj)
			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(tt.wantErr))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"slices"
	"testing"
)

func TestResolveWatchNamespaces(t *testing.T) {
	const rt = "flux-system"
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{name: "empty input is no-op", in: nil, want: nil},
		{name: "single namespace adds runtime", in: []string{"app-a"}, want: []string{"app-a", "flux-system"}},
		{name: "comma separated", in: []string{"app-a,app-b"}, want: []string{"app-a", "app-b", "flux-system"}},
		{name: "repeated flags", in: []string{"app-a", "app-b"}, want: []string{"app-a", "app-b", "flux-system"}},
		{name: "dedup including runtime", in: []string{"app-a", "app-a", "flux-system"}, want: []string{"app-a", "flux-system"}},
		{name: "whitespace trimmed", in: []string{" app-a , app-b "}, want: []string{"app-a", "app-b", "flux-system"}},
		{name: "all-empty tokens error", in: []string{" , "}, wantErr: true},
		{name: "invalid dns label error", in: []string{"Bad_NS"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveWatchNamespaces(rt, tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveWatchNamespaces() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("resolveWatchNamespaces() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package spiffe

import (
	"errors"
	"testing"

	fluxspiffe "github.com/fluxcd/pkg/spiffe"
	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// testTLSSource is a TLSSource stub; ArtifactServerPeers only checks it is set.
type testTLSSource struct{}

func (testTLSSource) GetX509SVID() (*x509svid.SVID, error) {
	return nil, errors.New("not implemented")
}

func (testTLSSource) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return nil, errors.New("not implemented")
}

// newTestInstance returns a FluxInstance with the given SPIFFE mTLS settings.
func newTestInstance(name string, mtls *fluxcdv1.MTLS, components ...fluxcdv1.Component) *fluxcdv1.FluxInstance {
	return &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system"},
		Spec: fluxcdv1.FluxInstanceSpec{
			Components: components,
			SPIFFE: &fluxcdv1.SPIFFE{
				WorkloadAPI: fluxcdv1.WorkloadAPI{Endpoint: "unix:///spiffe-workload-api/agent.sock"},
				MTLS:        mtls,
			},
		},
	}
}

// newTestClient returns a fake client holding the given objects.
func newTestClient(g *WithT, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestArtifactServerPeers(t *testing.T) {
	enabled := &fluxcdv1.MTLS{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
	}

	testCases := []struct {
		name    string
		objs    []client.Object
		source  bool
		want    []string
		wantErr string
	}{
		{
			name:   "no FluxInstance",
			source: true,
		},
		{
			name:   "mTLS disabled",
			objs:   []client.Object{newTestInstance("flux", &fluxcdv1.MTLS{Enabled: false})},
			source: true,
		},
		{
			name:   "mTLS not configured",
			objs:   []client.Object{newTestInstance("flux", nil)},
			source: true,
		},
		{
			name:   "default components",
			objs:   []client.Object{newTestInstance("flux", enabled)},
			source: true,
			want:   []string{"spiffe://cluster.local/ns/flux-system/sa/source-controller"},
		},
		{
			name: "source-controller and source-watcher",
			objs: []client.Object{newTestInstance("flux", enabled,
				fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher, fluxcdv1.FluxKustomizeController)},
			source: true,
			want: []string{
				"spiffe://cluster.local/ns/flux-system/sa/source-controller",
				"spiffe://cluster.local/ns/flux-system/sa/source-watcher",
			},
		},
		{
			name:    "missing source",
			objs:    []client.Object{newTestInstance("flux", enabled)},
			wantErr: "SPIFFE Workload API source is not configured",
		},
		{
			name: "no artifact servers",
			objs: []client.Object{newTestInstance("flux", enabled,
				fluxcdv1.FluxKustomizeController)},
			source:  true,
			wantErr: "no artifact server SPIFFE IDs were found",
		},
		{
			name: "multiple FluxInstances",
			objs: []client.Object{
				newTestInstance("flux1", enabled),
				newTestInstance("flux2", enabled),
			},
			source:  true,
			wantErr: "multiple FluxInstances found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			var tlsSource fluxspiffe.TLSSource
			if tc.source {
				tlsSource = testTLSSource{}
			}

			peers, err := ArtifactServerPeers(t.Context(), newTestClient(g, tc.objs...), tlsSource, nil)
			if tc.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.wantErr)))
				g.Expect(peers).To(BeNil())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(peers).To(Equal(tc.want))
		})
	}
}

func TestArtifactServerPeersUsesGivenInstance(t *testing.T) {
	g := NewWithT(t)

	instance := newTestInstance("flux", &fluxcdv1.MTLS{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
	})

	peers, err := ArtifactServerPeers(t.Context(), nil, testTLSSource{}, instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(peers).To(Equal([]string{"spiffe://cluster.local/ns/flux-system/sa/source-controller"}))
}

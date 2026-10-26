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

// defaultComponents are the components of the default distribution.
var defaultComponents = []string{
	fluxcdv1.FluxSourceController,
	fluxcdv1.FluxKustomizeController,
	fluxcdv1.FluxHelmController,
	fluxcdv1.FluxNotificationController,
}

// newTestInstance returns a FluxInstance with the given applied mTLS
// settings and components.
func newTestInstance(name string, mtls *fluxcdv1.MTLSStatus, components ...string) *fluxcdv1.FluxInstance {
	if len(components) == 0 {
		components = defaultComponents
	}
	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system"},
	}
	for _, c := range components {
		obj.Status.Components = append(obj.Status.Components, fluxcdv1.ComponentImage{Name: c})
	}
	if mtls != nil {
		obj.Status.SPIFFE = &fluxcdv1.SPIFFEStatus{MTLS: mtls}
	}
	return obj
}

// newTestClient returns a fake client holding the given objects.
func newTestClient(g *WithT, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestArtifactServerPeers(t *testing.T) {
	enabled := &fluxcdv1.MTLSStatus{
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
			objs:   []client.Object{newTestInstance("flux", &fluxcdv1.MTLSStatus{Enabled: false})},
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

	instance := newTestInstance("flux", &fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
	})

	peers, err := ArtifactServerPeers(t.Context(), nil, testTLSSource{}, instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(peers).To(Equal([]string{"spiffe://cluster.local/ns/flux-system/sa/source-controller"}))
}

func TestAppliedMTLSEnabled(t *testing.T) {
	template := "spiffe://cluster.local/ns/flux-system/sa/{component}"
	specMTLS := func(enabled bool) *fluxcdv1.SPIFFE {
		return &fluxcdv1.SPIFFE{
			MTLS: &fluxcdv1.MTLS{
				Enabled:             enabled,
				ComponentIDTemplate: "spiffe://spec.local/ns/flux-system/sa/{component}",
			},
		}
	}

	testCases := []struct {
		name     string
		spec     *fluxcdv1.SPIFFE
		status   *fluxcdv1.MTLSStatus
		enabled  bool
		sourceID string
		eventID  string
	}{
		{
			name:    "enabled in spec but not applied",
			spec:    specMTLS(true),
			enabled: false,
		},
		{
			name:    "enabled in spec and disabled in status",
			spec:    specMTLS(true),
			status:  &fluxcdv1.MTLSStatus{Enabled: false},
			enabled: false,
		},
		{
			name:     "disabled in spec but still applied",
			spec:     specMTLS(false),
			status:   &fluxcdv1.MTLSStatus{Enabled: true, ComponentIDTemplate: template},
			enabled:  true,
			sourceID: "spiffe://cluster.local/ns/flux-system/sa/source-controller",
			eventID:  "spiffe://cluster.local/ns/flux-system/sa/notification-controller",
		},
		{
			name:     "enabled in spec and status",
			spec:     specMTLS(true),
			status:   &fluxcdv1.MTLSStatus{Enabled: true, ComponentIDTemplate: template},
			enabled:  true,
			sourceID: "spiffe://cluster.local/ns/flux-system/sa/source-controller",
			eventID:  "spiffe://cluster.local/ns/flux-system/sa/notification-controller",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			instance := newTestInstance("flux", tc.status)
			instance.Spec.SPIFFE = tc.spec

			g.Expect(AppliedMTLSEnabled(instance)).To(Equal(tc.enabled))

			peers, err := ArtifactServerPeers(t.Context(), nil, testTLSSource{}, instance)
			g.Expect(err).NotTo(HaveOccurred())
			eventID, err := EventServerPeer(testTLSSource{}, instance)
			g.Expect(err).NotTo(HaveOccurred())

			if !tc.enabled {
				g.Expect(peers).To(BeNil())
				g.Expect(eventID).To(BeEmpty())
				return
			}
			g.Expect(peers).To(Equal([]string{tc.sourceID}))
			g.Expect(eventID).To(Equal(tc.eventID))
		})
	}
}

func TestArtifactServerIDsUsesAppliedComponents(t *testing.T) {
	g := NewWithT(t)

	instance := newTestInstance("flux", &fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
	}, fluxcdv1.FluxSourceController, fluxcdv1.FluxSourceWatcher)
	// Components added to the spec are not installed yet.
	instance.Spec.Components = []fluxcdv1.Component{fluxcdv1.FluxSourceController}

	g.Expect(ArtifactServerIDs(instance)).To(Equal([]string{
		"spiffe://cluster.local/ns/flux-system/sa/source-controller",
		"spiffe://cluster.local/ns/flux-system/sa/source-watcher",
	}))
}

func TestEventServerPeer(t *testing.T) {
	enabled := &fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
	}

	testCases := []struct {
		name     string
		instance *fluxcdv1.FluxInstance
		source   bool
		want     string
		wantErr  string
	}{
		{
			name:     "nil instance",
			instance: nil,
			source:   true,
		},
		{
			name:     "mTLS not applied",
			instance: newTestInstance("flux", nil),
			source:   true,
		},
		{
			name:     "notification-controller installed",
			instance: newTestInstance("flux", enabled),
			source:   true,
			want:     "spiffe://cluster.local/ns/flux-system/sa/notification-controller",
		},
		{
			name:     "missing source",
			instance: newTestInstance("flux", enabled),
			wantErr:  "SPIFFE Workload API source is not configured",
		},
		{
			name:     "notification-controller not installed",
			instance: newTestInstance("flux", enabled, fluxcdv1.FluxSourceController),
			source:   true,
			wantErr:  "notification-controller is not installed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			var tlsSource fluxspiffe.TLSSource
			if tc.source {
				tlsSource = testTLSSource{}
			}

			id, err := EventServerPeer(tlsSource, tc.instance)
			if tc.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.wantErr)))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(id).To(Equal(tc.want))
		})
	}
}

func TestIsSelfTrustDomainPeer(t *testing.T) {
	testCases := []struct {
		peer string
		want bool
	}{
		{peer: "spiffe://self", want: true},
		{peer: "spiffe://self/ns/flux-system/sa/source-controller", want: true},
		{peer: "spiffe://self/ns/flux-system/", want: true},
		{peer: "spiffe://selfish.org/ns/flux-system/sa/source-controller", want: false},
		{peer: "spiffe://cluster.local/ns/flux-system/sa/source-controller", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.peer, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(IsSelfTrustDomainPeer(tc.peer)).To(Equal(tc.want))
		})
	}
}

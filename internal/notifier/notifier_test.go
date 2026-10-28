// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package notifier

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestAddress(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		clusterDomain string
		expected      string
	}{
		{
			name:          "default domain",
			namespace:     "flux-system",
			clusterDomain: "cluster.local",
			expected:      "http://notification-controller.flux-system.svc.cluster.local./",
		},
		{
			name:          "custom domain",
			namespace:     "custom-ns",
			clusterDomain: "my.domain.io",
			expected:      "http://notification-controller.custom-ns.svc.my.domain.io./",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			addr := Address(tt.namespace, tt.clusterDomain)
			g.Expect(addr).To(Equal(tt.expected))
		})
	}
}

func TestNew_WithEnvAddress(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", "http://custom-address:9090/")

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme)
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom())
	g.Expect(addr).To(Equal("http://custom-address:9090/"))
}

func TestNew_NotificationsDisabled(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("NOTIFICATIONS_DISABLED", "true")

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme)
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom())
	g.Expect(addr).To(Equal(""))
}

func TestNew_NoClientNoInstance(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme)
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom())
	g.Expect(addr).To(Equal(""))
}

func TestNew_WithFluxInstance_HasNotificationController(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Components: []fluxcdv1.Component{
				"source-controller",
				"kustomize-controller",
				"notification-controller",
			},
			Cluster: &fluxcdv1.Cluster{
				Domain: "cluster.local",
			},
		},
	}

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme, WithFluxInstance(instance))
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom(WithFluxInstance(instance)))
	g.Expect(addr).To(Equal("http://notification-controller.flux-system.svc.cluster.local./"))
}

func TestNew_WithFluxInstance_NoNotificationController(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Components: []fluxcdv1.Component{
				"source-controller",
				"kustomize-controller",
			},
		},
	}

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme, WithFluxInstance(instance))
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom(WithFluxInstance(instance)))
	g.Expect(addr).To(Equal(""))
}

func TestNew_WithClient_NoFluxInstances(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme, WithClient(kubeClient))
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom(WithClient(kubeClient)))
	g.Expect(addr).To(Equal(""))
}

func TestNew_WithClient_SingleFluxInstance(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Components: []fluxcdv1.Component{
				"source-controller",
				"notification-controller",
			},
			Cluster: &fluxcdv1.Cluster{
				Domain: "cluster.local",
			},
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance).
		Build()

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme, WithClient(kubeClient))
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom(WithClient(kubeClient)))
	g.Expect(addr).To(Equal("http://notification-controller.flux-system.svc.cluster.local./"))
}

func TestNew_WithClient_MultipleFluxInstances(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	instance1 := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-1",
			Namespace: "flux-system",
		},
	}
	instance2 := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-2",
			Namespace: "other-ns",
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance1, instance2).
		Build()

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	er := New(ctx, base, scheme, WithClient(kubeClient))
	g.Expect(er).NotTo(BeNil())
	_, addr := eventsAddress(ctx, optionsFrom(WithClient(kubeClient)))
	g.Expect(addr).To(Equal(""))
}

func optionsFrom(opts ...Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// resetRecorderCache empties the recorder cache before and after a test.
func resetRecorderCache(t *testing.T) {
	t.Helper()
	reset := func() {
		recorderCache.mu.Lock()
		defer recorderCache.mu.Unlock()
		recorderCache.entries = map[record.EventRecorder]recorderCacheEntry{}
	}
	reset()
	t.Cleanup(reset)
}

// testTLSSource returns an SVID in a configurable trust domain.
type testTLSSource struct {
	trustDomain string
}

// GetX509SVID returns an SVID in the configured trust domain.
func (s *testTLSSource) GetX509SVID() (*x509svid.SVID, error) {
	td := spiffeid.RequireTrustDomainFromString(s.trustDomain)
	return &x509svid.SVID{ID: spiffeid.RequireFromPath(td, "/flux-operator")}, nil
}

// GetX509BundleForTrustDomain is not used by the tests.
func (s *testTLSSource) GetX509BundleForTrustDomain(spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return nil, errors.New("not implemented")
}

// newMTLSInstance returns a FluxInstance with notification-controller
// applied with the given mTLS settings.
func newMTLSInstance(mtls *fluxcdv1.MTLSStatus) *fluxcdv1.FluxInstance {
	return &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "flux", Namespace: "flux-system"},
		Status: fluxcdv1.FluxInstanceStatus{
			Components: []fluxcdv1.ComponentImage{
				{Name: fluxcdv1.FluxSourceController},
				{Name: fluxcdv1.FluxNotificationController},
			},
			SPIFFE: &fluxcdv1.SPIFFEStatus{MTLS: mtls},
		},
	}
}

func TestNew_ReusesRecorder(t *testing.T) {
	g := NewWithT(t)
	resetRecorderCache(t)

	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", "http://notification-controller-a/")

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	first := New(ctx, base, scheme)
	g.Expect(New(ctx, base, scheme)).To(BeIdenticalTo(first))

	// Another base recorder gets its own recorder.
	other := New(ctx, record.NewFakeRecorder(10), scheme)
	g.Expect(other).NotTo(BeIdenticalTo(first))
	g.Expect(New(ctx, base, scheme)).To(BeIdenticalTo(first))

	// An address change builds a new recorder.
	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", "http://notification-controller-b/")
	second := New(ctx, base, scheme)
	g.Expect(second).NotTo(BeIdenticalTo(first))
	g.Expect(New(ctx, base, scheme)).To(BeIdenticalTo(second))
}

func TestNew_RecorderFollowsMTLS(t *testing.T) {
	g := NewWithT(t)
	resetRecorderCache(t)

	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", "http://notification-controller/")

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	base := record.NewFakeRecorder(10)
	ctx := context.Background()
	source := &testTLSSource{trustDomain: "cluster.local"}

	newWith := func(mtls *fluxcdv1.MTLSStatus) any {
		return New(ctx, base, scheme, WithFluxInstance(newMTLSInstance(mtls)), WithSPIFFESource(source))
	}

	plain := newWith(nil)
	g.Expect(newWith(&fluxcdv1.MTLSStatus{Enabled: false})).To(BeIdenticalTo(plain))

	explicit := &fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
	}
	withMTLS := newWith(explicit)
	g.Expect(withMTLS).NotTo(BeIdenticalTo(plain))
	g.Expect(newWith(explicit)).To(BeIdenticalTo(withMTLS))

	// Explicit IDs do not depend on the local trust domain.
	source.trustDomain = "example.org"
	g.Expect(newWith(explicit)).To(BeIdenticalTo(withMTLS))

	// A different notification-controller ID builds a new recorder.
	otherID := newWith(&fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://cluster.local/ns/flux/sa/{component}",
	})
	g.Expect(otherID).NotTo(BeIdenticalTo(withMTLS))

	// Self-form IDs follow the local trust domain.
	self := &fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://self/ns/flux-system/sa/{component}",
	}
	selfRecorder := newWith(self)
	g.Expect(selfRecorder).NotTo(BeIdenticalTo(otherID))
	g.Expect(newWith(self)).To(BeIdenticalTo(selfRecorder))
	source.trustDomain = "cluster.local"
	g.Expect(newWith(self)).NotTo(BeIdenticalTo(selfRecorder))

	// Disabling mTLS builds a plain recorder again.
	g.Expect(newWith(nil)).NotTo(BeIdenticalTo(plain))
}

func TestNew_MTLSFromAppliedState(t *testing.T) {
	g := NewWithT(t)
	resetRecorderCache(t)

	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", "http://notification-controller/")

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	base := record.NewFakeRecorder(10)
	ctx := context.Background()
	source := &testTLSSource{trustDomain: "cluster.local"}

	// mTLS enabled only in the spec is ignored.
	instance := newMTLSInstance(nil)
	instance.Spec.SPIFFE = &fluxcdv1.SPIFFE{
		MTLS: &fluxcdv1.MTLS{
			Enabled:             true,
			ComponentIDTemplate: "spiffe://cluster.local/ns/flux-system/sa/{component}",
		},
	}

	o := optionsFrom(WithFluxInstance(instance), WithSPIFFESource(source))
	peer, key, err := eventMTLS(o.spiffeSource, instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(peer).To(BeEmpty())
	g.Expect(key).To(BeEmpty())
	g.Expect(New(ctx, base, scheme, WithFluxInstance(instance), WithSPIFFESource(source))).NotTo(BeNil())
}

func TestNew_ReusesConnection(t *testing.T) {
	g := NewWithT(t)
	resetRecorderCache(t)

	var newConns, requests atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConns.Add(1)
		}
	}
	server.Start()
	defer server.Close()

	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", server.URL)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	base := record.NewFakeRecorder(100)
	ctx := context.Background()

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "flux", Namespace: "flux-system"},
	}

	const events = 20
	for i := range events {
		New(ctx, base, scheme).AnnotatedEventf(obj, nil, nil,
			corev1.EventTypeNormal, "ReconciliationSucceeded", "reconcile", "event %d", i)
	}

	g.Expect(requests.Load()).To(Equal(int32(events)))
	g.Expect(newConns.Load()).To(Equal(int32(1)))
}

func TestEventsAddress_AppliedComponents(t *testing.T) {
	const addr = "http://notification-controller.flux-system.svc.cluster.local./"
	testCases := []struct {
		name    string
		spec    []fluxcdv1.Component
		applied []string
		want    string
	}{
		{
			name: "before the first apply uses the spec",
			spec: []fluxcdv1.Component{"source-controller", "notification-controller"},
			want: addr,
		},
		{
			name:    "added to the spec but not applied",
			spec:    []fluxcdv1.Component{"source-controller", "notification-controller"},
			applied: []string{"source-controller"},
			want:    "",
		},
		{
			name:    "removed from the spec but still applied",
			spec:    []fluxcdv1.Component{"source-controller"},
			applied: []string{"source-controller", "notification-controller"},
			want:    addr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			instance := &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{Name: "flux", Namespace: "flux-system"},
				Spec: fluxcdv1.FluxInstanceSpec{
					Components: tc.spec,
					Cluster:    &fluxcdv1.Cluster{Domain: "cluster.local"},
				},
			}
			for _, c := range tc.applied {
				instance.Status.Components = append(instance.Status.Components, fluxcdv1.ComponentImage{Name: c})
			}

			_, got := eventsAddress(context.Background(), optionsFrom(WithFluxInstance(instance)))
			g.Expect(got).To(Equal(tc.want))
		})
	}
}

func TestNew_ConcurrentCallsShareRecorder(t *testing.T) {
	g := NewWithT(t)
	resetRecorderCache(t)

	t.Setenv("NOTIFICATION_CONTROLLER_ADDRESS", "http://notification-controller/")

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	base := record.NewFakeRecorder(10)
	source := &testTLSSource{trustDomain: "cluster.local"}
	instance := newMTLSInstance(&fluxcdv1.MTLSStatus{
		Enabled:             true,
		ComponentIDTemplate: "spiffe://self/ns/flux-system/sa/{component}",
	})

	const callers = 20
	recorders := make([]any, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			recorders[i] = New(context.Background(), base, scheme,
				WithFluxInstance(instance), WithSPIFFESource(source))
		})
	}
	wg.Wait()

	for _, er := range recorders {
		g.Expect(er).To(BeIdenticalTo(recorders[0]))
	}
}

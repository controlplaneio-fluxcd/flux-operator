// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package notifier

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
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
	// When NOTIFICATION_CONTROLLER_ADDRESS is set, the recorder should be
	// created regardless of flux instance presence.
	g.Expect(er).NotTo(Equal(base))
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
	// Events address is empty, so the recorder is still created (with empty webhook).
}

func TestNew_NoClientNoInstance(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	base := record.NewFakeRecorder(10)
	ctx := context.Background()

	// No client and no flux instance provided — should fall back to base.
	er := New(ctx, base, scheme)
	g.Expect(er).To(Equal(base))
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
	// With notification-controller present, should create an external recorder.
	g.Expect(er).NotTo(Equal(base))
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
	// Without notification-controller, eventsAddr is empty.
	// The recorder is created with an empty webhook address.
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

	// Client provided but no FluxInstance objects exist — should fall back to base.
	er := New(ctx, base, scheme, WithClient(kubeClient))
	g.Expect(er).To(Equal(base))
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
	g.Expect(er).NotTo(Equal(base))
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

	// Multiple instances — should fall back to base.
	er := New(ctx, base, scheme, WithClient(kubeClient))
	g.Expect(er).To(Equal(base))
}

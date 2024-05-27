// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1alpha1 "github.com/controlplaneio-fluxcd/fluxcd-operator/api/v1alpha1"
)

func TestFluxInstanceReconciler_Install(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1alpha1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns.Name,
			Namespace: ns.Name,
		},
	}

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = getFluxInstanceReconciler().Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1alpha1.FluxInstance{}
	err = testEnv.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	logObjectStatus(t, result)

	checkInstanceReadiness(g, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.SucceededReason))

	events := getEvents(result.Name)
	g.Expect(events).ToNot(BeEmpty())
	g.Expect(events[0].Type).To(Equal(corev1.EventTypeNormal))
	g.Expect(events[0].Reason).To(Equal(meta.SucceededReason))
}

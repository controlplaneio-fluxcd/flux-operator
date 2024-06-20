// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
)

func TestEntitlementReconciler_ReconcileDefaultVendor(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	reconciler := getEntitlementReconciler(ns.Name)
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ns)})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Requeue).To(BeTrue())

	secret, err := reconciler.GetEntitlementSecret(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(secret.Data).To(HaveKeyWithValue(entitlement.VendorKey, []byte(entitlement.DefaultVendor)))
	g.Expect(secret.Data).To(HaveKey(entitlement.TokenKey))

	result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ns)})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(Equal(30 * time.Minute))

	dc := &entitlement.DefaultClient{Vendor: entitlement.DefaultVendor}
	token, err := dc.RegisterUsage(ctx, string(ns.UID))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(secret.Data).To(HaveKeyWithValue(entitlement.TokenKey, []byte(token)))
}

func TestEntitlementReconciler_InitEntitlementSecret(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	reconciler := getEntitlementReconciler(ns.Name)

	rs, err := reconciler.InitEntitlementSecret(ctx)
	g.Expect(err).ToNot(HaveOccurred())

	secret := &corev1.Secret{}
	err = reconciler.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: rs.Name}, secret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(secret.Data).To(HaveKeyWithValue(entitlement.VendorKey, []byte(entitlement.DefaultVendor)))
}

func getEntitlementReconciler(ns string) *EntitlementReconciler {
	ec, err := entitlement.NewClient()
	if err != nil {
		panic(err)
	}
	return &EntitlementReconciler{
		Client:            testClient,
		EventRecorder:     testEnv.GetEventRecorderFor(controllerName),
		Scheme:            NewTestScheme(),
		StatusPoller:      polling.NewStatusPoller(testClient, testEnv.GetRESTMapper(), polling.Options{}),
		StatusManager:     controllerName,
		WatchNamespace:    ns,
		EntitlementClient: ec,
	}
}

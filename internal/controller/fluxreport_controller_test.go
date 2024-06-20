// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
)

func TestFluxReportReconciler_Reconcile(t *testing.T) {
	g := NewWithT(t)
	instRec := getFluxInstanceReconciler()
	reportRec := getFluxReportReconciler()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns.Name,
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(),
	}

	err = testEnv.Create(ctx, instance)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	r, err := instRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(instance),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile the instance.
	r, err = instRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(instance),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the instance was installed.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(instance), instance)
	g.Expect(err).ToNot(HaveOccurred())
	checkInstanceReadiness(g, instance)

	report := &fluxcdv1.FluxReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluxcdv1.DefaultInstanceName,
			Namespace: ns.Name,
		},
	}

	// Initialize the report.
	err = reportRec.initReport(ctx, report.GetName(), report.GetNamespace())
	g.Expect(err).ToNot(HaveOccurred())

	// Compute instance report.
	r, err = reportRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(report),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Read the report.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(report), report)
	g.Expect(err).ToNot(HaveOccurred())
	logObject(t, report)

	// Check reported components.
	g.Expect(report.Spec.ComponentsStatus).To(HaveLen(len(instance.Status.Components)))
	g.Expect(report.Spec.ComponentsStatus[0].Name).To(Equal("helm-controller"))
	g.Expect(report.Spec.ComponentsStatus[0].Image).To(ContainSubstring("fluxcd/helm-controller"))

	// Check reported distribution.
	g.Expect(instance.Status.LastAppliedRevision).To(ContainSubstring(report.Spec.Distribution.Version))
	g.Expect(report.Spec.Distribution.Status).To(Equal("Installed"))
	g.Expect(report.Spec.Distribution.Entitlement).To(Equal("Unknown"))
	g.Expect(report.Spec.Distribution.ManagedBy).To(Equal("flux-operator"))

	// Check reported reconcilers.
	g.Expect(report.Spec.ReconcilersStatus).To(HaveLen(10))
	g.Expect(report.Spec.ReconcilersStatus[9].Kind).To(Equal("OCIRepository"))
	g.Expect(report.Spec.ReconcilersStatus[9].Stats.Running).To(Equal(1))

	// Check reported sync.
	g.Expect(report.Spec.SyncStatus).ToNot(BeNil())
	g.Expect(report.Spec.SyncStatus.Source).To(Equal(instance.Spec.Sync.URL))

	// Check ready condition.
	g.Expect(conditions.GetReason(report, meta.ReadyCondition)).To(BeIdenticalTo(meta.SucceededReason))

	// Delete the instance.
	err = testClient.Delete(ctx, instance)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = instRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(instance),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Generate entitlement secret.
	entRec := getEntitlementReconciler(ns.Name)
	_, err = entRec.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(ns)})
	g.Expect(err).ToNot(HaveOccurred())

	// Generate the report with the instance deleted.
	r, err = reportRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(report),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Read the report and verify distribution.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(report), report)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(report.Spec.Distribution.Status).To(Equal("Not Installed"))
	g.Expect(report.Spec.Distribution.Entitlement).To(Equal("Issued by " + entitlement.DefaultVendor))
}

func getFluxReportReconciler() *FluxReportReconciler {
	return &FluxReportReconciler{
		Client:        testClient,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
		Scheme:        NewTestScheme(),
		StatusManager: controllerName,
	}
}

// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestFluxReportReconciler_CELNameValidation(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	report := &fluxcdv1.FluxReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fluxx", // Invalid name
			Namespace: ns.Name,
		},
	}

	err = testEnv.Create(ctx, report)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("the only accepted name for a FluxReport is 'flux'"))
}

func TestFluxReportReconciler_Reconcile(t *testing.T) {
	g := NewWithT(t)
	instRec := getFluxInstanceReconciler(t)
	instSpec := getDefaultFluxSpec(t)
	reportRec := getFluxReportReconciler()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the report.
	report := &fluxcdv1.FluxReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluxcdv1.DefaultInstanceName,
			Namespace: ns.Name,
		},
	}
	err = reportRec.initReport(ctx, report.GetName(), report.GetNamespace())
	g.Expect(err).ToNot(HaveOccurred())

	// Create the Flux instance.
	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: instSpec,
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

	// Compute instance report.
	r, err = reportRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(report),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Read the report.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(report), report)
	g.Expect(err).ToNot(HaveOccurred())
	testutils.LogObject(t, report)

	// Check annotation set by the instance reconciler.
	g.Expect(report.GetAnnotations()).To(HaveKey(meta.ReconcileRequestAnnotation))

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
	g.Expect(report.Spec.ReconcilersStatus).To(HaveLen(13))
	g.Expect(report.Spec.ReconcilersStatus[12].Kind).To(Equal("OCIRepository"))
	g.Expect(report.Spec.ReconcilersStatus[12].Stats.Running).To(Equal(1))

	// Check reported sync.
	g.Expect(report.Spec.SyncStatus).ToNot(BeNil())
	g.Expect(report.Spec.SyncStatus.Source).To(Equal(instance.Spec.Sync.URL))
	g.Expect(report.Spec.SyncStatus.ID).To(Equal("kustomization/" + ns.Name))

	// Check reported cluster.
	g.Expect(report.Spec.Cluster).ToNot(BeNil())
	g.Expect(report.Spec.Cluster.ServerVersion).To(ContainSubstring("v1."))
	g.Expect(report.Spec.Cluster.Platform).To(ContainSubstring("64"))

	// Check reported operator.
	g.Expect(report.Spec.Operator).ToNot(BeNil())
	g.Expect(report.Spec.Operator.APIVersion).To(Equal(fluxcdv1.GroupVersion.String()))
	g.Expect(report.Spec.Operator.Version).To(Equal("v0.0.0-dev"))
	g.Expect(report.Spec.Operator.Platform).To(ContainSubstring("/"))

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
	emptyReport := &fluxcdv1.FluxReport{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(report), emptyReport)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(emptyReport.Spec.Distribution.Status).To(Equal("Not Installed"))
	g.Expect(emptyReport.Spec.Distribution.Entitlement).To(Equal("Issued by " + entitlement.DefaultVendor))
}

func TestFluxReportReconciler_CustomSyncName(t *testing.T) {
	g := NewWithT(t)
	instRec := getFluxInstanceReconciler(t)
	instSpec := getDefaultFluxSpec(t)
	instSpec.Distribution.Version = "2.x"
	reportRec := getFluxReportReconciler()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the report.
	report := &fluxcdv1.FluxReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluxcdv1.DefaultInstanceName,
			Namespace: ns.Name,
		},
	}
	err = reportRec.initReport(ctx, report.GetName(), report.GetNamespace())
	g.Expect(err).ToNot(HaveOccurred())

	// Create the Flux instance.
	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: instSpec,
	}

	// Set custom sync name.
	instance.Spec.Sync.Name = "custom-sync"

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

	// Compute instance report.
	rp, err := reportRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(report),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rp.RequeueAfter).To(BeEquivalentTo(time.Minute))

	// Read the report.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(report), report)
	g.Expect(err).ToNot(HaveOccurred())
	testutils.LogObject(t, report)

	// Check reported sync with custom name.
	g.Expect(report.Spec.SyncStatus).ToNot(BeNil())
	g.Expect(report.Spec.SyncStatus.Source).To(Equal(instance.Spec.Sync.URL))
	g.Expect(report.Spec.SyncStatus.ID).To(Equal("kustomization/" + instance.Spec.Sync.Name))

	// Check ready condition.
	g.Expect(conditions.GetReason(report, meta.ReadyCondition)).To(BeIdenticalTo(meta.SucceededReason))

	// Verify that sync name is immutable.
	inst := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(instance), inst)
	g.Expect(err).ToNot(HaveOccurred())

	inst.Spec.Sync.Name = "new-sync"
	err = testClient.Update(ctx, inst)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("immutable"))

	// Delete the instance.
	err = testClient.Delete(ctx, instance)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = instRec.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(instance),
	})
	g.Expect(err).ToNot(HaveOccurred())
}

func getFluxReportReconciler() *FluxReportReconciler {
	return &FluxReportReconciler{
		Client:            testClient,
		EventRecorder:     testEnv.GetEventRecorderFor(controllerName),
		Scheme:            NewTestScheme(),
		StatusManager:     controllerName,
		ReportingInterval: time.Minute,
		Version:           "v0.0.0-dev",
	}
}

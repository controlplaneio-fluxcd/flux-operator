// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1alpha1 "github.com/controlplaneio-fluxcd/fluxcd-operator/api/v1alpha1"
)

func TestFluxInstanceReconciler_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1alpha1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns.Name,
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(),
	}

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Check if the finalizer was added.
	resultInit := &fluxcdv1alpha1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultInit)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultInit.Finalizers).To(ContainElement(fluxcdv1alpha1.Finalizer))
	g.Expect(resultInit.Status.ObservedGeneration).To(BeEquivalentTo(-1))

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the instance was installed.
	result := &fluxcdv1alpha1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	checkInstanceReadiness(g, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check if the inventory was updated.
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_source-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_kustomize-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_helm-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_notification-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
	))

	// Update the instance.
	resultP := result.DeepCopy()
	resultP.SetAnnotations(map[string]string{
		fluxcdv1alpha1.ReconcileAnnotation:      fluxcdv1alpha1.EnabledValue,
		fluxcdv1alpha1.ReconcileEveryAnnotation: "1m",
	})
	resultP.Spec.Components = []fluxcdv1alpha1.Component{"source-controller", "kustomize-controller"}
	resultP.Spec.Cluster = &fluxcdv1alpha1.Cluster{
		NetworkPolicy: false,
	}
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the instance was scheduled for reconciliation.
	g.Expect(r.RequeueAfter).To(Equal(time.Minute))

	// Check the final status.
	resultFinal := &fluxcdv1alpha1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	logObjectStatus(t, resultFinal)
	g.Expect(resultFinal.Status.ObservedGeneration).To(BeEquivalentTo(resultFinal.Generation))
	g.Expect(resultFinal.Status.LastAttemptedRevision).To(HavePrefix("v2.3.0@sha256:"))
	g.Expect(resultFinal.Status.LastAppliedRevision).To(BeIdenticalTo(resultFinal.Status.LastAttemptedRevision))

	// Check if the inventory was updated.
	g.Expect(resultFinal.Status.Inventory.Entries).ToNot(ContainElements(
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_helm-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_notification-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1alpha1.ResourceRef{
			ID:      fmt.Sprintf("%s_allow-egress_networking.k8s.io_NetworkPolicy", ns.Name),
			Version: "v1",
		},
	))

	// Check if events were recorded for each step.
	events := getEvents(result.Name)
	g.Expect(events).To(HaveLen(4))
	g.Expect(events[0].Reason).To(Equal(meta.ProgressingReason))
	g.Expect(events[0].Message).To(HavePrefix("Installing"))
	g.Expect(events[1].Reason).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(events[1].Message).To(HavePrefix("Reconciliation finished"))
	g.Expect(events[2].Reason).To(Equal(meta.ProgressingReason))
	g.Expect(events[2].Message).To(HavePrefix("Upgrading"))
	g.Expect(events[3].Reason).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(events[3].Annotations).To(HaveKeyWithValue(fluxcdv1alpha1.RevisionAnnotation, resultFinal.Status.LastAppliedRevision))

	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the instance was uninstalled.
	result = &fluxcdv1alpha1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestFluxInstanceReconciler_InstallFail(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler()
	reconciler.StoragePath = "notfound"
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1alpha1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ns.Name,
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(),
	}

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Try to install the instance.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the instance was marked as failed.
	result := &fluxcdv1alpha1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	logObjectStatus(t, result)
	g.Expect(conditions.IsStalled(result)).To(BeTrue())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.BuildFailedReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring(reconciler.StoragePath))

	// Check if events were recorded for each step.
	events := getEvents(result.Name)
	g.Expect(events).To(HaveLen(1))
	g.Expect(events[0].Reason).To(Equal(meta.BuildFailedReason))
	g.Expect(events[0].Message).To(ContainSubstring(reconciler.StoragePath))

	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the instance was uninstalled.
	result = &fluxcdv1alpha1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

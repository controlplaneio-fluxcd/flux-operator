// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	kcheck "github.com/fluxcd/pkg/runtime/conditions/check"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestFluxInstanceReconciler_CELNameValidation(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-name",
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(t),
	}

	err = testEnv.Create(ctx, obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("the only accepted name for a FluxInstance is 'flux'"))
}

func TestFluxInstanceReconciler_InitDisabled(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
			Annotations: map[string]string{
				fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
			},
		},
		Spec: getDefaultFluxSpec(t),
	}

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	resultInit := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultInit)
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the finalizer was added.
	g.Expect(resultInit.Finalizers).To(ContainElement(fluxcdv1.Finalizer))

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the Ready condition is set to ReconciliationDisabled.
	checkInstanceReadiness(g, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(fluxcdv1.ReconciliationDisabledReason))
}

func TestFluxInstanceReconciler_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	const manifestsURL = "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest"
	reconciler := getFluxInstanceReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(t),
	}
	obj.Spec.Distribution.Artifact = manifestsURL

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Check if the finalizer was added.
	resultInit := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultInit)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultInit)
	g.Expect(resultInit.Finalizers).To(ContainElement(fluxcdv1.Finalizer))

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the instance was installed.
	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	checkInstanceReadiness(g, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check artifact digest.
	lastArtifactRevision := result.Status.LastArtifactRevision
	g.Expect(lastArtifactRevision).To(HavePrefix("sha256:"))
	g.Expect(strings.TrimPrefix(lastArtifactRevision, "sha256:")).To(HaveLen(64))

	// Check if the inventory was updated.
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_source-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_kustomize-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_helm-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_notification-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_allow-egress_networking.k8s.io_NetworkPolicy", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("_cluster-reconciler-%s_rbac.authorization.k8s.io_ClusterRoleBinding", ns.Name),
			Version: "v1",
		},
	))

	// Check if components images were recorded.
	g.Expect(result.Status.Components).To(HaveLen(4))
	g.Expect(result.Status.Components[0].Repository).To(Equal("ghcr.io/fluxcd/source-controller"))
	g.Expect(result.Status.Components[1].Repository).To(Equal("ghcr.io/fluxcd/kustomize-controller"))
	g.Expect(result.Status.Components[2].Repository).To(Equal("ghcr.io/fluxcd/helm-controller"))
	g.Expect(result.Status.Components[3].Repository).To(Equal("ghcr.io/fluxcd/notification-controller"))

	// Check if the deployments images have digests.
	sc := &appsv1.Deployment{}
	err = testClient.Get(ctx, types.NamespacedName{Name: "source-controller", Namespace: ns.Name}, sc)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sc.Spec.Template.Spec.Containers[0].Image).To(HavePrefix("ghcr.io/fluxcd/source-controller"))
	g.Expect(sc.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring("@sha256:"))

	// Check if the deployments have the correct labels.
	g.Expect(sc.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "flux"))

	// Update the instance.
	resultP := result.DeepCopy()
	resultP.SetAnnotations(map[string]string{
		fluxcdv1.ReconcileAnnotation:      fluxcdv1.EnabledValue,
		fluxcdv1.ReconcileEveryAnnotation: "1m",
	})
	resultP.Spec.Distribution.Registry = "docker.io/fluxcd"
	resultP.Spec.Components = []fluxcdv1.Component{"source-controller", "kustomize-controller"}
	resultP.Spec.Cluster = &fluxcdv1.Cluster{
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
	resultFinal := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFinal)
	g.Expect(resultFinal.Status.LastAttemptedRevision).To(HavePrefix("v2.3.0@sha256:"))
	g.Expect(resultFinal.Status.LastAppliedRevision).To(BeIdenticalTo(resultFinal.Status.LastAttemptedRevision))

	// Check cluster default values.
	g.Expect(resultFinal.Spec.Cluster.Type).To(BeIdenticalTo("kubernetes"))
	g.Expect(resultFinal.Spec.Cluster.Domain).To(BeIdenticalTo("cluster.local"))
	g.Expect(resultFinal.Spec.Cluster.Multitenant).To(BeFalse())

	// Check if the inventory was updated.
	g.Expect(resultFinal.Status.Inventory.Entries).ToNot(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_helm-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_notification-controller_apps_Deployment", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_allow-egress_networking.k8s.io_NetworkPolicy", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%[1]s_%[1]s_source.toolkit.fluxcd.io_OCIRepository", ns.Name),
			Version: "v1beta2",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%[1]s_%[1]s_kustomize.toolkit.fluxcd.io_Kustomization", ns.Name),
			Version: "v1",
		},
	))

	// Check if components images were updated.
	g.Expect(resultFinal.Status.Components).To(HaveLen(2))
	g.Expect(resultFinal.Status.Components[0].Repository).To(Equal("docker.io/fluxcd/source-controller"))
	g.Expect(resultFinal.Status.Components[1].Repository).To(Equal("docker.io/fluxcd/kustomize-controller"))

	// Check if events were recorded for each step.
	events := getEvents(result.Name, result.Namespace)
	for _, event := range events {
		t.Log(event.Message)
	}
	messages := []string{
		"is outdated",
		"Installing",
		"installed",
		"Upgrading",
		"updated",
		"Reconciliation finished",
	}
	for _, message := range messages {
		g.Expect(events).Should(ContainElement(WithTransform(func(e corev1.Event) string { return e.Message }, ContainSubstring(message))))
	}

	// Check if events contain the revision metadata.
	g.Expect(events[len(events)-1].Annotations).To(HaveKeyWithValue(fluxcdv1.RevisionAnnotation, resultFinal.Status.LastAppliedRevision))

	// Uninstall the instance.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the instance was uninstalled.
	result = &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestFluxInstanceReconciler_FetchFail(t *testing.T) {
	g := NewWithT(t)
	const manifestsURL = "oci://not.found/artifact"
	reconciler := getFluxInstanceReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(t),
	}
	obj.Spec.Distribution.Artifact = manifestsURL

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
	g.Expect(err).To(HaveOccurred())

	// Check if the instance was marked as failed.
	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.IsStalled(result)).To(BeFalse())
	g.Expect(conditions.IsFalse(result, meta.ReadyCondition)).To(BeTrue())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ArtifactFailedReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring(manifestsURL))
	g.Expect(conditions.GetReason(result, meta.ReconcilingCondition)).To(BeIdenticalTo(meta.ProgressingWithRetryReason))

	// Check if events were recorded for each step.
	events := getEvents(result.Name, result.Namespace)
	g.Expect(events).To(HaveLen(1))
	g.Expect(events[0].Reason).To(Equal(meta.ArtifactFailedReason))

	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestFluxInstanceReconciler_BuildFail(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler(t)
	reconciler.StoragePath = "notfound"
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: getDefaultFluxSpec(t),
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
	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.IsStalled(result)).To(BeTrue())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.BuildFailedReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring(reconciler.StoragePath))

	// Check if events were recorded for each step.
	events := getEvents(result.Name, result.Namespace)
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
	result = &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestFluxInstanceReconciler_Downgrade(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler(t)
	spec := getDefaultFluxSpec(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: spec,
	}

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Install the instance.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the instance was installed.
	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	checkInstanceReadiness(g, result)

	// Try to downgrade.
	resultP := result.DeepCopy()
	resultP.Spec.Distribution.Version = "v2.2.x"
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check the final status.
	resultFinal := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the downgraded was rejected.
	testutils.LogObjectStatus(t, resultFinal)
	g.Expect(conditions.IsStalled(resultFinal)).To(BeTrue())
	g.Expect(conditions.GetMessage(resultFinal, meta.ReadyCondition)).To(ContainSubstring("is not supported"))

	// Uninstall the instance.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the instance was uninstalled.
	sc := &appsv1.Deployment{}
	err = testClient.Get(ctx, types.NamespacedName{Name: "source-controller", Namespace: ns.Name}, sc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestFluxInstanceReconciler_Disabled(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler(t)
	spec := getDefaultFluxSpec(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: spec,
	}

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Install the instance.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the instance was installed.
	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	checkInstanceReadiness(g, result)

	// Disable the instance reconciliation.
	resultP := result.DeepCopy()
	resultP.SetAnnotations(
		map[string]string{
			fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
		})
	resultP.Spec.Components = []fluxcdv1.Component{"source-controller"}
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the instance with disabled reconciliation.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check the final status.
	resultFinal := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the ReconciliationDisabled event was recorded.
	events := getEvents(result.Name, result.Namespace)
	g.Expect(events[len(events)-1].Reason).To(Equal("ReconciliationDisabled"))

	// Check that resources were not deleted.
	kc := &appsv1.Deployment{}
	err = testClient.Get(ctx, types.NamespacedName{Name: "kustomize-controller", Namespace: ns.Name}, kc)
	g.Expect(err).ToNot(HaveOccurred())

	// Enable the instance reconciliation.
	resultP = resultFinal.DeepCopy()
	resultP.SetAnnotations(
		map[string]string{
			fluxcdv1.ReconcileAnnotation: fluxcdv1.EnabledValue,
		})
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	// Uninstall the instance.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check that resources were not deleted.
	sc := &appsv1.Deployment{}
	err = testClient.Get(ctx, types.NamespacedName{Name: "source-controller", Namespace: ns.Name}, sc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestFluxInstanceReconciler_Profiles(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler(t)
	spec := getDefaultFluxSpec(t)
	spec.Distribution.Version = "v2.4.x"
	spec.Cluster = &fluxcdv1.Cluster{
		Type:        "openshift",
		Multitenant: true,
	}
	spec.Sharding = &fluxcdv1.Sharding{
		Shards: []string{"shard1", "shard2"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: spec,
	}

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Install the instance.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	sync := unstructured.Unstructured{}
	sync.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
	sync.SetKind("Kustomization")
	err = testClient.Get(ctx, types.NamespacedName{Name: ns.Name, Namespace: ns.Name}, &sync)
	g.Expect(err).ToNot(HaveOccurred())

	// Check multitenant profile.
	nestedString, b, err := unstructured.NestedString(sync.Object, "spec", "serviceAccountName")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b).To(BeTrue())
	g.Expect(nestedString).To(Equal("kustomize-controller"))

	// Check if the components were installed with the profiles.
	kc := &appsv1.Deployment{}
	err = testClient.Get(ctx, types.NamespacedName{Name: "kustomize-controller", Namespace: ns.Name}, kc)
	g.Expect(err).ToNot(HaveOccurred())

	// Check multitenant profile.
	g.Expect(kc.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
		"--no-cross-namespace-refs=true",
		"--default-service-account=default",
		"--no-remote-bases=true",
	))

	// Check openshift profile.
	g.Expect(kc.Spec.Template.Spec.Containers[0].SecurityContext.SeccompProfile).To(BeNil())

	// Check custom patches.
	g.Expect(*kc.Spec.Replicas).To(BeNumerically("==", 0))

	// Check if the shards were installed.
	for _, shard := range spec.Sharding.Shards {
		sc := &appsv1.Deployment{}
		err = testClient.Get(ctx, types.NamespacedName{Name: "source-controller-" + shard, Namespace: ns.Name}, sc)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sc.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
			fmt.Sprintf("--watch-label-selector=sharding.fluxcd.io/key=%s", shard),
			fmt.Sprintf("--storage-adv-addr=source-controller-%s.$(RUNTIME_NAMESPACE).svc.cluster.local.", shard),
		))
	}

	// Check if the notification CRD was patched.
	crd := &unstructured.Unstructured{}
	crd.SetAPIVersion("apiextensions.k8s.io/v1")
	crd.SetKind("CustomResourceDefinition")
	err = testClient.Get(ctx, types.NamespacedName{Name: "alerts.notification.toolkit.fluxcd.io"}, crd)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(
		crd.Object["spec"].(map[string]any)["versions"].([]any)[2].(map[string]any)["schema"].(map[string]any)["openAPIV3Schema"].(map[string]any)["properties"].(map[string]any)["spec"].(map[string]any)["properties"].(map[string]any)["eventSources"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["kind"].(map[string]any)["enum"]).
		To(ContainElement("FluxInstance"))

	// Check if the receivers CRD was patched.
	rcrd := &unstructured.Unstructured{}
	rcrd.SetAPIVersion("apiextensions.k8s.io/v1")
	rcrd.SetKind("CustomResourceDefinition")
	err = testClient.Get(ctx, types.NamespacedName{Name: "receivers.notification.toolkit.fluxcd.io"}, rcrd)
	g.Expect(err).ToNot(HaveOccurred())
	rawData, err := rcrd.MarshalJSON()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(rawData)).To(ContainSubstring("FluxInstance"))
	g.Expect(string(rawData)).To(ContainSubstring("ResourceSetInputProvider"))

	// Uninstall the instance.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the instance was uninstalled.
	sc := &appsv1.Deployment{}
	err = testClient.Get(ctx, types.NamespacedName{Name: "source-controller", Namespace: ns.Name}, sc)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestFluxInstanceReconciler_NewVersion(t *testing.T) {
	g := NewWithT(t)
	reconciler := getFluxInstanceReconciler(t)
	spec := getDefaultFluxSpec(t)
	spec.Distribution.Version = "v2.2.x"

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: spec,
	}

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Install the instance.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if events were recorded for each step.
	events := getEvents(obj.Name, obj.Namespace)
	g.Expect(events).To(HaveLen(4))
	g.Expect(events[0].Reason).To(Equal("OutdatedVersion"))

	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

}

func getDefaultFluxSpec(t *testing.T) fluxcdv1.FluxInstanceSpec {
	// Disable notifications for the tests as no pod is running.
	// This is required to avoid the 30s retry loop performed by the HTTP client.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")

	return fluxcdv1.FluxInstanceSpec{
		Wait:             ptr.To(false),
		MigrateResources: ptr.To(true),
		Distribution: fluxcdv1.Distribution{
			Version:  "v2.3.0",
			Registry: "ghcr.io/fluxcd",
		},
		Sync: &fluxcdv1.Sync{
			Kind:     "OCIRepository",
			URL:      "oci://registry/repo",
			Path:     "./",
			Ref:      "latest",
			Provider: "generic",
		},
		CommonMetadata: &fluxcdv1.CommonMetadata{
			Labels: map[string]string{
				"app.kubernetes.io/name": "flux",
			},
		},
		Kustomize: &fluxcdv1.Kustomize{
			Patches: []kustomize.Patch{
				{
					Target: &kustomize.Selector{
						Kind: "Deployment",
					},
					Patch: `
- op: replace
  path: /spec/replicas
  value: 0
`,
				},
			},
		},
	}
}

func getFluxInstanceReconciler(t *testing.T) *FluxInstanceReconciler {
	tmpDir := t.TempDir()
	err := os.WriteFile(fmt.Sprintf("%s/kubeconfig", tmpDir), testKubeConfig, 0644)
	if err != nil {
		panic(fmt.Sprintf("failed to create the testenv-admin user kubeconfig: %v", err))
	}

	// Set the kubeconfig environment variable for the impersonator.
	t.Setenv("KUBECONFIG", fmt.Sprintf("%s/kubeconfig", tmpDir))

	return &FluxInstanceReconciler{
		Client:        testClient,
		Scheme:        NewTestScheme(),
		StoragePath:   filepath.Join("..", "..", "config", "data"),
		StatusManager: controllerName,
		EventRecorder: testEnv.GetEventRecorderFor(controllerName),
	}
}

func checkInstanceReadiness(g *WithT, obj *fluxcdv1.FluxInstance) {
	statusCheck := kcheck.NewInProgressChecker(testClient)
	statusCheck.DisableFetch = true
	statusCheck.WithT(g).CheckErr(context.Background(), obj)
	g.Expect(conditions.IsTrue(obj, meta.ReadyCondition)).To(BeTrue())
}

// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetInventoryObjects_StatusAndManifest(t *testing.T) {
	g := NewWithT(t)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-config", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}
	g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
	defer testClient.Delete(ctx, configMap)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-deploy", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "inv-deploy"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "inv-deploy"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}}},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	results := handler.GetInventoryObjects(ctx, []InventoryObjectItem{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "inv-config"},
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "inv-deploy"},
	}, false, nil)

	g.Expect(results).To(HaveLen(2))

	// Results keep request order and carry status + sanitized manifest.
	g.Expect(results[0].Kind).To(Equal("ConfigMap"))
	g.Expect(results[0].Error).To(BeEmpty())
	// A ConfigMap has no status, so kstatus reports it as Current (applied).
	g.Expect(results[0].Status).To(Equal("Current"))
	g.Expect(results[0].Object).NotTo(BeNil())

	g.Expect(results[1].Kind).To(Equal("Deployment"))
	g.Expect(results[1].Error).To(BeEmpty())
	g.Expect(results[1].Status).NotTo(BeEmpty())
	g.Expect(results[1].Object).NotTo(BeNil())

	// Manifest is sanitized: runtime metadata is stripped.
	meta := results[1].Object["metadata"].(map[string]any)
	g.Expect(meta).To(HaveKey("name"))
	g.Expect(meta).NotTo(HaveKey("managedFields"))
	g.Expect(meta).NotTo(HaveKey("uid"))
}

func TestGetInventoryObjects_StatusOnly(t *testing.T) {
	g := NewWithT(t)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-status-only", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}
	g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
	defer testClient.Delete(ctx, configMap)

	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	results := handler.GetInventoryObjects(ctx, []InventoryObjectItem{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "inv-status-only"},
	}, true, nil)

	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Error).To(BeEmpty())
	// Status is still computed, but the sanitized manifest is omitted.
	g.Expect(results[0].Status).To(Equal("Current"))
	g.Expect(results[0].Object).To(BeNil())
}

func TestGetInventoryObjects_ClusterScoped(t *testing.T) {
	g := NewWithT(t)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "inv-cluster-scoped"}}
	g.Expect(testClient.Create(ctx, ns)).To(Succeed())
	defer testClient.Delete(ctx, ns)

	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	results := handler.GetInventoryObjects(ctx, []InventoryObjectItem{
		{APIVersion: "v1", Kind: "Namespace", Name: "inv-cluster-scoped"},
	}, false, nil)

	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Error).To(BeEmpty())
	g.Expect(results[0].Object).NotTo(BeNil())
}

func TestGetInventoryObjects_NotFoundPerItem(t *testing.T) {
	g := NewWithT(t)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-present", Namespace: "default"},
	}
	g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
	defer testClient.Delete(ctx, configMap)

	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	results := handler.GetInventoryObjects(ctx, []InventoryObjectItem{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "inv-present"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "inv-missing"},
	}, false, nil)

	g.Expect(results).To(HaveLen(2))

	// The missing item reports its own error; the sibling still returns.
	g.Expect(results[0].Error).To(BeEmpty())
	g.Expect(results[0].Object).NotTo(BeNil())
	g.Expect(results[1].Error).To(Equal("NotFound"))
	g.Expect(results[1].Object).To(BeNil())
}

func TestGetInventoryObjects_ForbiddenPerItem(t *testing.T) {
	g := NewWithT(t)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "inv-forbidden", Namespace: "default"},
	}
	g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
	defer testClient.Delete(ctx, configMap)

	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	imp := user.Impersonation{Username: "inv-unprivileged", Groups: []string{"unprivileged-group"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	results := handler.GetInventoryObjects(userCtx, []InventoryObjectItem{
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "inv-forbidden"},
	}, false, nil)

	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Error).To(Equal("Forbidden"))
	g.Expect(results[0].Object).To(BeNil())
}

func TestComputeObjectStatus(t *testing.T) {
	g := NewWithT(t)

	// Flux kind with Ready=True → NewResourceStatus reports Ready.
	fluxObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": "apps", "namespace": "flux-system"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "message": "Applied revision"},
			},
		},
	}}
	st, msg := computeObjectStatus(ctx, fluxObj, nil)
	g.Expect(st).To(Equal(reporter.StatusReady))
	g.Expect(msg).To(Equal("Applied revision"))

	// Non-Flux object with no status → kstatus reports Current.
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cfg", "namespace": "default"},
	}}
	st, _ = computeObjectStatus(ctx, cm, nil)
	g.Expect(st).To(Equal(string(status.CurrentStatus)))

	// CronJob → workload logic reports Idle with the schedule (not raw kstatus).
	cronJob := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]any{"name": "backup", "namespace": "default"},
		"spec":       map[string]any{"schedule": "0 0 * * *"},
	}}
	st, msg = computeObjectStatus(ctx, cronJob, nil)
	g.Expect(st).To(Equal("Idle"))
	g.Expect(msg).To(Equal("0 0 * * *"))

	// Suspended CronJob → Suspended.
	suspendedCronJob := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]any{"name": "backup", "namespace": "default"},
		"spec":       map[string]any{"schedule": "0 0 * * *", "suspend": true},
	}}
	st, _ = computeObjectStatus(ctx, suspendedCronJob, nil)
	g.Expect(st).To(Equal("Suspended"))
}

func TestInventoryObjectsHandler(t *testing.T) {
	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	t.Run("rejects non-POST methods", func(t *testing.T) {
		g := NewWithT(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/objects", nil)
		rec := httptest.NewRecorder()
		handler.InventoryObjectsHandler(rec, req)
		g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	t.Run("rejects an invalid body", func(t *testing.T) {
		g := NewWithT(t)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/objects", strings.NewReader("not json"))
		rec := httptest.NewRecorder()
		handler.InventoryObjectsHandler(rec, req)
		g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	})

	t.Run("returns the objects list", func(t *testing.T) {
		g := NewWithT(t)

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "inv-handler", Namespace: "default"},
		}
		g.Expect(testClient.Create(ctx, configMap)).To(Succeed())
		defer testClient.Delete(ctx, configMap)

		body := `{"objects":[{"apiVersion":"v1","kind":"ConfigMap","namespace":"default","name":"inv-handler"}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/objects", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.InventoryObjectsHandler(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusOK))

		var resp struct {
			Objects []InventoryObjectResult `json:"objects"`
		}
		g.Expect(json.Unmarshal(rec.Body.Bytes(), &resp)).To(Succeed())
		g.Expect(resp.Objects).To(HaveLen(1))
		g.Expect(resp.Objects[0].Name).To(Equal("inv-handler"))
		g.Expect(resp.Objects[0].Object).NotTo(BeNil())
	})
}

// newTestOwner returns an unstructured Flux object of the given kind declaring
// the provided spec.healthCheckExprs entries.
func newTestOwner(apiVersion, kind string, exprs ...map[string]any) *unstructured.Unstructured {
	raw := make([]any, 0, len(exprs))
	for _, e := range exprs {
		raw = append(raw, e)
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": "apps", "namespace": "flux-system"},
		"spec":       map[string]any{"healthCheckExprs": raw},
	}}
}

// newTestCustomResource returns an unstructured custom resource of the given
// kind with the provided status fields.
func newTestCustomResource(kind string, generation int64, objStatus map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": "demo", "namespace": "default", "generation": generation},
		"status":     objStatus,
	}}
}

func TestNewHealthCheckEvaluators(t *testing.T) {
	phaseExprs := map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"inProgress": "status.phase == 'Pending'",
		"failed":     "status.phase == 'Error'",
		"current":    "status.phase == 'Ready'",
	}

	t.Run("returns nil without expressions", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization"))).To(BeNil())

		noSpec := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata":   map[string]any{"name": "apps", "namespace": "flux-system"},
		}}
		g.Expect(newHealthCheckEvaluators(ctx, noSpec)).To(BeNil())
	})

	t.Run("compiles Kustomization and HelmRelease expressions", func(t *testing.T) {
		g := NewWithT(t)
		gk := schema.GroupKind{Group: "example.io", Kind: "Widget"}

		ks := newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization", phaseExprs))
		g.Expect(ks).To(HaveLen(1))
		g.Expect(ks.lookup(gk)).NotTo(BeNil())

		hr := newHealthCheckEvaluators(ctx, newTestOwner("helm.toolkit.fluxcd.io/v2", "HelmRelease", phaseExprs))
		g.Expect(hr).To(HaveLen(1))
		g.Expect(hr.lookup(gk)).NotTo(BeNil())
	})

	t.Run("matches a whole group when kind is omitted", func(t *testing.T) {
		g := NewWithT(t)
		groupExprs := map[string]any{
			"apiVersion": "example.io/v1",
			"current":    "status.phase == 'Ready'",
		}
		evs := newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization", groupExprs))
		g.Expect(evs).To(HaveLen(1))
		g.Expect(evs.lookup(schema.GroupKind{Group: "example.io", Kind: "Widget"})).NotTo(BeNil())
		g.Expect(evs.lookup(schema.GroupKind{Group: "example.io", Kind: "Gadget"})).NotTo(BeNil())
		g.Expect(evs.lookup(schema.GroupKind{Group: "other.io", Kind: "Widget"})).To(BeNil())
	})

	t.Run("prefers the kind match over the group match", func(t *testing.T) {
		g := NewWithT(t)
		evs := newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization",
			map[string]any{"apiVersion": "example.io/v1", "current": "status.phase == 'Ready'"},
			map[string]any{"apiVersion": "example.io/v1", "kind": "Widget", "current": "status.phase == 'Done'"},
		))
		g.Expect(evs).To(HaveLen(2))

		obj := newTestCustomResource("Widget", 1, map[string]any{"phase": "Done"})
		st, _ := computeObjectStatus(ctx, obj, evs)
		g.Expect(st).To(Equal(string(status.CurrentStatus)))

		obj = newTestCustomResource("Gadget", 1, map[string]any{"phase": "Done"})
		st, _ = computeObjectStatus(ctx, obj, evs)
		g.Expect(st).To(Equal(string(status.InProgressStatus)))
	})

	t.Run("skips invalid entries", func(t *testing.T) {
		g := NewWithT(t)
		evs := newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization",
			map[string]any{"apiVersion": "example.io/v1", "kind": "Broken", "current": "status.phase =="},
			map[string]any{"apiVersion": "example.io/v1", "kind": "Missing"},
			map[string]any{"apiVersion": "example.io/v1", "kind": map[string]any{"not": "a string"}, "current": "true"},
			phaseExprs,
		))
		g.Expect(evs).To(HaveLen(1))
		g.Expect(evs.lookup(schema.GroupKind{Group: "example.io", Kind: "Broken"})).To(BeNil())
		g.Expect(evs.lookup(schema.GroupKind{Group: "example.io", Kind: "Missing"})).To(BeNil())
		g.Expect(evs.lookup(schema.GroupKind{Group: "example.io", Kind: "Widget"})).NotTo(BeNil())

		onlyBroken := newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization",
			map[string]any{"apiVersion": "example.io/v1", "kind": "Broken", "current": "status.phase =="},
		))
		g.Expect(onlyBroken).To(BeNil())
	})
}

func TestComputeObjectStatus_HealthCheckExprs(t *testing.T) {
	evs := newHealthCheckEvaluators(ctx, newTestOwner("kustomize.toolkit.fluxcd.io/v1", "Kustomization",
		map[string]any{
			"apiVersion": "example.io/v1",
			"kind":       "Widget",
			"inProgress": "status.phase == 'Pending'",
			"failed":     "status.phase == 'Error'",
			"current":    "status.phase in ['Ready', 'Paused']",
		},
		map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"current":    "has(status.readyReplicas) && status.readyReplicas > 0",
		},
	))

	t.Run("evaluates phase-based expressions", func(t *testing.T) {
		g := NewWithT(t)
		for phase, want := range map[string]string{
			"Ready":   string(status.CurrentStatus),
			"Paused":  string(status.CurrentStatus),
			"Pending": string(status.InProgressStatus),
			"Error":   string(status.FailedStatus),
			"Other":   string(status.InProgressStatus),
		} {
			obj := newTestCustomResource("Widget", 3, map[string]any{"phase": phase, "observedGeneration": int64(3)})
			st, msg := computeObjectStatus(ctx, obj, evs)
			g.Expect(st).To(Equal(want), "phase %s", phase)
			g.Expect(msg).To(Equal(want), "phase %s", phase)
		}
	})

	t.Run("accepts a string observedGeneration", func(t *testing.T) {
		g := NewWithT(t)
		obj := newTestCustomResource("Widget", 12, map[string]any{"phase": "Paused", "observedGeneration": "12"})

		// Without expressions kstatus cannot assess the object.
		st, msg := computeObjectStatus(ctx, obj, nil)
		g.Expect(st).To(Equal(reporter.StatusUnknown))
		g.Expect(msg).To(HavePrefix("Failed to compute status"))

		// With expressions the non-int64 field is skipped and the phase decides.
		st, msg = computeObjectStatus(ctx, obj, evs)
		g.Expect(st).To(Equal(string(status.CurrentStatus)))
		g.Expect(msg).To(Equal(string(status.CurrentStatus)))
	})

	t.Run("reports InProgress on a stale int64 observedGeneration", func(t *testing.T) {
		g := NewWithT(t)
		obj := newTestCustomResource("Widget", 5, map[string]any{"phase": "Ready", "observedGeneration": int64(4)})
		st, _ := computeObjectStatus(ctx, obj, evs)
		g.Expect(st).To(Equal(string(status.InProgressStatus)))
	})

	t.Run("takes precedence over the builtin workload logic", func(t *testing.T) {
		g := NewWithT(t)
		deploy := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "app", "namespace": "default", "generation": int64(1)},
			"status":     map[string]any{"observedGeneration": int64(1), "readyReplicas": int64(2)},
		}}
		st, msg := computeObjectStatus(ctx, deploy, evs)
		g.Expect(st).To(Equal(string(status.CurrentStatus)))
		g.Expect(msg).To(Equal(string(status.CurrentStatus)))

		// Kinds without expressions keep the builtin logic.
		cm := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "cfg", "namespace": "default"},
		}}
		st, _ = computeObjectStatus(ctx, cm, evs)
		g.Expect(st).To(Equal(string(status.CurrentStatus)))
	})

	t.Run("reports Unknown on an evaluation error", func(t *testing.T) {
		g := NewWithT(t)
		// The expression references a field the object does not have.
		obj := newTestCustomResource("Widget", 1, map[string]any{"ready": true})
		st, msg := computeObjectStatus(ctx, obj, evs)
		g.Expect(st).To(Equal(reporter.StatusUnknown))
		g.Expect(msg).To(HavePrefix("Failed to compute status"))
	})
}

func TestGetOwnerHealthCheckEvaluators(t *testing.T) {
	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	t.Run("returns nil without an owner", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(handler.getOwnerHealthCheckEvaluators(ctx, nil)).To(BeNil())
	})

	t.Run("returns nil for a non-Flux owner", func(t *testing.T) {
		g := NewWithT(t)
		owner := &InventoryObjectItem{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "app"}
		g.Expect(handler.getOwnerHealthCheckEvaluators(ctx, owner)).To(BeNil())
	})

	t.Run("returns nil for a Flux kind without health check expressions", func(t *testing.T) {
		g := NewWithT(t)
		owner := &InventoryObjectItem{APIVersion: "fluxcd.controlplane.io/v1", Kind: "ResourceSet", Namespace: "flux-system", Name: "apps"}
		g.Expect(handler.getOwnerHealthCheckEvaluators(ctx, owner)).To(BeNil())
	})

	t.Run("returns nil when the owner cannot be read", func(t *testing.T) {
		g := NewWithT(t)
		owner := &InventoryObjectItem{APIVersion: "kustomize.toolkit.fluxcd.io/v1", Kind: "Kustomization", Namespace: "flux-system", Name: "missing"}
		g.Expect(handler.getOwnerHealthCheckEvaluators(ctx, owner)).To(BeNil())
	})
}

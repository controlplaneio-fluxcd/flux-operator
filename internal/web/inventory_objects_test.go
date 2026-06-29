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
	})

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

func TestGetInventoryObjects_ClusterScoped(t *testing.T) {
	g := NewWithT(t)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "inv-cluster-scoped"}}
	g.Expect(testClient.Create(ctx, ns)).To(Succeed())
	defer testClient.Delete(ctx, ns)

	handler := &Handler{kubeClient: kubeClient, version: "v1.0.0", statusManager: "test", namespace: "flux-system"}

	results := handler.GetInventoryObjects(ctx, []InventoryObjectItem{
		{APIVersion: "v1", Kind: "Namespace", Name: "inv-cluster-scoped"},
	})

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
	})

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
	})

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
	st, msg := computeObjectStatus(fluxObj)
	g.Expect(st).To(Equal(reporter.StatusReady))
	g.Expect(msg).To(Equal("Applied revision"))

	// Non-Flux object with no status → kstatus reports Current.
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cfg", "namespace": "default"},
	}}
	st, _ = computeObjectStatus(cm)
	g.Expect(st).To(Equal(string(status.CurrentStatus)))

	// CronJob → workload logic reports Idle with the schedule (not raw kstatus).
	cronJob := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]any{"name": "backup", "namespace": "default"},
		"spec":       map[string]any{"schedule": "0 0 * * *"},
	}}
	st, msg = computeObjectStatus(cronJob)
	g.Expect(st).To(Equal("Idle"))
	g.Expect(msg).To(Equal("0 0 * * *"))

	// Suspended CronJob → Suspended.
	suspendedCronJob := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata":   map[string]any{"name": "backup", "namespace": "default"},
		"spec":       map[string]any{"schedule": "0 0 * * *", "suspend": true},
	}}
	st, _ = computeObjectStatus(suspendedCronJob)
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

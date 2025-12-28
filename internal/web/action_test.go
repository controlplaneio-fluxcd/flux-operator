// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestActionHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test with GET method (should fail)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/action", nil)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestActionHandler_InvalidJSON(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBufferString("invalid json"))
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Invalid request body"))
}

func TestActionHandler_MissingFields(t *testing.T) {
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name    string
		request ActionRequest
	}{
		{
			name:    "missing kind",
			request: ActionRequest{Namespace: "default", Name: "test", Action: "reconcile"},
		},
		{
			name:    "missing namespace",
			request: ActionRequest{Kind: "ResourceSet", Name: "test", Action: "reconcile"},
		},
		{
			name:    "missing name",
			request: ActionRequest{Kind: "ResourceSet", Namespace: "default", Action: "reconcile"},
		},
		{
			name:    "missing action",
			request: ActionRequest{Kind: "ResourceSet", Namespace: "default", Name: "test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			body, _ := json.Marshal(tc.request)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
			rec := httptest.NewRecorder()

			handler.ActionHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required fields"))
		})
	}
}

func TestActionHandler_InvalidAction(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test",
		Action:    "invalid-action",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Invalid action"))
}

func TestActionHandler_UnknownKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "UnknownKind",
		Namespace: "default",
		Name:      "test",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Unknown resource kind"))
}

func TestActionHandler_NonReconcilableKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Alert is not reconcilable
	actionReq := ActionRequest{
		Kind:      "Alert",
		Namespace: "default",
		Name:      "test",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("does not support actions"))
}

func TestActionHandler_Reconcile_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-reconcile",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-reconcile",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp ActionResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
	g.Expect(resp.Message).To(ContainSubstring("Reconciliation triggered"))

	// Verify annotation was set
	var updated fluxcdv1.ResourceSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), &updated)).To(Succeed())
	g.Expect(updated.Annotations).To(HaveKey("reconcile.fluxcd.io/requestedAt"))
}

func TestActionHandler_Suspend_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-suspend",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-suspend",
		Action:    "suspend",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp ActionResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
	g.Expect(resp.Message).To(ContainSubstring("Suspended"))

	// Verify suspend annotation was set (Flux Operator resources use annotations)
	var updated fluxcdv1.ResourceSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), &updated)).To(Succeed())
	g.Expect(updated.Annotations).To(HaveKeyWithValue(fluxcdv1.ReconcileAnnotation, fluxcdv1.DisabledValue))
}

func TestActionHandler_Resume_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a suspended ResourceSet for testing (using annotation)
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-resume",
			Namespace: "default",
			Annotations: map[string]string{
				fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
			},
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-resume",
		Action:    "resume",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp ActionResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
	g.Expect(resp.Message).To(ContainSubstring("Resumed"))

	// Verify resume annotation was set (Flux Operator resources use annotations)
	var updated fluxcdv1.ResourceSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(resourceSet), &updated)).To(Succeed())
	g.Expect(updated.Annotations).To(HaveKeyWithValue(fluxcdv1.ReconcileAnnotation, fluxcdv1.EnabledValue))
	g.Expect(updated.Annotations).To(HaveKey("reconcile.fluxcd.io/requestedAt"))
}

func TestActionHandler_ResourceNotFound(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "non-existent-resource",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusNotFound))
	g.Expect(rec.Body.String()).To(ContainSubstring("not found"))
}

func TestActionHandler_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-forbidden",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-action-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-forbidden",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestActionHandler_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-rbac-success",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user to patch resourcesets
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-action-resourceset-patcher",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list", "patch"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	defer testClient.Delete(ctx, clusterRole)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-action-resourceset-patcher-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "action-patcher-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with patch access
	imp := user.Impersonation{
		Username: "action-patcher-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Action Patcher User"},
		Impersonation: imp,
	}, userClient)

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-rbac-success",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp ActionResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
}

func TestActionHandler_NamespaceScopedRBAC_ForbiddenInOtherNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet in flux-system namespace
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-ns-scoped",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user with access only in a different namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-ns-patcher",
			Namespace: "kube-system", // Different namespace
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list", "patch"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-ns-patcher-binding",
			Namespace: "kube-system",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "action-ns-scoped-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with namespace-scoped access
	imp := user.Impersonation{
		Username: "action-ns-scoped-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Scoped User"},
		Impersonation: imp,
	}, userClient)

	// Try to reconcile in default namespace (user only has access to kube-system)
	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-ns-scoped",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestActionHandler_ResponseContentType(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-content-type",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-content-type",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
}

func TestActionHandler_AllValidActions(t *testing.T) {
	validActions := []string{"reconcile", "suspend", "resume"}

	for _, action := range validActions {
		t.Run(action, func(t *testing.T) {
			g := NewWithT(t)

			// Create annotations - start suspended if testing resume
			var annotations map[string]string
			if action == "resume" {
				annotations = map[string]string{
					fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
				}
			}

			// Create a ResourceSet for testing
			resourceSet := &fluxcdv1.ResourceSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-action-" + action,
					Namespace:   "default",
					Annotations: annotations,
				},
				Spec: fluxcdv1.ResourceSetSpec{},
			}
			g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
			defer testClient.Delete(ctx, resourceSet)

			handler := &Handler{
				kubeClient:    kubeClient,
				version:       "v1.0.0",
				statusManager: "test-status-manager",
				namespace:     "flux-system",
			}

			actionReq := ActionRequest{
				Kind:      "ResourceSet",
				Namespace: "default",
				Name:      "test-action-" + action,
				Action:    action,
			}
			body, _ := json.Marshal(actionReq)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/action", bytes.NewBuffer(body))
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.ActionHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusOK))

			var resp ActionResponse
			respBody, _ := io.ReadAll(rec.Body)
			err := json.Unmarshal(respBody, &resp)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Success).To(BeTrue())
		})
	}
}

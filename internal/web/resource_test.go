// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetResource_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing (FluxInstance has name validation)
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetResource without any user session (privileged)
	resource, err := handler.GetResource(ctx, "ResourceSet", "test-resourceset", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resource).NotTo(BeNil())
	g.Expect(resource.GetKind()).To(Equal(fluxcdv1.ResourceSetKind))
	g.Expect(resource.GetName()).To(Equal("test-resourceset"))
	g.Expect(resource.GetNamespace()).To(Equal("default"))
}

func TestGetResource_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset-forbidden",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-resource-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResource with the unprivileged user context
	// Should fail with forbidden error because GetResource respects RBAC
	_, err = handler.GetResource(userCtx, "ResourceSet", "test-resourceset-forbidden", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error, got: %v", err)
}

func TestGetResource_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset-rbac",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user to access resourcesets
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resource-resourceset-reader",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	defer testClient.Delete(ctx, clusterRole)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resource-resourceset-reader-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "resource-user-with-access",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with resourcesets access
	imp := user.Impersonation{
		Username: "resource-user-with-access",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Resource User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResource with the user context - should succeed
	resource, err := handler.GetResource(userCtx, "ResourceSet", "test-resourceset-rbac", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resource).NotTo(BeNil())
	g.Expect(resource.GetKind()).To(Equal(fluxcdv1.ResourceSetKind))
	g.Expect(resource.GetName()).To(Equal("test-resourceset-rbac"))
}

func TestGetResource_WithGroupRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset-group-rbac",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test group to access resourcesets
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resource-group-resourceset-reader",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	defer testClient.Delete(ctx, clusterRole)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resource-group-resourceset-reader-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "flux-readers",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with group membership
	imp := user.Impersonation{
		Username: "resource-group-user",
		Groups:   []string{"flux-readers"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Group User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResource with the user context - should succeed via group RBAC
	resource, err := handler.GetResource(userCtx, "ResourceSet", "test-resourceset-group-rbac", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resource).NotTo(BeNil())
	g.Expect(resource.GetKind()).To(Equal(fluxcdv1.ResourceSetKind))
	g.Expect(resource.GetName()).To(Equal("test-resourceset-group-rbac"))
}

func TestGetResource_WithNamespaceScopedRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing in default namespace
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset-ns-scoped",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user with access only in the default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource-ns-resourceset-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource-ns-resourceset-reader-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "resource-ns-scoped-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with namespace-scoped access
	imp := user.Impersonation{
		Username: "resource-ns-scoped-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Scoped User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResource in default namespace - should succeed
	resource, err := handler.GetResource(userCtx, "ResourceSet", "test-resourceset-ns-scoped", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resource).NotTo(BeNil())
	g.Expect(resource.GetKind()).To(Equal(fluxcdv1.ResourceSetKind))
	g.Expect(resource.GetName()).To(Equal("test-resourceset-ns-scoped"))
}

func TestGetResource_WithNamespaceScopedRBAC_ForbiddenInOtherNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create the flux-system namespace for this test
	fluxSystemNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}
	g.Expect(testClient.Create(ctx, fluxSystemNS)).To(Succeed())
	defer testClient.Delete(ctx, fluxSystemNS)

	// Create a ResourceSet in flux-system namespace
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset-other-ns",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user with access only in the default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource-default-only-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource-default-only-reader-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "resource-default-only-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with access only in default namespace
	imp := user.Impersonation{
		Username: "resource-default-only-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Default Only User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResource in flux-system namespace - should be forbidden
	_, err = handler.GetResource(userCtx, "ResourceSet", "test-resourceset-other-ns", "flux-system")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error when accessing resource in unauthorized namespace, got: %v", err)
}

func TestGetResource_NotFound(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetResource for a non-existent resource
	_, err := handler.GetResource(ctx, "ResourceSet", "non-existent-resourceset", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected not found error, got: %v", err)
}

func TestGetResource_InvalidKind(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetResource with an invalid kind
	_, err := handler.GetResource(ctx, "InvalidKind", "test", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unable to find Flux kind"))
}

// Tests for getReconcilerRef function

func TestGetReconcilerRef_KustomizationLabels(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "default",
				"labels": map[string]any{
					"kustomize.toolkit.fluxcd.io/name":      "my-kustomization",
					"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
				},
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(Equal("Kustomization/flux-system/my-kustomization"))
}

func TestGetReconcilerRef_HelmReleaseLabels(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "default",
				"labels": map[string]any{
					"helm.toolkit.fluxcd.io/name":      "my-helmrelease",
					"helm.toolkit.fluxcd.io/namespace": "flux-system",
				},
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(Equal("HelmRelease/flux-system/my-helmrelease"))
}

func TestGetReconcilerRef_ResourceSetLabels(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "default",
				"labels": map[string]any{
					"resourceset.fluxcd.controlplane.io/name":      "my-resourceset",
					"resourceset.fluxcd.controlplane.io/namespace": "flux-system",
				},
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(Equal("ResourceSet/flux-system/my-resourceset"))
}

func TestGetReconcilerRef_FluxOperatorLabels(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "source-controller",
				"namespace": "flux-system",
				"labels": map[string]any{
					"app.kubernetes.io/managed-by":     "flux-operator",
					"fluxcd.controlplane.io/name":      "flux",
					"fluxcd.controlplane.io/namespace": "flux-system",
				},
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(Equal("FluxInstance/flux-system/flux"))
}

func TestGetReconcilerRef_NoLabels(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "default",
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(BeEmpty())
}

func TestGetReconcilerRef_NonFluxLabels(t *testing.T) {
	g := NewWithT(t)

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "default",
				"labels": map[string]any{
					"app":     "my-app",
					"version": "v1",
				},
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(BeEmpty())
}

func TestGetReconcilerRef_PartialLabels(t *testing.T) {
	g := NewWithT(t)

	// Only has name, missing namespace
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-app",
				"namespace": "default",
				"labels": map[string]any{
					"kustomize.toolkit.fluxcd.io/name": "my-kustomization",
				},
			},
		},
	}

	result := getReconcilerRef(obj)
	g.Expect(result).To(BeEmpty())
}

// Tests for ResourceHandler HTTP handler

func TestResourceHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test with POST method (should fail)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()

	handler.ResourceHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestResourceHandler_MissingParameters(t *testing.T) {
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name  string
		query string
	}{
		{"missing all", ""},
		{"missing kind", "name=test&namespace=default"},
		{"missing name", "kind=ResourceSet&namespace=default"},
		{"missing namespace", "kind=ResourceSet&name=test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/resource?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.ResourceHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required parameters"))
		})
	}
}

func TestResourceHandler_HeadMethod(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-head-method",
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

	req := httptest.NewRequest(http.MethodHead, "/api/v1/resource?kind=ResourceSet&name=test-head-method&namespace=default", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ResourceHandler(rec, req)

	// HEAD should succeed (not return 405)
	g.Expect(rec.Code).NotTo(Equal(http.StatusMethodNotAllowed))
}

func TestResourceHandler_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-handler-success",
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource?kind=ResourceSet&name=test-handler-success&namespace=default", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ResourceHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

	// Verify response body is valid JSON with expected fields
	var result map[string]any
	err := json.NewDecoder(rec.Body).Decode(&result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result["kind"]).To(Equal("ResourceSet"))

	// Check metadata
	metadata, ok := result["metadata"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(metadata["name"]).To(Equal("test-handler-success"))
	g.Expect(metadata["namespace"]).To(Equal("default"))
}

func TestResourceHandler_NotFound_ReturnsEmptyJSON(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource?kind=ResourceSet&name=non-existent&namespace=default", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ResourceHandler(rec, req)

	// Not found returns 200 with empty JSON
	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
	g.Expect(rec.Body.String()).To(Equal("{}"))
}

func TestResourceHandler_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-handler-forbidden",
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

	// Create an unprivileged user session
	imp := user.Impersonation{
		Username: "unprivileged-handler-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource?kind=ResourceSet&name=test-handler-forbidden&namespace=default", nil)
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ResourceHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("do not have access"))
}

// Suppress unused import warning
var _ = bytes.Buffer{}

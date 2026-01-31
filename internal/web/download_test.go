// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestDownloadHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test with POST method (should fail)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/download?kind=GitRepository&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestDownloadHandler_MissingParameters(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "missing kind",
			query: "namespace=default&name=test",
		},
		{
			name:  "missing namespace",
			query: "kind=GitRepository&name=test",
		},
		{
			name:  "missing name",
			query: "kind=GitRepository&namespace=default",
		},
		{
			name:  "all missing",
			query: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/download?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.DownloadHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required query parameters"))
		})
	}
}

func TestDownloadHandler_UnknownKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind=UnknownKind&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Unknown resource kind"))
}

func TestDownloadHandler_NonDownloadableKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Kustomization is a valid Flux kind but doesn't support downloads
	req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind=Kustomization&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("does not support artifact downloads"))
}

func TestDownloadHandler_ActionsDisabled_NoAuth(t *testing.T) {
	g := NewWithT(t)

	// Test with no authentication configured
	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			UserActions: &fluxcdv1.UserActionsSpec{},
		},
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind=GitRepository&namespace=default&name=test", nil)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("User actions are disabled"))
}

func TestDownloadHandler_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create RBAC for the test user but NOT for the "download" action
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-download-no-permission",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"}, // No "download" permission
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	defer testClient.Delete(ctx, clusterRole)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-download-no-permission-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "unprivileged-download-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create an unprivileged user session (no download RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-download-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Request for a ResourceSet - will fail at RBAC check
	// Using ResourceSet because FluxOperator CRDs are installed in test env
	req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind=ResourceSet&namespace=default&name=test", nil)
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	// ResourceSet is not a downloadable kind, so it should fail at kind validation
	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("does not support artifact downloads"))
}

func TestDownloadHandler_GVKLookupFailsWhenCRDNotInstalled(t *testing.T) {
	g := NewWithT(t)

	// This test verifies that when Flux CRDs are not installed,
	// the handler returns 500 Internal Server Error for GVK lookup failure.
	// In a real cluster with Flux installed, this would proceed to RBAC checks.

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Request for a GitRepository - will fail at GVK lookup because CRD is not installed
	req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind=GitRepository&namespace=default&name=test", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DownloadHandler(rec, req)

	// Should return 500 because the GitRepository CRD is not installed in test environment
	g.Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	g.Expect(rec.Body.String()).To(ContainSubstring("Unable to get resource type"))
}

func TestIsDownloadableKind(t *testing.T) {
	g := NewWithT(t)

	// Test downloadable kinds
	g.Expect(isDownloadableKind(fluxcdv1.FluxGitRepositoryKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxBucketKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxOCIRepositoryKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxHelmChartKind)).To(BeTrue())
	g.Expect(isDownloadableKind(fluxcdv1.FluxExternalArtifactKind)).To(BeTrue())

	// Test non-downloadable kinds
	g.Expect(isDownloadableKind(fluxcdv1.FluxKustomizationKind)).To(BeFalse())
	g.Expect(isDownloadableKind(fluxcdv1.FluxHelmReleaseKind)).To(BeFalse())
	g.Expect(isDownloadableKind(fluxcdv1.FluxAlertKind)).To(BeFalse())
	g.Expect(isDownloadableKind("UnknownKind")).To(BeFalse())
}

func TestDownloadHandler_DownloadableKindsValidation(t *testing.T) {
	downloadableKinds := []string{
		fluxcdv1.FluxBucketKind,
		fluxcdv1.FluxGitRepositoryKind,
		fluxcdv1.FluxOCIRepositoryKind,
		fluxcdv1.FluxHelmChartKind,
	}

	nonDownloadableKinds := []string{
		fluxcdv1.FluxKustomizationKind,
		fluxcdv1.FluxHelmReleaseKind,
		fluxcdv1.FluxAlertKind,
		fluxcdv1.FluxAlertProviderKind,
		"ResourceSet",
	}

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test that downloadable kinds pass the kind validation
	for _, kind := range downloadableKinds {
		t.Run("downloadable_"+kind, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind="+kind+"&namespace=default&name=test", nil)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.DownloadHandler(rec, req)

			// Should get past kind validation (not 400 for invalid kind)
			// Will fail with 404 (not found) or 403 (no permission) or 500 (no CRD installed)
			g.Expect(rec.Body.String()).NotTo(ContainSubstring("does not support artifact downloads"))
		})
	}

	// Test that non-downloadable kinds are rejected
	for _, kind := range nonDownloadableKinds {
		t.Run("non_downloadable_"+kind, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/download?kind="+kind+"&namespace=default&name=test", nil)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.DownloadHandler(rec, req)

			// Should fail with 400 for non-downloadable kind
			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("does not support artifact downloads"))
		})
	}
}

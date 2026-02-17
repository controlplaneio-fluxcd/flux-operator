// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestHasWildcard(t *testing.T) {
	for _, tt := range []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "empty pattern",
			pattern:  "",
			expected: false,
		},
		{
			name:     "no wildcard",
			pattern:  "test",
			expected: false,
		},
		{
			name:     "single wildcard",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "wildcard at start",
			pattern:  "*test",
			expected: true,
		},
		{
			name:     "wildcard at end",
			pattern:  "test*",
			expected: true,
		},
		{
			name:     "wildcard in middle",
			pattern:  "te*st",
			expected: true,
		},
		{
			name:     "multiple wildcards",
			pattern:  "*test*",
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := hasWildcard(tt.pattern)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestMatchesWildcard(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    string
		pattern  string
		expected bool
	}{
		// Exact matching (no wildcards)
		{
			name:     "exact match",
			input:    "test",
			pattern:  "test",
			expected: true,
		},
		{
			name:     "exact match case insensitive",
			input:    "Test",
			pattern:  "test",
			expected: true,
		},
		{
			name:     "exact match uppercase",
			input:    "TEST",
			pattern:  "test",
			expected: true,
		},
		{
			name:     "exact no match",
			input:    "test",
			pattern:  "other",
			expected: false,
		},
		{
			name:     "empty pattern matches empty input",
			input:    "",
			pattern:  "",
			expected: true,
		},

		// Single wildcard
		{
			name:     "wildcard matches everything",
			input:    "anything",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "wildcard matches empty",
			input:    "",
			pattern:  "*",
			expected: true,
		},

		// Prefix matching
		{
			name:     "prefix match",
			input:    "test-service",
			pattern:  "test*",
			expected: true,
		},
		{
			name:     "prefix no match",
			input:    "other-service",
			pattern:  "test*",
			expected: false,
		},
		{
			name:     "prefix match case insensitive",
			input:    "Test-Service",
			pattern:  "test*",
			expected: true,
		},

		// Suffix matching
		{
			name:     "suffix match",
			input:    "my-test",
			pattern:  "*test",
			expected: true,
		},
		{
			name:     "suffix no match",
			input:    "my-other",
			pattern:  "*test",
			expected: false,
		},
		{
			name:     "suffix match case insensitive",
			input:    "my-Test",
			pattern:  "*test",
			expected: true,
		},

		// Contains matching
		{
			name:     "contains match",
			input:    "prefix-test-suffix",
			pattern:  "*test*",
			expected: true,
		},
		{
			name:     "contains no match",
			input:    "prefix-other-suffix",
			pattern:  "*test*",
			expected: false,
		},
		{
			name:     "contains match case insensitive",
			input:    "prefix-Test-suffix",
			pattern:  "*test*",
			expected: true,
		},

		// Middle wildcard
		{
			name:     "middle wildcard match",
			input:    "test-anything-service",
			pattern:  "test*service",
			expected: true,
		},
		{
			name:     "middle wildcard no match prefix",
			input:    "other-anything-service",
			pattern:  "test*service",
			expected: false,
		},
		{
			name:     "middle wildcard no match suffix",
			input:    "test-anything-other",
			pattern:  "test*service",
			expected: false,
		},
		{
			name:     "middle wildcard match case insensitive",
			input:    "Test-Anything-Service",
			pattern:  "test*service",
			expected: true,
		},

		// Multiple wildcards
		{
			name:     "multiple wildcards match",
			input:    "flux-test-my-service",
			pattern:  "*test*service*",
			expected: true,
		},
		{
			name:     "multiple wildcards no match",
			input:    "flux-other-my-deployment",
			pattern:  "*test*service*",
			expected: false,
		},
		{
			name:     "multiple wildcards adjacent",
			input:    "test",
			pattern:  "**test**",
			expected: true,
		},

		// Edge cases
		{
			name:     "wildcard with empty segments",
			input:    "test",
			pattern:  "***test***",
			expected: true,
		},
		{
			name:     "pattern longer than input",
			input:    "test",
			pattern:  "testservice",
			expected: false,
		},
		{
			name:     "input longer than pattern",
			input:    "testservice",
			pattern:  "test",
			expected: false,
		},
		{
			name:     "wildcard at start requires match at end",
			input:    "my-test-service",
			pattern:  "*test",
			expected: false,
		},
		{
			name:     "wildcard at end requires match at start",
			input:    "my-test-service",
			pattern:  "test*",
			expected: false,
		},
		{
			name:     "complex pattern match",
			input:    "flux-system-notification-controller",
			pattern:  "flux*notification*",
			expected: true,
		},
		{
			name:     "complex pattern no match",
			input:    "flux-system-source-controller",
			pattern:  "flux*notification*",
			expected: false,
		},

		// Real-world examples
		{
			name:     "flux resource prefix",
			input:    "flux-system",
			pattern:  "flux*",
			expected: true,
		},
		{
			name:     "controller suffix",
			input:    "kustomize-controller",
			pattern:  "*controller",
			expected: true,
		},
		{
			name:     "partial name search",
			input:    "my-app-deployment",
			pattern:  "*app*",
			expected: true,
		},
		{
			name:     "hyphenated names",
			input:    "flux-system-kustomize-controller",
			pattern:  "*kustomize*",
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := matchesWildcard(tt.input, tt.pattern)
			g.Expect(result).To(Equal(tt.expected), "matchesWildcard(%q, %q)", tt.input, tt.pattern)
		})
	}
}

func TestGetCachedResources_Privileged(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	handler.searchIndex.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-2", Kind: "HelmRelease", Namespace: "team-a", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
	})

	// Privileged context (no user session) = cluster-wide access
	resources := handler.GetCachedResources(ctx, "", "", "", "", 100)
	g.Expect(resources).To(HaveLen(2))
}

func TestGetCachedResources_UnprivilegedUser(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	handler.searchIndex.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-search-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	resources := handler.GetCachedResources(userCtx, "", "", "", "", 100)
	g.Expect(resources).To(BeEmpty(), "unprivileged user should get empty result")
}

func TestGetCachedResources_WithUserRBAC(t *testing.T) {
	g := NewWithT(t)

	// Create RBAC for the test user to access resources in default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cached-search-reader",
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
	defer func() { g.Expect(testClient.Delete(ctx, role)).To(Succeed()) }()

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cached-search-reader-binding",
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
				Name: "cached-search-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer func() { g.Expect(testClient.Delete(ctx, roleBinding)).To(Succeed()) }()

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	handler.searchIndex.Update([]reporter.ResourceStatus{
		{Name: "app-default", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-other", Kind: "Kustomization", Namespace: "other-ns", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	// Create a user session with namespace-scoped access
	imp := user.Impersonation{
		Username: "cached-search-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Cached Search User"},
		Impersonation: imp,
	}, userClient)

	resources := handler.GetCachedResources(userCtx, "", "", "", "", 100)
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("app-default"))
	g.Expect(resources[0].Namespace).To(Equal("default"))
}

func TestGetCachedResources_StatusFilter(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	handler.searchIndex.Update([]reporter.ResourceStatus{
		{Name: "app-ready", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-failed", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
		{Name: "app-suspended", Kind: "HelmRelease", Namespace: "default", Status: reporter.StatusSuspended, LastReconciled: metav1.Now()},
	})

	// Filter by status
	resources := handler.GetCachedResources(ctx, "", "", "", reporter.StatusFailed, 100)
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("app-failed"))
}

func TestSearchHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", nil)
	rec := httptest.NewRecorder()
	handler.SearchHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
}

func TestSearchHandler_WildcardExpansion(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	handler.searchIndex.Update([]reporter.ResourceStatus{
		{Name: "flux-system", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "my-flux-app", Kind: "HelmRelease", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "other-app", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	// Search with "flux" (no wildcard) — handler should wrap to "*flux*" for partial match
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?name=flux", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.SearchHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var response map[string]any
	g.Expect(json.Unmarshal(rec.Body.Bytes(), &response)).To(Succeed())

	resources, ok := response["resources"].([]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(resources).To(HaveLen(2), "should match both flux-system and my-flux-app")
}

func TestSearchHandler_MessageStripped(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	handler.searchIndex.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, Message: "Applied revision main@sha1:abc123", LastReconciled: metav1.Now()},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?name=app", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.SearchHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var response map[string]any
	g.Expect(json.Unmarshal(rec.Body.Bytes(), &response)).To(Succeed())

	resources := response["resources"].([]any)
	g.Expect(resources).To(HaveLen(1))

	resource := resources[0].(map[string]any)
	g.Expect(resource["message"]).To(Equal(""), "message should be stripped from search results")
}

func TestSearchHandler_EmptyIndex(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?name=anything", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.SearchHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var response map[string]any
	g.Expect(json.Unmarshal(rec.Body.Bytes(), &response)).To(Succeed())

	resources, ok := response["resources"].([]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(resources).To(BeEmpty(), "empty index should return empty array, not null")
}

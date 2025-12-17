// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetResourcesStatus_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-status",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetResourcesStatus without any user session (privileged)
	resources, err := router.GetResourcesStatus(ctx, "ResourceSet", "", "", "", 100)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resources).NotTo(BeNil())

	// Should find our test resource
	found := false
	for _, r := range resources {
		if r.Name == "test-resources-status" && r.Namespace == "default" {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "should find the test resource")
}

func TestGetResourcesStatus_UnprivilegedUser_EmptyResult(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-unprivileged",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-resources-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResourcesStatus with the unprivileged user context
	// Should return empty result (not error) because user has no namespace access
	resources, err := router.GetResourcesStatus(userCtx, "ResourceSet", "", "", "", 100)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resources).To(BeEmpty(), "unprivileged user should get empty result, not error")
}

func TestGetResourcesStatus_WithUserRBAC_OnlyAccessibleResources(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing in default namespace
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-rbac",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user to access resourcesets in default namespace only
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-status-reader",
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
			Name:      "test-resources-status-reader-binding",
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
				Name: "resources-status-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session with namespace-scoped access
	imp := user.Impersonation{
		Username: "resources-status-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Resources Status User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResourcesStatus with the user context
	resources, err := router.GetResourcesStatus(userCtx, "ResourceSet", "", "", "", 100)
	g.Expect(err).NotTo(HaveOccurred())

	// Should find our test resource in default namespace
	found := false
	for _, r := range resources {
		if r.Name == "test-resources-rbac" && r.Namespace == "default" {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "should find the test resource in accessible namespace")
}

func TestGetResourcesStatus_WithSpecificNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-specific-ns",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user to access resourcesets in default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-specific-ns-reader",
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
			Name:      "test-resources-specific-ns-reader-binding",
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
				Name: "resources-specific-ns-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session
	imp := user.Impersonation{
		Username: "resources-specific-ns-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Resources Specific NS User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResourcesStatus with specific namespace - should work
	resources, err := router.GetResourcesStatus(userCtx, "ResourceSet", "", "default", "", 100)
	g.Expect(err).NotTo(HaveOccurred())

	// Should find our test resource
	found := false
	for _, r := range resources {
		if r.Name == "test-resources-specific-ns" && r.Namespace == "default" {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "should find the test resource when querying specific namespace")
}

func TestGetResourcesStatus_IgnoresForbiddenErrors(t *testing.T) {
	g := NewWithT(t)

	// Create RBAC for the test user with access only to resourcesets (not other kinds)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resources-partial-access",
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
			Name:      "test-resources-partial-access-binding",
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
				Name: "resources-partial-access-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session
	imp := user.Impersonation{
		Username: "resources-partial-access-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Partial Access User"},
		Impersonation: imp,
	}, userClient)

	// Call GetResourcesStatus without specifying kind - will query multiple kinds
	// User only has access to resourcesets, should get forbidden for other kinds
	// but the function should NOT return an error, just return results for accessible resources
	resources, err := router.GetResourcesStatus(userCtx, "", "", "default", "", 100)
	g.Expect(err).NotTo(HaveOccurred(), "should not return error even when some kinds are forbidden")
	// Result can be empty (if no resources exist) but should not error
	g.Expect(resources).To(BeEmpty())
}

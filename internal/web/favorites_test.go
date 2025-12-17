// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetFavoritesStatus_Privileged_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-success",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetFavoritesStatus without any user session (privileged)
	favorites := []FavoriteItem{
		{Kind: "ResourceSet", Namespace: "default", Name: "test-fav-success"},
	}
	resources := router.GetFavoritesStatus(ctx, favorites)

	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("test-fav-success"))
	g.Expect(resources[0].Namespace).To(Equal("default"))
	g.Expect(resources[0].Kind).To(Equal("ResourceSet"))
	g.Expect(resources[0].Status).NotTo(Equal("NotFound"))
}

func TestGetFavoritesStatus_Privileged_NotFound(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetFavoritesStatus with a non-existent resource
	favorites := []FavoriteItem{
		{Kind: "ResourceSet", Namespace: "default", Name: "nonexistent-resourceset"},
	}
	resources := router.GetFavoritesStatus(ctx, favorites)

	// Should return result with NotFound status and user-friendly message
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("nonexistent-resourceset"))
	g.Expect(resources[0].Namespace).To(Equal("default"))
	g.Expect(resources[0].Kind).To(Equal("ResourceSet"))
	g.Expect(resources[0].Status).To(Equal("NotFound"))
	g.Expect(resources[0].Message).To(Equal("Resource not found in the cluster"))
}

func TestGetFavoritesStatus_Privileged_InvalidKind(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetFavoritesStatus with an invalid kind (unknown to Flux API)
	favorites := []FavoriteItem{
		{Kind: "InvalidKind", Namespace: "default", Name: "some-resource"},
	}
	resources := router.GetFavoritesStatus(ctx, favorites)

	// Should return result with NotFound status and internal error message
	// (unknown kinds are treated as internal errors since they don't map to any Flux group)
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("some-resource"))
	g.Expect(resources[0].Namespace).To(Equal("default"))
	g.Expect(resources[0].Kind).To(Equal("InvalidKind"))
	g.Expect(resources[0].Status).To(Equal("NotFound"))
	g.Expect(resources[0].Message).To(Equal("Internal error while fetching resource kind"))
}

func TestGetFavoritesStatus_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-forbidden",
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
		Username: "unprivileged-favorites-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetFavoritesStatus with the unprivileged user context
	favorites := []FavoriteItem{
		{Kind: "ResourceSet", Namespace: "default", Name: "test-fav-forbidden"},
	}
	resources := router.GetFavoritesStatus(userCtx, favorites)

	// Should return result with NotFound status and forbidden message
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("test-fav-forbidden"))
	g.Expect(resources[0].Status).To(Equal("NotFound"))
	g.Expect(resources[0].Message).To(Equal("User does not have access to the resource"))
}

func TestGetFavoritesStatus_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-rbac-success",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-favorites-reader",
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
			Name:      "test-favorites-reader-binding",
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
				Name: "favorites-reader-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session with RBAC access
	imp := user.Impersonation{
		Username: "favorites-reader-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Favorites Reader User"},
		Impersonation: imp,
	}, userClient)

	// Call GetFavoritesStatus with the user context
	favorites := []FavoriteItem{
		{Kind: "ResourceSet", Namespace: "default", Name: "test-fav-rbac-success"},
	}
	resources := router.GetFavoritesStatus(userCtx, favorites)

	// Should return the resource successfully
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("test-fav-rbac-success"))
	g.Expect(resources[0].Status).NotTo(Equal("NotFound"))
}

func TestGetFavoritesStatus_WithNamespaceScopedRBAC_ForbiddenInOtherNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create a unique namespace for this test
	otherNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fav-ns-test",
		},
	}
	g.Expect(testClient.Create(ctx, otherNS)).To(Succeed())
	defer testClient.Delete(ctx, otherNS)

	// Create a ResourceSet in the other namespace
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-other-ns",
			Namespace: "fav-ns-test",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user in default namespace only
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-ns-scoped-reader",
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
			Name:      "test-fav-ns-scoped-reader-binding",
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
				Name: "fav-ns-scoped-user",
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
		Username: "fav-ns-scoped-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Scoped User"},
		Impersonation: imp,
	}, userClient)

	// Call GetFavoritesStatus for resource in fav-ns-test (user only has access to default)
	favorites := []FavoriteItem{
		{Kind: "ResourceSet", Namespace: "fav-ns-test", Name: "test-fav-other-ns"},
	}
	resources := router.GetFavoritesStatus(userCtx, favorites)

	// Should return forbidden message
	g.Expect(resources).To(HaveLen(1))
	g.Expect(resources[0].Name).To(Equal("test-fav-other-ns"))
	g.Expect(resources[0].Status).To(Equal("NotFound"))
	g.Expect(resources[0].Message).To(Equal("User does not have access to the resource"))
}

func TestGetFavoritesStatus_MixedResults(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-mixed-exists",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create RBAC for the test user
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fav-mixed-reader",
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
			Name:      "test-fav-mixed-reader-binding",
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
				Name: "fav-mixed-user",
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
		Username: "fav-mixed-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Mixed User"},
		Impersonation: imp,
	}, userClient)

	// Call GetFavoritesStatus with mixed scenarios:
	// 1. Resource that exists and user has access
	// 2. Resource that doesn't exist
	// 3. Invalid kind
	favorites := []FavoriteItem{
		{Kind: "ResourceSet", Namespace: "default", Name: "test-fav-mixed-exists"},
		{Kind: "ResourceSet", Namespace: "default", Name: "nonexistent"},
		{Kind: "InvalidKind", Namespace: "default", Name: "some-resource"},
	}
	resources := router.GetFavoritesStatus(userCtx, favorites)

	// Should return results for all 3 items with appropriate messages
	g.Expect(resources).To(HaveLen(3))

	// 1. Existing resource with access - should succeed
	g.Expect(resources[0].Name).To(Equal("test-fav-mixed-exists"))
	g.Expect(resources[0].Status).NotTo(Equal("NotFound"))

	// 2. Non-existent resource - should show not found message
	g.Expect(resources[1].Name).To(Equal("nonexistent"))
	g.Expect(resources[1].Status).To(Equal("NotFound"))
	g.Expect(resources[1].Message).To(Equal("Resource not found in the cluster"))

	// 3. Invalid kind - should show internal error message (unknown kinds don't map to Flux group)
	g.Expect(resources[2].Name).To(Equal("some-resource"))
	g.Expect(resources[2].Kind).To(Equal("InvalidKind"))
	g.Expect(resources[2].Status).To(Equal("NotFound"))
	g.Expect(resources[2].Message).To(Equal("Internal error while fetching resource kind"))
}

func TestGetFavoritesStatus_EmptyList(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetFavoritesStatus with empty list
	favorites := []FavoriteItem{}
	resources := router.GetFavoritesStatus(ctx, favorites)

	// Should return empty result
	g.Expect(resources).To(BeEmpty())
}

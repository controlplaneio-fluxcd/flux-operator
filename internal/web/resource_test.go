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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetResource without any user session (privileged)
	resource, err := router.GetResource(ctx, "ResourceSet", "test-resourceset", "default")
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

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

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
	_, err = router.GetResource(userCtx, "ResourceSet", "test-resourceset-forbidden", "default")
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

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

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
	resource, err := router.GetResource(userCtx, "ResourceSet", "test-resourceset-rbac", "default")
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

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

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
	resource, err := router.GetResource(userCtx, "ResourceSet", "test-resourceset-group-rbac", "default")
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

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

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
	resource, err := router.GetResource(userCtx, "ResourceSet", "test-resourceset-ns-scoped", "default")
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

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

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
	_, err = router.GetResource(userCtx, "ResourceSet", "test-resourceset-other-ns", "flux-system")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error when accessing resource in unauthorized namespace, got: %v", err)
}

func TestGetResource_NotFound(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetResource for a non-existent resource
	_, err := router.GetResource(ctx, "ResourceSet", "non-existent-resourceset", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected not found error, got: %v", err)
}

func TestGetResource_InvalidKind(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetResource with an invalid kind
	_, err := router.GetResource(ctx, "InvalidKind", "test", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unable to find Flux kind"))
}

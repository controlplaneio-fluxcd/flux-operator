// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeclient_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestNew(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(kubeClient).NotTo(BeNil())
}

func TestGetAPIReader_Privileged(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Without a user session in context, should return the privileged reader
	reader := kubeClient.GetAPIReader(ctx)
	g.Expect(reader).NotTo(BeNil())

	// Should be able to list namespaces
	var namespaces corev1.NamespaceList
	err = reader.List(ctx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetAPIReader_Unprivileged_Forbidden(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a user client with no RBAC permissions
	imp := user.Impersonation{
		Username: "unprivileged-reader-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged Reader User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's reader
	reader := kubeClient.GetAPIReader(userCtx)
	g.Expect(reader).NotTo(BeNil())

	// Should get forbidden error when trying to list namespaces
	var namespaces corev1.NamespaceList
	err = reader.List(userCtx, &namespaces)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error, got: %v", err)
}

func TestGetAPIReader_WithUserRBAC(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with namespace list access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-reader-ns-list",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding for the user
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-reader-ns-list-user-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "reader-with-rbac-user",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-reader-ns-list",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	// Create a user client with RBAC permissions via User
	imp := user.Impersonation{
		Username: "reader-with-rbac-user",
		Groups:   []string{},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Reader With RBAC User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's reader
	reader := kubeClient.GetAPIReader(userCtx)
	g.Expect(reader).NotTo(BeNil())

	// Should be able to list namespaces
	var namespaces corev1.NamespaceList
	err = reader.List(userCtx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetAPIReader_WithGroupRBAC(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with namespace list access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-reader-ns-list-group",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding for the group
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-reader-ns-list-group-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "reader-privileged-group",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-reader-ns-list-group",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	// Create a user client with RBAC permissions via Group
	imp := user.Impersonation{
		Username: "reader-some-user",
		Groups:   []string{"reader-privileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Reader Some User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's reader
	reader := kubeClient.GetAPIReader(userCtx)
	g.Expect(reader).NotTo(BeNil())

	// Should be able to list namespaces via group membership
	var namespaces corev1.NamespaceList
	err = reader.List(userCtx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetClient_Privileged(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Without a user session in context, should return the privileged client
	c := kubeClient.GetClient(ctx)
	g.Expect(c).NotTo(BeNil())

	// Should be able to list namespaces
	var namespaces corev1.NamespaceList
	err = c.List(ctx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetClient_Unprivileged_Forbidden(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a user client with no RBAC permissions
	imp := user.Impersonation{
		Username: "unprivileged-client-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged Client User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's client
	c := kubeClient.GetClient(userCtx)
	g.Expect(c).NotTo(BeNil())

	// Should get forbidden error when trying to list namespaces
	var namespaces corev1.NamespaceList
	err = c.List(userCtx, &namespaces)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error, got: %v", err)
}

func TestGetClient_WithUserRBAC(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with namespace list access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-client-ns-list",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding for the user
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-client-ns-list-user-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "client-with-rbac-user",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-client-ns-list",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	// Create a user client with RBAC permissions via User
	imp := user.Impersonation{
		Username: "client-with-rbac-user",
		Groups:   []string{},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Client With RBAC User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's client
	c := kubeClient.GetClient(userCtx)
	g.Expect(c).NotTo(BeNil())

	// Should be able to list namespaces
	var namespaces corev1.NamespaceList
	err = c.List(userCtx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetClient_WithGroupRBAC(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with namespace list access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-client-ns-list-group",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding for the group
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-client-ns-list-group-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "client-privileged-group",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-client-ns-list-group",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	// Create a user client with RBAC permissions via Group
	imp := user.Impersonation{
		Username: "client-some-user",
		Groups:   []string{"client-privileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Client Some User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's client
	c := kubeClient.GetClient(userCtx)
	g.Expect(c).NotTo(BeNil())

	// Should be able to list namespaces via group membership
	var namespaces corev1.NamespaceList
	err = c.List(userCtx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetConfig_Privileged(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Without a user session in context, should return the privileged config
	config := kubeClient.GetConfig(ctx)
	g.Expect(config).NotTo(BeNil())
	g.Expect(config.Impersonate.UserName).To(BeEmpty())
}

func TestGetConfig_Unprivileged_Forbidden(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a user client with no RBAC permissions
	imp := user.Impersonation{
		Username: "unprivileged-config-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged Config User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's config with impersonation set
	config := kubeClient.GetConfig(userCtx)
	g.Expect(config).NotTo(BeNil())
	g.Expect(config.Impersonate.UserName).To(Equal("unprivileged-config-user"))

	// Create a client using this config and verify it gets forbidden errors
	c, err := client.New(config, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())

	// Should get forbidden error when trying to list namespaces
	var namespaces corev1.NamespaceList
	err = c.List(userCtx, &namespaces)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error, got: %v", err)
}

func TestGetConfig_WithUserRBAC(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with namespace list access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config-ns-list",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding for the user
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config-ns-list-user-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "config-with-rbac-user",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-config-ns-list",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	// Create a user client with RBAC permissions via User
	imp := user.Impersonation{
		Username: "config-with-rbac-user",
		Groups:   []string{},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Config With RBAC User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's config with impersonation set
	config := kubeClient.GetConfig(userCtx)
	g.Expect(config).NotTo(BeNil())
	g.Expect(config.Impersonate.UserName).To(Equal("config-with-rbac-user"))

	// Create a client using this config and verify it can list namespaces
	c, err := client.New(config, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())

	// Should be able to list namespaces
	var namespaces corev1.NamespaceList
	err = c.List(userCtx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestGetConfig_WithGroupRBAC(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with namespace list access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config-ns-list-group",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding for the group
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config-ns-list-group-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "config-privileged-group",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-config-ns-list-group",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	// Create a user client with RBAC permissions via Group
	imp := user.Impersonation{
		Username: "config-some-user",
		Groups:   []string{"config-privileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Config Some User"},
		Impersonation: imp,
	}, userClient)

	// With user session in context, should return the user's config with impersonation set
	config := kubeClient.GetConfig(userCtx)
	g.Expect(config).NotTo(BeNil())
	g.Expect(config.Impersonate.UserName).To(Equal("config-some-user"))
	g.Expect(config.Impersonate.Groups).To(ContainElement("config-privileged-group"))

	// Create a client using this config and verify it can list namespaces via group membership
	c, err := client.New(config, client.Options{Scheme: testScheme})
	g.Expect(err).NotTo(HaveOccurred())

	// Should be able to list namespaces via group membership
	var namespaces corev1.NamespaceList
	err = c.List(userCtx, &namespaces)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces.Items).NotTo(BeEmpty())
}

func TestWithPrivileges_OverridesUserContext(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a user client and store in context
	imp := user.Impersonation{
		Username: "test-user",
		Groups:   []string{"test-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Test User"},
		Impersonation: imp,
	}, userClient)

	// Without WithPrivileges, should use user client
	userConfig := kubeClient.GetConfig(userCtx)
	g.Expect(userConfig.Impersonate.UserName).To(Equal("test-user"))
	g.Expect(userConfig.Impersonate.Groups).To(ContainElement("test-group"))

	// With WithPrivileges, should use privileged client
	privConfig := kubeClient.GetConfig(userCtx, kubeclient.WithPrivileges())
	g.Expect(privConfig.Impersonate.UserName).To(BeEmpty())
	g.Expect(privConfig.Impersonate.Groups).To(BeEmpty())
}

func TestGetUserClientFromCache(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	imp := user.Impersonation{
		Username: "cache-test-user",
		Groups:   []string{"cache-test-group"},
	}

	// First call should create a new user client
	userClient1, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(userClient1).NotTo(BeNil())

	// Second call should return the same cached client
	userClient2, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(userClient2).To(BeIdenticalTo(userClient1))
}

func TestGetUserClientFromCache_DifferentUsers(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	imp1 := user.Impersonation{
		Username: "user-1",
		Groups:   []string{"group-1"},
	}
	imp2 := user.Impersonation{
		Username: "user-2",
		Groups:   []string{"group-2"},
	}

	userClient1, err := kubeClient.GetUserClientFromCache(imp1)
	g.Expect(err).NotTo(HaveOccurred())

	userClient2, err := kubeClient.GetUserClientFromCache(imp2)
	g.Expect(err).NotTo(HaveOccurred())

	// Different users should have different clients
	g.Expect(userClient2).NotTo(BeIdenticalTo(userClient1))
}

func TestListUserNamespaces_Privileged(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Without a user session, should list all namespaces with full access
	namespaces, allNamespaces, err := kubeClient.ListUserNamespaces(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(namespaces).NotTo(BeEmpty())
	g.Expect(allNamespaces).To(BeTrue())

	// Namespaces should be sorted alphabetically
	for i := 1; i < len(namespaces); i++ {
		g.Expect(namespaces[i] >= namespaces[i-1]).To(BeTrue(), "namespaces should be sorted: %s should come after %s", namespaces[i], namespaces[i-1])
	}
}

func TestListUserNamespaces_Cached(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// First call
	namespaces1, allNamespaces1, err := kubeClient.ListUserNamespaces(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	// Second call should return cached results
	namespaces2, allNamespaces2, err := kubeClient.ListUserNamespaces(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(namespaces2).To(Equal(namespaces1))
	g.Expect(allNamespaces2).To(Equal(allNamespaces1))
}

func TestListUserNamespaces_WithUserSession(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create test namespace for RBAC testing
	testNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-user-ns",
		},
	}
	err = testClient.Create(ctx, testNs)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), testNs)
	})

	imp := user.Impersonation{
		Username: "ns-test-user",
		Groups:   []string{"ns-test-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Test User"},
		Impersonation: imp,
	}, userClient)

	// User without any RBAC should get empty namespace list
	namespaces, allNamespaces, err := kubeClient.ListUserNamespaces(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(allNamespaces).To(BeFalse())
	g.Expect(namespaces).To(BeEmpty())
}

func TestListUserNamespaces_WithClusterRoleBinding(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create ClusterRole with resourcesets access
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resourcesets-reader",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"fluxcd.controlplane.io"},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, clusterRole)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRole)
	})

	// Create ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-resourcesets-reader-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "cluster-wide-user",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "test-resourcesets-reader",
		},
	}
	err = testClient.Create(ctx, clusterRoleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), clusterRoleBinding)
	})

	imp := user.Impersonation{
		Username: "cluster-wide-user",
		Groups:   []string{},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Cluster Wide User"},
		Impersonation: imp,
	}, userClient)

	// User with cluster-wide access should get all namespaces
	namespaces, allNamespaces, err := kubeClient.ListUserNamespaces(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(allNamespaces).To(BeTrue())
	g.Expect(namespaces).NotTo(BeEmpty())
}

func TestListUserNamespaces_WithRoleBinding(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	// Create test namespace
	testNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rbac-ns",
		},
	}
	err = testClient.Create(ctx, testNs)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), testNs)
	})

	// Create Role in the test namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ns-resourcesets-reader",
			Namespace: "test-rbac-ns",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"fluxcd.controlplane.io"},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	err = testClient.Create(ctx, role)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), role)
	})

	// Create RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ns-resourcesets-reader-binding",
			Namespace: "test-rbac-ns",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "ns-scoped-user",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "test-ns-resourcesets-reader",
		},
	}
	err = testClient.Create(ctx, roleBinding)
	g.Expect(client.IgnoreAlreadyExists(err)).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = testClient.Delete(context.Background(), roleBinding)
	})

	imp := user.Impersonation{
		Username: "ns-scoped-user",
		Groups:   []string{},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Scoped User"},
		Impersonation: imp,
	}, userClient)

	// User with namespace-scoped access should get only that namespace
	namespaces, allNamespaces, err := kubeClient.ListUserNamespaces(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(allNamespaces).To(BeFalse())
	g.Expect(namespaces).To(ContainElement("test-rbac-ns"))
}

func TestUserClientImpersonation(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	imp := user.Impersonation{
		Username: "impersonate-test-user",
		Groups:   []string{"impersonate-test-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Impersonate Test User"},
		Impersonation: imp,
	}, userClient)

	// Get config and verify impersonation is set
	config := kubeClient.GetConfig(userCtx)
	g.Expect(config.Impersonate.UserName).To(Equal("impersonate-test-user"))
	g.Expect(config.Impersonate.Groups).To(ContainElement("impersonate-test-group"))
}

func TestUserClientCanCreateSSAR(t *testing.T) {
	g := NewWithT(t)

	kubeClient, err := kubeclient.New(testCluster, 100, 5*time.Minute)
	g.Expect(err).NotTo(HaveOccurred())

	imp := user.Impersonation{
		Username: "ssar-test-user",
		Groups:   []string{"ssar-test-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "SSAR Test User"},
		Impersonation: imp,
	}, userClient)

	// User should be able to create a SelfSubjectAccessReview
	c := kubeClient.GetClient(userCtx)
	ssar := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Verb:     "get",
				Group:    "fluxcd.controlplane.io",
				Resource: "resourcesets",
			},
		},
	}
	err = c.Create(userCtx, ssar)
	g.Expect(err).NotTo(HaveOccurred())
	// The result should indicate denied since the user has no RBAC permissions
	g.Expect(ssar.Status.Allowed).To(BeFalse())
}

func TestNamespaceCacheExpiration(t *testing.T) {
	g := NewWithT(t)

	// Use a very short cache duration for this test
	kubeClient, err := kubeclient.New(testCluster, 100, 100*time.Millisecond)
	g.Expect(err).NotTo(HaveOccurred())

	// First call
	namespaces1, _, err := kubeClient.ListUserNamespaces(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Second call should fetch fresh data (though results should be the same)
	namespaces2, _, err := kubeClient.ListUserNamespaces(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(namespaces2).To(Equal(namespaces1))
}

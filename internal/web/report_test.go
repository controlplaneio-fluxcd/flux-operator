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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestCleanObjectForExport(t *testing.T) {
	for _, tt := range []struct {
		name       string
		input      map[string]any
		keepStatus bool
		expected   map[string]any
	}{
		{
			name: "removes status when keepStatus is false",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"status": map[string]any{
					"phase": "Active",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "keeps status when keepStatus is true",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"status": map[string]any{
					"phase": "Active",
				},
			},
			keepStatus: true,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"status": map[string]any{
					"phase": "Active",
				},
			},
		},
		{
			name: "removes runtime metadata fields",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":              "test",
					"namespace":         "default",
					"uid":               "12345",
					"resourceVersion":   "67890",
					"generation":        int64(1),
					"creationTimestamp": "2025-01-01T00:00:00Z",
					"managedFields":     []any{},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "preserves labels and annotations",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app": "myapp",
						"env": "prod",
					},
					"annotations": map[string]any{
						"description": "test config",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app": "myapp",
						"env": "prod",
					},
					"annotations": map[string]any{
						"description": "test config",
					},
				},
			},
		},
		{
			name: "removes Flux ownership labels",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app":                                   "myapp",
						"kustomize.toolkit.fluxcd.io/name":      "flux-system",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
						"helm.toolkit.fluxcd.io/name":           "my-release",
						"helm.toolkit.fluxcd.io/namespace":      "default",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app": "myapp",
					},
				},
			},
		},
		{
			name: "removes kubectl and Flux CLI annotations",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"annotations": map[string]any{
						"description": "keep this",
						"kubectl.kubernetes.io/last-applied-configuration": "{}",
						"reconcile.fluxcd.io/requestedAt":                  "2025-01-01T00:00:00Z",
						"reconcile.fluxcd.io/forceAt":                      "2025-01-01T00:00:00Z",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"annotations": map[string]any{
						"description": "keep this",
					},
				},
			},
		},
		{
			name: "removes empty labels map after cleanup",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"kustomize.toolkit.fluxcd.io/name":      "flux-system",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "removes empty annotations map after cleanup",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"annotations": map[string]any{
						"kubectl.kubernetes.io/last-applied-configuration": "{}",
						"reconcile.fluxcd.io/requestedAt":                  "2025-01-01T00:00:00Z",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "handles object without namespace",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": "test",
					"uid":  "12345",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": "test",
				},
			},
		},
		{
			name: "handles object without labels and annotations",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "keeps non-Flux ownership labels",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app":                                   "myapp",
						"kustomize.toolkit.fluxcd.io/name":      "flux-system",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
						"kustomize.toolkit.fluxcd.io/prune":     "disabled",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app":                               "myapp",
						"kustomize.toolkit.fluxcd.io/prune": "disabled",
					},
				},
			},
		},
		{
			name: "complex real-world example",
			input: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":              "my-app",
					"namespace":         "production",
					"uid":               "abc-123",
					"resourceVersion":   "12345",
					"generation":        int64(5),
					"creationTimestamp": "2025-01-01T00:00:00Z",
					"labels": map[string]any{
						"app":                                   "my-app",
						"version":                               "v1.0.0",
						"kustomize.toolkit.fluxcd.io/name":      "apps",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
						"helm.toolkit.fluxcd.io/name":           "my-chart",
						"helm.toolkit.fluxcd.io/namespace":      "flux-system",
					},
					"annotations": map[string]any{
						"description": "My application",
						"kubectl.kubernetes.io/last-applied-configuration": "large-json-blob",
						"reconcile.fluxcd.io/requestedAt":                  "2025-01-01T00:00:00Z",
						"reconcile.fluxcd.io/forceAt":                      "2025-01-01T01:00:00Z",
						"custom.io/annotation":                             "keep-this",
					},
					"managedFields": []any{},
				},
				"spec": map[string]any{
					"replicas": int64(3),
				},
				"status": map[string]any{
					"availableReplicas": int64(3),
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "my-app",
					"namespace": "production",
					"labels": map[string]any{
						"app":     "my-app",
						"version": "v1.0.0",
					},
					"annotations": map[string]any{
						"description":          "My application",
						"custom.io/annotation": "keep-this",
					},
				},
				"spec": map[string]any{
					"replicas": int64(3),
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create unstructured object from input
			obj := &unstructured.Unstructured{Object: tt.input}

			// Call the function
			cleanObjectForExport(obj, tt.keepStatus)

			// Verify the result matches expected
			g.Expect(obj.Object).To(Equal(tt.expected))
		})
	}
}

func TestGetReport_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create the router with the test kubeclient
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetReport without any user session (privileged)
	report, err := router.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify basic report structure
	g.Expect(report.GetKind()).To(Equal(fluxcdv1.FluxReportKind))
	g.Expect(report.GetAPIVersion()).To(Equal(fluxcdv1.GroupVersion.String()))
}

func TestGetReport_WithUnprivilegedUser_ReportStillBuilt(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-report-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetReport with the unprivileged user context
	// The report should still be built successfully because it uses privileged access internally
	report, err := router.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify basic report structure
	g.Expect(report.GetKind()).To(Equal(fluxcdv1.FluxReportKind))
	g.Expect(report.GetAPIVersion()).To(Equal(fluxcdv1.GroupVersion.String()))

	// Verify user info is injected into the report
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("Unprivileged User"))

	// Verify namespaces is empty since the user has no access
	_, found = spec["namespaces"]
	g.Expect(found).To(BeFalse())
}

func TestGetReport_WithUserRBAC_NamespacesPopulated(t *testing.T) {
	g := NewWithT(t)

	// Create a test namespace for this test
	testNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-report-ns",
		},
	}
	g.Expect(testClient.Create(ctx, testNS)).To(Succeed())
	defer testClient.Delete(ctx, testNS)

	// Create RBAC for the test user to access resourcesets (which is used for namespace filtering)
	// ListUserNamespaces checks access to resourcesets.fluxcd.controlplane.io to determine
	// which namespaces the user can see
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-report-resourcesets-reader",
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
			Name: "test-report-resourcesets-reader-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "report-user-with-ns-access",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session with resourcesets access (which grants namespace visibility)
	imp := user.Impersonation{
		Username: "report-user-with-ns-access",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Report User"},
		Impersonation: imp,
	}, userClient)

	// Call GetReport with the user context
	report, err := router.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify namespaces are populated in the spec
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	// namespaces can be []string or []any depending on how they were serialized
	namespaces, found := spec["namespaces"]
	g.Expect(found).To(BeTrue(), "namespaces should be present in spec")
	switch ns := namespaces.(type) {
	case []string:
		g.Expect(ns).NotTo(BeEmpty())
	case []any:
		g.Expect(ns).NotTo(BeEmpty())
	default:
		t.Fatalf("unexpected namespaces type: %T", namespaces)
	}
}

func TestGetReport_WithUserRBAC_SingleNamespaceAccess(t *testing.T) {
	g := NewWithT(t)

	// Create RBAC for the test user with access only in the default namespace
	// Using a Role and RoleBinding (namespace-scoped) instead of ClusterRole/ClusterRoleBinding
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-report-single-ns-resourcesets-reader",
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
			Name:      "test-report-single-ns-resourcesets-reader-binding",
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
				Name: "report-user-single-ns-access",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session with access only in the default namespace
	imp := user.Impersonation{
		Username: "report-user-single-ns-access",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Single NS User"},
		Impersonation: imp,
	}, userClient)

	// Call GetReport with the user context
	report, err := router.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify the report is built successfully (uses privileged access)
	g.Expect(report.GetKind()).To(Equal(fluxcdv1.FluxReportKind))
	g.Expect(report.GetAPIVersion()).To(Equal(fluxcdv1.GroupVersion.String()))

	// Verify namespaces are populated with only the default namespace
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	namespaces, found := spec["namespaces"]
	g.Expect(found).To(BeTrue(), "namespaces should be present in spec")

	// User should only see the default namespace
	switch ns := namespaces.(type) {
	case []string:
		g.Expect(ns).To(ConsistOf("default"))
	case []any:
		g.Expect(ns).To(ConsistOf("default"))
	default:
		t.Fatalf("unexpected namespaces type: %T", namespaces)
	}

	// Verify user info is injected
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("Single NS User"))
}

func TestGetReport_InjectsUserInfo(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session with name set (this tests the case where Name is displayed)
	imp := user.Impersonation{
		Username: "info-test-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile: user.Profile{
			Name: "Info Test User",
		},
		Impersonation: imp,
	}, userClient)

	// Call GetReport
	report, err := router.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify user info is injected
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("Info Test User"))
}

func TestGetReport_InjectsUserInfoWithRole(t *testing.T) {
	g := NewWithT(t)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session without name but with groups (role comes from groups when name is empty)
	imp := user.Impersonation{
		Username: "info-test-user",
		Groups:   []string{"admin", "developers"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{}, // No name set - role will be returned from groups
		Impersonation: imp,
	}, userClient)

	// Call GetReport
	report, err := router.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify user info is injected - when name is empty, role is derived from groups
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("info-test-user"))
	g.Expect(userInfo["role"]).To(Equal("admin, developers"))
}

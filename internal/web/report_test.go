// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetReport_CachedAndPrivileged(t *testing.T) {
	g := NewWithT(t)

	// Create the handler with the test kubeclient
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
	}

	// Start report cache.
	ctx, cancel := context.WithCancel(ctx)
	stopped := handler.startReportCache(ctx, 5*time.Minute)
	defer func() {
		cancel()
		<-stopped
	}()

	// Call GetReport without any user session (privileged)
	report, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify basic report structure
	g.Expect(report.GetKind()).To(Equal(fluxcdv1.FluxReportKind))
	g.Expect(report.GetAPIVersion()).To(Equal(fluxcdv1.GroupVersion.String()))
}

func TestGetReport_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create the handler with the test kubeclient
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetReport without any user session (privileged)
	report, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify basic report structure
	g.Expect(report.GetKind()).To(Equal(fluxcdv1.FluxReportKind))
	g.Expect(report.GetAPIVersion()).To(Equal(fluxcdv1.GroupVersion.String()))
}

func TestGetReport_WithUnprivilegedUser_ReportStillBuilt(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

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
	report, err := handler.GetReport(userCtx)
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
	namespacesValue, found := spec["namespaces"]
	g.Expect(found).To(BeTrue())
	namespaces, ok := namespacesValue.([]string)
	g.Expect(ok).To(BeTrue())
	g.Expect(namespaces).To(BeEmpty())
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

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

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
	report, err := handler.GetReport(userCtx)
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

	// Create another namespace to test filtering
	otherNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "report-test-other-ns",
		},
	}
	g.Expect(testClient.Create(ctx, otherNS)).To(Succeed())
	defer testClient.Delete(ctx, otherNS)

	// Create ResourceSets in both namespaces to test filtering
	resourceSetDefault := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-report-rs-default",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{
			ResourcesTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\n",
		},
	}
	g.Expect(testClient.Create(ctx, resourceSetDefault)).To(Succeed())
	defer testClient.Delete(ctx, resourceSetDefault)

	resourceSetOther := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-report-rs-other",
			Namespace: "report-test-other-ns",
		},
		Spec: fluxcdv1.ResourceSetSpec{
			ResourcesTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\n",
		},
	}
	g.Expect(testClient.Create(ctx, resourceSetOther)).To(Succeed())
	defer testClient.Delete(ctx, resourceSetOther)

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

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

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
	report, err := handler.GetReport(userCtx)
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

	// Verify reconcilers stats are filtered to only show resources in accessible namespaces
	reconcilers, found := spec["reconcilers"]
	g.Expect(found).To(BeTrue(), "reconcilers should be present in spec")

	// Marshal and unmarshal to work with the data in a type-agnostic way
	reconcilersJSON, err := json.Marshal(reconcilers)
	g.Expect(err).NotTo(HaveOccurred())
	var reconcilersList []fluxcdv1.FluxReconcilerStatus
	g.Expect(json.Unmarshal(reconcilersJSON, &reconcilersList)).To(Succeed())

	// Find the ResourceSet entry and verify it only counts the one in "default" namespace
	for _, rec := range reconcilersList {
		if rec.Kind == fluxcdv1.ResourceSetKind {
			// Since user only has access to "default" namespace, they should only see
			// stats for the ResourceSet in "default" (running=1), not the one in "report-test-other-ns"
			g.Expect(rec.Stats.Running).To(Equal(1), "should only count ResourceSet in accessible namespace")
		}
	}
}

func TestGetReport_InjectsUserInfo(t *testing.T) {
	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	t.Run("at least one group", func(t *testing.T) {
		g := NewWithT(t)

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
		report, err := handler.GetReport(userCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(report).NotTo(BeNil())

		// Verify user info is injected
		spec, found := report.Object["spec"].(map[string]any)
		g.Expect(found).To(BeTrue())
		userInfo, found := spec["userInfo"].(map[string]any)
		g.Expect(found).To(BeTrue())
		g.Expect(userInfo["username"]).To(Equal("Info Test User"))
		g.Expect(userInfo["impersonation"]).To(Equal(user.Impersonation{
			Username: "info-test-user",
			Groups:   []string{"system:authenticated"},
		}))
	})

	t.Run("zero groups returns nil", func(t *testing.T) {
		g := NewWithT(t)

		// Create a user session with name set (this tests the case where Name is displayed)
		imp := user.Impersonation{
			Username: "info-test-user",
			Groups:   nil,
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
		report, err := handler.GetReport(userCtx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(report).NotTo(BeNil())

		// Verify user info is injected
		spec, found := report.Object["spec"].(map[string]any)
		g.Expect(found).To(BeTrue())
		userInfo, found := spec["userInfo"].(map[string]any)
		g.Expect(found).To(BeTrue())
		g.Expect(userInfo["username"]).To(Equal("Info Test User"))
		g.Expect(userInfo["impersonation"]).To(Equal(user.Impersonation{
			Username: "info-test-user",
			Groups:   nil,
		}))
		g.Expect(userInfo["impersonation"]).NotTo(Equal(user.Impersonation{
			Username: "info-test-user",
			Groups:   []string{},
		}))
	})
}

func TestGetReport_InjectsUserInfoWithRole(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

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
	report, err := handler.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify user info is injected - when name is empty, role is derived from groups
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("info-test-user"))
}

func TestGetReport_InjectsUserInfoWithProvider(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create provider details (simulating OIDC claims)
	providerDetails := map[string]any{
		"iss":   "https://accounts.example.com",
		"sub":   "1234567890",
		"email": "user@example.com",
		"name":  "Test User",
	}

	// Create a user session with provider details
	imp := user.Impersonation{
		Username: "provider-test-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Provider Test User"},
		Impersonation: imp,
		Provider:      providerDetails,
	}, userClient)

	// Call GetReport
	report, err := handler.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify provider info is injected in userInfo
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("Provider Test User"))
	g.Expect(userInfo["provider"]).To(Equal(providerDetails))
}

func TestGetReport_UserInfoProviderIsNilWhenNotSet(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session without provider details
	imp := user.Impersonation{
		Username: "no-provider-test-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "No Provider Test User"},
		Impersonation: imp,
		// Provider not set
	}, userClient)

	// Call GetReport
	report, err := handler.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify provider is nil in userInfo
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("No Provider Test User"))
	g.Expect(userInfo["provider"]).To(BeNil())
}

func TestGetReport_NoAuthConfigured_UserInfoOnlyContainsUsername(t *testing.T) {
	g := NewWithT(t)

	// Create the handler with the test kubeclient
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetReport with a plain context (no user session stored).
	// This simulates the case when authentication is not configured.
	report, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify the report structure
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())

	// Verify userInfo is present
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())

	// Verify username is present (will be "kubeconfig (dev)" or hostname when no auth)
	_, hasUsername := userInfo["username"]
	g.Expect(hasUsername).To(BeTrue(), "userInfo should contain 'username'")

	// Verify impersonation is NOT present (since auth is not configured, Permissions().IsEmpty() == true)
	_, hasImpersonation := userInfo["impersonation"]
	g.Expect(hasImpersonation).To(BeFalse(), "userInfo should NOT contain 'impersonation' when auth is not configured")

	// Verify provider is NOT present (since auth is not configured, Provider() returns nil)
	_, hasProvider := userInfo["provider"]
	g.Expect(hasProvider).To(BeFalse(), "userInfo should NOT contain 'provider' when auth is not configured")

	g.Expect(userInfo["userActionsEnabled"]).To(BeFalse(), "userActionsEnabled should be false when auth is not configured")
}

func TestGetReport_UserActionsEnabledWithAuth(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
			},
			UserActions: &fluxcdv1.UserActionsSpec{},
		},
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	report, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["userActionsEnabled"]).To(BeTrue(), "userActionsEnabled should be true when auth is configured")
}

func TestGetReport_InjectsUserInfoWithSessionStart(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a session start time
	sessionStartTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Create a user session with session start time
	imp := user.Impersonation{
		Username: "session-start-test-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Session Start Test User"},
		Impersonation: imp,
		SessionStart:  &sessionStartTime,
	}, userClient)

	// Call GetReport
	report, err := handler.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify session start is injected in userInfo
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("Session Start Test User"))
	g.Expect(userInfo["sessionStart"]).To(Equal(sessionStartTime.Format(time.RFC3339)))
}

func TestGetReport_UserInfoSessionStartIsNilWhenNotSet(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session without session start time
	imp := user.Impersonation{
		Username: "no-session-start-test-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "No Session Start Test User"},
		Impersonation: imp,
		// SessionStart not set
	}, userClient)

	// Call GetReport
	report, err := handler.GetReport(userCtx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(report).NotTo(BeNil())

	// Verify session start is nil in userInfo
	spec, found := report.Object["spec"].(map[string]any)
	g.Expect(found).To(BeTrue())
	userInfo, found := spec["userInfo"].(map[string]any)
	g.Expect(found).To(BeTrue())
	g.Expect(userInfo["username"]).To(Equal("No Session Start Test User"))
	_, hasSessionStart := userInfo["sessionStart"]
	g.Expect(hasSessionStart).To(BeFalse(), "userInfo should NOT contain 'sessionStart' when not set")
}

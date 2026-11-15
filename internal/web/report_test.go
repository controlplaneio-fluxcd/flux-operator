// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
}

func TestGetReport_InjectsUserInfoWithSessionStart(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
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

// gatedReportBuild is a report build hook that blocks until the gate is
// closed. It counts the builds and signals each start on started.
type gatedReportBuild struct {
	gate    chan struct{}
	started chan struct{}
	builds  atomic.Int32
	name    string
}

// newGatedReportBuild returns a gated build whose report, search index
// resource and workload are named after name.
func newGatedReportBuild(name string) *gatedReportBuild {
	return &gatedReportBuild{
		gate:    make(chan struct{}),
		started: make(chan struct{}, 100),
		name:    name,
	}
}

// build waits for the gate and returns a clean report. It fails if its
// context is done, which the cold path must prevent by detaching it.
func (b *gatedReportBuild) build(ctx context.Context) (*unstructured.Unstructured, *reporter.FluxStatusReport, *NodesSnapshot, error) {
	b.builds.Add(1)
	b.started <- struct{}{}
	<-b.gate
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	return testReport(b.name)
}

// testReport returns a clean report, its computation and a nodes snapshot,
// all named after name.
func testReport(name string) (*unstructured.Unstructured, *reporter.FluxStatusReport, *NodesSnapshot, error) {
	report := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": fluxcdv1.GroupVersion.String(),
		"kind":       fluxcdv1.FluxReportKind,
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"distribution": map[string]any{"version": "v2.7.0"}},
	}}
	computeResult := &reporter.FluxStatusReport{
		Resources: []reporter.ResourceStatus{{Name: name, Kind: "Kustomization", Namespace: "flux-system"}},
		Workloads: []reporter.WorkloadRef{{Name: name, Kind: "Deployment", Namespace: "flux-system", APIVersion: "apps/v1"}},
	}
	nodes := &NodesSnapshot{Nodes: []*NodeDetail{readyNode(name)}}
	return report, computeResult, nodes, nil
}

// newColdReportHandler returns a handler with an empty report cache
// whose builds go through the given gated build.
func newColdReportHandler(b *gatedReportBuild) *Handler {
	return &Handler{
		kubeClient:      kubeClient,
		namespace:       "flux-system",
		searchIndex:     &SearchIndex{},
		workloadIndex:   &WorkloadIndex{},
		buildReportHook: b.build,
	}
}

// reportResult holds the outcome of a GetReport call.
type reportResult struct {
	report *unstructured.Unstructured
	err    error
}

// getReportAsync calls GetReport on a goroutine and returns its result channel.
func getReportAsync(h *Handler, reqCtx context.Context) <-chan reportResult {
	ch := make(chan reportResult, 1)
	go func() {
		r, err := h.GetReport(reqCtx)
		ch <- reportResult{report: r, err: err}
	}()
	return ch
}

func TestGetReport_ColdConcurrentCallsShareOneBuild(t *testing.T) {
	g := NewWithT(t)

	b := newGatedReportBuild("cold-shared")
	handler := newColdReportHandler(b)

	const callers = 10
	results := make([]<-chan reportResult, callers)
	for i := range callers {
		results[i] = getReportAsync(handler, ctx)
	}

	// The cache stays empty while the build is gated.
	<-b.started
	g.Expect(handler.hasCachedReport()).To(BeFalse())

	close(b.gate)
	for _, ch := range results {
		r := <-ch
		g.Expect(r.err).NotTo(HaveOccurred())
		g.Expect(r.report.GetName()).To(Equal("cold-shared"))
	}
	g.Expect(b.builds.Load()).To(Equal(int32(1)))

	// The indexes are filled from the same build.
	g.Expect(handler.searchIndex.resources).To(HaveLen(1))
	g.Expect(handler.searchIndex.resources[0].Name).To(Equal("cold-shared"))
	g.Expect(handler.workloadIndex.workloads).To(HaveLen(1))
	g.Expect(handler.workloadIndex.workloads[0].Name).To(Equal("cold-shared"))
}

func TestGetReport_ColdPerUserFieldsStayOutOfCache(t *testing.T) {
	g := NewWithT(t)

	b := newGatedReportBuild("cold-per-user")
	handler := newColdReportHandler(b)

	// One caller without authentication sees every namespace,
	// the other one has no access and sees none.
	userCtx := userSession(g, kubeClient, "cold-report-user")
	adminCh := getReportAsync(handler, ctx)
	userCh := getReportAsync(handler, userCtx)
	<-b.started
	close(b.gate)

	admin := <-adminCh
	g.Expect(admin.err).NotTo(HaveOccurred())
	userRes := <-userCh
	g.Expect(userRes.err).NotTo(HaveOccurred())

	adminSpec := admin.report.Object["spec"].(map[string]any)
	userSpec := userRes.report.Object["spec"].(map[string]any)
	g.Expect(adminSpec["userInfo"].(map[string]any)["username"]).NotTo(Equal("cold-report-user"))
	g.Expect(userSpec["userInfo"].(map[string]any)["username"]).To(Equal("cold-report-user"))
	g.Expect(adminSpec["namespaces"]).To(ContainElement("default"))
	g.Expect(userSpec["namespaces"]).To(BeEmpty())

	// The cached report is the clean build output.
	handler.reportCacheMu.RLock()
	cachedSpec := handler.reportCache.Object["spec"].(map[string]any)
	g.Expect(cachedSpec).NotTo(HaveKey("userInfo"))
	g.Expect(cachedSpec).NotTo(HaveKey("namespaces"))
	g.Expect(cachedSpec).NotTo(HaveKey("reconcilers"))
	handler.reportCacheMu.RUnlock()
}

func TestGetReport_ColdCanceledCallerDoesNotFailOthers(t *testing.T) {
	g := NewWithT(t)

	b := newGatedReportBuild("cold-cancel")
	handler := newColdReportHandler(b)

	// The first caller starts the build, then cancels while it is gated.
	firstCtx, cancelFirst := context.WithCancel(ctx)
	defer cancelFirst()
	firstCh := getReportAsync(handler, firstCtx)
	<-b.started
	secondCh := getReportAsync(handler, ctx)

	cancelFirst()
	first := <-firstCh
	g.Expect(first.err).To(MatchError(context.Canceled))

	close(b.gate)
	second := <-secondCh
	g.Expect(second.err).NotTo(HaveOccurred())
	g.Expect(second.report.GetName()).To(Equal("cold-cancel"))
	g.Expect(b.builds.Load()).To(Equal(int32(1)))
}

func TestGetReport_ColdFillsNodesSnapshot(t *testing.T) {
	g := NewWithT(t)

	b := newGatedReportBuild("cold-nodes")
	close(b.gate)
	handler := newColdReportHandler(b)

	// Before any build, the nodes dashboard is unavailable.
	serveNodes := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		handler.NodesHandler(rec, req)
		return rec.Code
	}
	g.Expect(serveNodes()).To(Equal(http.StatusServiceUnavailable))

	_, err := handler.GetReport(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(handler.getCachedNodesSnapshot()).NotTo(BeNil())
	g.Expect(serveNodes()).To(Equal(http.StatusOK))
}

func TestGetReport_ColdBuildDoesNotReplaceNewerRefresh(t *testing.T) {
	g := NewWithT(t)

	b := newGatedReportBuild("cold-stale")
	handler := newColdReportHandler(b)

	ch := getReportAsync(handler, ctx)
	<-b.started

	// A refresh stores newer data while the cold build is gated.
	report, computeResult, nodes, err := testReport("refreshed")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(handler.storeReport(report, computeResult, nodes, false)).To(BeTrue())

	close(b.gate)
	r := <-ch
	g.Expect(r.err).NotTo(HaveOccurred())
	g.Expect(r.report.GetName()).To(Equal("refreshed"))

	// The cache, the snapshot and the indexes keep the newer data.
	handler.reportCacheMu.RLock()
	g.Expect(handler.reportCache.GetName()).To(Equal("refreshed"))
	handler.reportCacheMu.RUnlock()
	g.Expect(handler.getCachedNodesSnapshot().Nodes[0].Name).To(Equal("refreshed"))
	g.Expect(handler.searchIndex.resources[0].Name).To(Equal("refreshed"))
	g.Expect(handler.workloadIndex.workloads[0].Name).To(Equal("refreshed"))

	// A store with onlyIfEmpty is skipped once the cache is set.
	report, computeResult, nodes, err = testReport("stale")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(handler.storeReport(report, computeResult, nodes, true)).To(BeFalse())
	g.Expect(handler.searchIndex.resources[0].Name).To(Equal("refreshed"))
}

func TestStoreReport_IndexesPublishedWithReport(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		searchIndex:   &SearchIndex{},
		workloadIndex: &WorkloadIndex{},
	}

	// Hold the search index lock so the cold store stops at its index update.
	handler.searchIndex.mu.Lock()

	report, computeResult, nodes, err := testReport("cold")
	g.Expect(err).NotTo(HaveOccurred())
	coldDone := make(chan bool, 1)
	go func() {
		coldDone <- handler.storeReport(report, computeResult, nodes, true)
	}()

	// The cold store holds the report cache lock while it waits on the index.
	reportCacheLocked := func() bool {
		if handler.reportCacheMu.TryRLock() {
			handler.reportCacheMu.RUnlock()
			return false
		}
		return true
	}
	g.Eventually(reportCacheLocked).Should(BeTrue())
	g.Consistently(reportCacheLocked, 100*time.Millisecond).Should(BeTrue(),
		"the report cache must stay locked until the indexes are updated")

	// A refresh started now runs after the cold store has published everything.
	newReport, newResult, newNodes, err := testReport("refreshed")
	g.Expect(err).NotTo(HaveOccurred())
	refreshDone := make(chan bool, 1)
	go func() {
		refreshDone <- handler.storeReport(newReport, newResult, newNodes, false)
	}()

	handler.searchIndex.mu.Unlock()
	g.Expect(<-coldDone).To(BeTrue())
	g.Expect(<-refreshDone).To(BeTrue())

	handler.reportCacheMu.RLock()
	g.Expect(handler.reportCache.GetName()).To(Equal("refreshed"))
	handler.reportCacheMu.RUnlock()
	g.Expect(handler.searchIndex.resources[0].Name).To(Equal("refreshed"))
	g.Expect(handler.workloadIndex.workloads[0].Name).To(Equal("refreshed"))
}

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

func TestGetEvents_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-resourceset",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create an event for the ResourceSet
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-resourceset-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.ResourceSetKind,
			Name:      "test-events-resourceset",
			Namespace: "default",
		},
		Type:          "Normal",
		Reason:        "TestReason",
		Message:       "Test event message",
		LastTimestamp: metav1.Now(),
	}
	g.Expect(testClient.Create(ctx, event)).To(Succeed())
	defer testClient.Delete(ctx, event)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetEvents without any user session (privileged)
	events, err := router.GetEvents(ctx, "ResourceSet", "test-events-resourceset", "default", "", "")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(events).NotTo(BeNil())

	// Should find our test event
	found := false
	for _, e := range events {
		if e.InvolvedObject == "ResourceSet/test-events-resourceset" && e.Namespace == "default" {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "should find the test event")
}

func TestGetEvents_UnprivilegedUser_EmptyResult(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-unprivileged",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create an event for the ResourceSet
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-unprivileged-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.ResourceSetKind,
			Name:      "test-events-unprivileged",
			Namespace: "default",
		},
		Type:          "Warning",
		Reason:        "TestWarning",
		Message:       "Test warning message",
		LastTimestamp: metav1.Now(),
	}
	g.Expect(testClient.Create(ctx, event)).To(Succeed())
	defer testClient.Delete(ctx, event)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-events-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetEvents with the unprivileged user context
	// Should return empty result (not error) because user has no namespace access
	events, err := router.GetEvents(userCtx, "ResourceSet", "", "", "", "")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(events).To(BeEmpty(), "unprivileged user should get empty result, not error")
}

func TestGetEvents_WithUserRBAC_OnlyAccessibleEvents(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-rbac",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create an event for the ResourceSet
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-rbac-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.ResourceSetKind,
			Name:      "test-events-rbac",
			Namespace: "default",
		},
		Type:          "Normal",
		Reason:        "Reconciled",
		Message:       "Reconciliation successful",
		LastTimestamp: metav1.Now(),
	}
	g.Expect(testClient.Create(ctx, event)).To(Succeed())
	defer testClient.Delete(ctx, event)

	// Create RBAC for the test user to access events and resourcesets in default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-reader-binding",
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
				Name: "events-reader-user",
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
		Username: "events-reader-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Events Reader User"},
		Impersonation: imp,
	}, userClient)

	// Call GetEvents with the user context
	events, err := router.GetEvents(userCtx, "ResourceSet", "test-events-rbac", "default", "", "")
	g.Expect(err).NotTo(HaveOccurred())

	// Should find our test event
	found := false
	for _, e := range events {
		if e.InvolvedObject == "ResourceSet/test-events-rbac" && e.Namespace == "default" {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "should find the test event in accessible namespace")
}

func TestGetEvents_WithSpecificNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-specific-ns",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create an event for the ResourceSet
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-specific-ns-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.ResourceSetKind,
			Name:      "test-events-specific-ns",
			Namespace: "default",
		},
		Type:          "Normal",
		Reason:        "TestEvent",
		Message:       "Test event for specific namespace",
		LastTimestamp: metav1.Now(),
	}
	g.Expect(testClient.Create(ctx, event)).To(Succeed())
	defer testClient.Delete(ctx, event)

	// Create RBAC for the test user to access events in default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-specific-ns-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-specific-ns-reader-binding",
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
				Name: "events-specific-ns-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session
	imp := user.Impersonation{
		Username: "events-specific-ns-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Events Specific NS User"},
		Impersonation: imp,
	}, userClient)

	// Call GetEvents with specific namespace - should work
	events, err := router.GetEvents(userCtx, "ResourceSet", "test-events-specific-ns", "default", "", "")
	g.Expect(err).NotTo(HaveOccurred())

	// Should find our test event
	found := false
	for _, e := range events {
		if e.InvolvedObject == "ResourceSet/test-events-specific-ns" && e.Namespace == "default" {
			found = true
			break
		}
	}
	g.Expect(found).To(BeTrue(), "should find the test event when querying specific namespace")
}

func TestGetEvents_IgnoresForbiddenErrors(t *testing.T) {
	g := NewWithT(t)

	// Create RBAC for the test user with access only to events for ResourceSet (not other kinds)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-partial-access",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{fluxcdv1.GroupVersion.Group},
				Resources: []string{"resourcesets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-partial-access-binding",
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
				Name: "events-partial-access-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Create a user session
	imp := user.Impersonation{
		Username: "events-partial-access-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Partial Access User"},
		Impersonation: imp,
	}, userClient)

	// Call GetEvents without specifying kind - will query multiple kinds
	// Even if some queries fail with forbidden, the function should NOT return an error
	events, err := router.GetEvents(userCtx, "", "", "default", "", "")
	g.Expect(err).NotTo(HaveOccurred(), "should not return error even when some kinds are forbidden")
	g.Expect(events).NotTo(BeNil())
}

func TestGetEvents_FilterByEventType(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-filter-type",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create a Normal event
	normalEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-normal",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.ResourceSetKind,
			Name:      "test-events-filter-type",
			Namespace: "default",
		},
		Type:          "Normal",
		Reason:        "Reconciled",
		Message:       "Normal event",
		LastTimestamp: metav1.Now(),
	}
	g.Expect(testClient.Create(ctx, normalEvent)).To(Succeed())
	defer testClient.Delete(ctx, normalEvent)

	// Create a Warning event
	warningEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-events-warning",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.ResourceSetKind,
			Name:      "test-events-filter-type",
			Namespace: "default",
		},
		Type:          "Warning",
		Reason:        "Failed",
		Message:       "Warning event",
		LastTimestamp: metav1.Now(),
	}
	g.Expect(testClient.Create(ctx, warningEvent)).To(Succeed())
	defer testClient.Delete(ctx, warningEvent)

	// Create the router
	mux := http.NewServeMux()
	router := NewRouter(mux, nil, kubeClient, testLog, "v1.0.0", "test-status-manager", "flux-system", 5*time.Minute, func(h http.Handler) http.Handler { return h })

	// Call GetEvents filtering by Warning type only
	events, err := router.GetEvents(ctx, "ResourceSet", "test-events-filter-type", "default", "", "Warning")
	g.Expect(err).NotTo(HaveOccurred())

	// Should only find Warning events
	for _, e := range events {
		if e.InvolvedObject == "ResourceSet/test-events-filter-type" {
			g.Expect(e.Type).To(Equal("Warning"), "should only return Warning events when filtered")
		}
	}
}

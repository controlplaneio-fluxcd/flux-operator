// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestIsAuditEnabled(t *testing.T) {
	tests := []struct {
		name          string
		auditList     []string
		action        string
		eventRecorder bool
		expected      bool
	}{
		{
			name:          "specific action in list",
			auditList:     []string{"reconcile"},
			action:        "reconcile",
			eventRecorder: true,
			expected:      true,
		},
		{
			name:          "action not in list",
			auditList:     []string{"suspend"},
			action:        "reconcile",
			eventRecorder: true,
			expected:      false,
		},
		{
			name:          "wildcard matches any action",
			auditList:     []string{"*"},
			action:        "reconcile",
			eventRecorder: true,
			expected:      true,
		},
		{
			name:          "wildcard matches restart action",
			auditList:     []string{"*"},
			action:        "restart",
			eventRecorder: true,
			expected:      true,
		},
		{
			name:          "empty list",
			auditList:     []string{},
			action:        "reconcile",
			eventRecorder: true,
			expected:      false,
		},
		{
			name:          "nil event recorder",
			auditList:     []string{"reconcile"},
			action:        "reconcile",
			eventRecorder: false,
			expected:      false,
		},
		{
			name:          "multiple actions in list",
			auditList:     []string{"reconcile", "suspend", "resume"},
			action:        "suspend",
			eventRecorder: true,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var recorder record.EventRecorder
			if tt.eventRecorder {
				recorder = record.NewFakeRecorder(10)
			}

			handler := &Handler{
				conf: &fluxcdv1.WebConfigSpec{
					UserActions: &fluxcdv1.UserActionsSpec{
						Audit: tt.auditList,
					},
				},
				eventRecorder: recorder,
			}

			result := handler.isAuditEnabled(tt.action)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestFetchReconcilerRef(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing.
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fetch-reconciler",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	handler := &Handler{
		conf:       oauthConfig(),
		kubeClient: kubeClient,
	}

	client := kubeClient.GetClient(ctx)

	t.Run("valid reconciler ref", func(t *testing.T) {
		g := NewWithT(t)

		obj, err := handler.fetchReconcilerRef(ctx, client, "ResourceSet/default/test-fetch-reconciler")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(obj).ToNot(BeNil())
		g.Expect(obj.GetName()).To(Equal("test-fetch-reconciler"))
		g.Expect(obj.GetNamespace()).To(Equal("default"))
	})

	t.Run("invalid format - too few parts", func(t *testing.T) {
		g := NewWithT(t)

		_, err := handler.fetchReconcilerRef(ctx, client, "ResourceSet/default")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid reconciler ref"))
	})

	t.Run("invalid format - too many parts", func(t *testing.T) {
		g := NewWithT(t)

		_, err := handler.fetchReconcilerRef(ctx, client, "ResourceSet/default/name/extra")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid reconciler ref"))
	})

	t.Run("unknown kind", func(t *testing.T) {
		g := NewWithT(t)

		_, err := handler.fetchReconcilerRef(ctx, client, "UnknownKind/default/test")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unable to get GVK"))
	})

	t.Run("resource not found", func(t *testing.T) {
		g := NewWithT(t)

		_, err := handler.fetchReconcilerRef(ctx, client, "ResourceSet/default/nonexistent")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("unable to fetch reconciler"))
	})
}

func TestSendAuditEvent_WithWorkload(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet to act as the reconciler.
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-audit-reconciler-ref",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	fakeRecorder := record.NewFakeRecorder(10)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
			},
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*"},
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: fakeRecorder,
	}

	// Create a workload object (the action target).
	workloadObj := &metav1.PartialObjectMetadata{}
	workloadObj.SetName("my-deployment")
	workloadObj.SetNamespace("default")
	workloadObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})

	// Create a workload with labels pointing to the ResourceSet.
	workload := &unstructured.Unstructured{}
	workload.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	workload.SetName("my-deployment")
	workload.SetNamespace("default")
	workload.SetLabels(map[string]string{
		"resourceset.fluxcd.controlplane.io/name":      "test-audit-reconciler-ref",
		"resourceset.fluxcd.controlplane.io/namespace": "default",
	})

	// Set up user context.
	testCtx := user.StoreSession(ctx, user.Details{
		Impersonation: user.Impersonation{
			Username: "test-user",
			Groups:   []string{"developers"},
		},
	}, nil)

	// Send audit event with workload.
	handler.sendAuditEvent(testCtx, "restart", workloadObj, workload)

	// Check that an event was recorded.
	select {
	case event := <-fakeRecorder.Events:
		g.Expect(event).To(ContainSubstring("WebAction"))
		g.Expect(event).To(ContainSubstring("test-user"))
		g.Expect(event).To(ContainSubstring("restart"))
	default:
		t.Fatal("expected an audit event to be recorded")
	}
}

func TestSendAuditEvent_WorkloadReconcilerNotFound(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
			},
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*"},
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: fakeRecorder,
	}

	// Create a workload object (the action target).
	workloadObj := &metav1.PartialObjectMetadata{}
	workloadObj.SetName("my-deployment")
	workloadObj.SetNamespace("default")
	workloadObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})

	// Create a workload with labels pointing to a non-existent ResourceSet.
	workload := &unstructured.Unstructured{}
	workload.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	workload.SetName("my-deployment")
	workload.SetNamespace("default")
	workload.SetLabels(map[string]string{
		"resourceset.fluxcd.controlplane.io/name":      "nonexistent",
		"resourceset.fluxcd.controlplane.io/namespace": "default",
	})

	// Set up user context.
	testCtx := user.StoreSession(ctx, user.Details{
		Impersonation: user.Impersonation{
			Username: "test-user",
			Groups:   []string{"developers"},
		},
	}, nil)

	// Send audit event with workload pointing to non-existent reconciler.
	handler.sendAuditEvent(testCtx, "restart", workloadObj, workload)

	// Check that NO event was recorded (fetch failed, so audit is skipped).
	select {
	case event := <-fakeRecorder.Events:
		t.Fatalf("expected no audit event when reconciler ref not found, but got: %s", event)
	default:
		// No event - this is expected.
	}
}

func TestSendAuditEvent_NilWorkload(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing.
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-audit-empty-ref",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	fakeRecorder := record.NewFakeRecorder(10)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
			},
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{"reconcile"},
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: fakeRecorder,
	}

	// Create the object directly (no workload).
	obj := &metav1.PartialObjectMetadata{}
	obj.SetName("test-audit-empty-ref")
	obj.SetNamespace("default")
	obj.SetGroupVersionKind(fluxcdv1.GroupVersion.WithKind("ResourceSet"))

	// Set up user context.
	testCtx := user.StoreSession(ctx, user.Details{
		Impersonation: user.Impersonation{
			Username: "admin",
			Groups:   []string{"system:masters"},
		},
	}, nil)

	// Send audit event with nil workload.
	handler.sendAuditEvent(testCtx, "reconcile", obj, nil)

	// Check that an event was recorded.
	select {
	case event := <-fakeRecorder.Events:
		g.Expect(event).To(ContainSubstring("WebAction"))
		g.Expect(event).To(ContainSubstring("admin"))
	default:
		t.Fatal("expected an audit event to be recorded")
	}
}

func TestSendAuditEvent_AuditDisabled(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{}, // Empty audit list.
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: fakeRecorder,
	}

	obj := &metav1.PartialObjectMetadata{}
	obj.SetName("test")
	obj.SetNamespace("default")

	// Set up user context.
	testCtx := user.StoreSession(ctx, user.Details{
		Impersonation: user.Impersonation{
			Username: "test-user",
			Groups:   []string{"developers"},
		},
	}, nil)

	// Send audit event - should be skipped.
	handler.sendAuditEvent(testCtx, "reconcile", obj, nil)

	// Check that NO event was recorded.
	select {
	case event := <-fakeRecorder.Events:
		t.Fatalf("expected no audit event when audit is disabled, but got: %s", event)
	default:
		// No event - this is expected.
	}
}

func TestSendAuditEvent_NilEventRecorder(t *testing.T) {
	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*"},
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: nil, // No event recorder.
	}

	obj := &metav1.PartialObjectMetadata{}
	obj.SetName("test")
	obj.SetNamespace("default")

	// Set up user context.
	testCtx := user.StoreSession(ctx, user.Details{
		Impersonation: user.Impersonation{
			Username: "test-user",
			Groups:   []string{"developers"},
		},
	}, nil)

	// This should not panic.
	handler.sendAuditEvent(testCtx, "reconcile", obj, nil)
}

// Integration tests for audit through handlers.

func TestActionHandler_Audit_Integration(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet for testing.
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-action-audit-integration",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	fakeRecorder := record.NewFakeRecorder(10)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
			},
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*"},
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: fakeRecorder,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create user session for the audit test.
	imp := user.Impersonation{
		Username: "audit-action-user",
		Groups:   []string{"system:masters"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Audit Action User"},
		Impersonation: imp,
	}, userClient)

	actionReq := ActionRequest{
		Kind:      "ResourceSet",
		Namespace: "default",
		Name:      "test-action-audit-integration",
		Action:    "reconcile",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.ActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	// Check that an audit event was recorded.
	select {
	case event := <-fakeRecorder.Events:
		g.Expect(event).To(ContainSubstring("WebAction"))
		g.Expect(event).To(ContainSubstring("reconcile"))
		// The event message should include the resource reference.
		g.Expect(event).To(ContainSubstring("ResourceSet/default/test-action-audit-integration"))
		// The event message should include the username.
		g.Expect(event).To(ContainSubstring("audit-action-user"))
	default:
		t.Fatal("expected an audit event to be recorded for resource action")
	}
}

func TestWorkloadActionHandler_Audit_Integration(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet to act as the reconciler.
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-audit-ks",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer testClient.Delete(ctx, resourceSet)

	// Create a Deployment managed by the ResourceSet.
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-audit-deploy",
			Namespace: "default",
			Labels: map[string]string{
				"resourceset.fluxcd.controlplane.io/name":      "test-workload-audit-ks",
				"resourceset.fluxcd.controlplane.io/namespace": "default",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-audit"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-audit"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	fakeRecorder := record.NewFakeRecorder(10)

	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
			},
			UserActions: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*"},
			},
		},
		kubeClient:    kubeClient,
		eventRecorder: fakeRecorder,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create user session for the audit test.
	imp := user.Impersonation{
		Username: "audit-workload-user",
		Groups:   []string{"system:masters"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Audit Workload User"},
		Impersonation: imp,
	}, userClient)

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-workload-audit-deploy",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	// Check that an audit event was recorded with the reconciler ref in the message.
	select {
	case event := <-fakeRecorder.Events:
		g.Expect(event).To(ContainSubstring("WebAction"))
		g.Expect(event).To(ContainSubstring("restart"))
		// The event message should include the workload reference.
		g.Expect(event).To(ContainSubstring("Deployment/default/test-workload-audit-deploy"))
		// The event message should include the username.
		g.Expect(event).To(ContainSubstring("audit-workload-user"))
	default:
		t.Fatal("expected an audit event to be recorded for workload action")
	}
}

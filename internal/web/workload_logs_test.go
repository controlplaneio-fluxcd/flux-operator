// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/podlogs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestCollectLogStreams(t *testing.T) {
	errWaiting := errors.New("waiting to start")
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "pods/log"}, "p2", errors.New("nope"))

	t.Run("all succeed across two pods", func(t *testing.T) {
		g := NewWithT(t)
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p2", Container: "app"}}
		out := collectLogStreams(targets, []string{"x", "y"}, []error{nil, nil})
		g.Expect(out.streams).To(Equal([]podlogs.LogStream{{Pod: "p1", Container: "app", Blob: "x"}, {Pod: "p2", Container: "app", Blob: "y"}}))
		g.Expect(out.streamedSet).To(HaveLen(2))
		g.Expect(out.forbidden).To(Equal(0))
		g.Expect(out.firstErr).NotTo(HaveOccurred())
	})

	t.Run("partial failure skips the failed target and counts forbidden", func(t *testing.T) {
		g := NewWithT(t)
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p2", Container: "app"}}
		out := collectLogStreams(targets, []string{"x", ""}, []error{nil, forbidden})
		g.Expect(out.streams).To(Equal([]podlogs.LogStream{{Pod: "p1", Container: "app", Blob: "x"}}))
		g.Expect(out.streamedSet).To(HaveLen(1))
		g.Expect(out.forbidden).To(Equal(1))
		g.Expect(out.firstErr).To(MatchError(forbidden))
	})

	t.Run("all fail returns no streams and the first error", func(t *testing.T) {
		g := NewWithT(t)
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p2", Container: "app"}}
		out := collectLogStreams(targets, []string{"", ""}, []error{errWaiting, forbidden})
		g.Expect(out.streams).To(BeEmpty())
		g.Expect(out.streamedSet).To(BeEmpty())
		g.Expect(out.forbidden).To(Equal(1))
		g.Expect(out.firstErr).To(MatchError(errWaiting))
	})

	t.Run("forbidden is counted per pod, not per container target", func(t *testing.T) {
		g := NewWithT(t)
		// One forbidden pod with two containers must count once, not twice.
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p1", Container: "side"}}
		out := collectLogStreams(targets, []string{"", ""}, []error{forbidden, forbidden})
		g.Expect(out.forbidden).To(Equal(1))
	})
}

func TestBuildWorkloadLogsResponse(t *testing.T) {
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "pods/log"}, "p2", errors.New("nope"))

	t.Run("partial coverage when one requested pod is forbidden", func(t *testing.T) {
		g := NewWithT(t)
		// Two pods requested, only p1 streams; p2 is forbidden. The response must
		// report 200-style partial coverage with the per-pod counts the UI shows.
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p2", Container: "app"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00Z hi\n", ""}, []error{nil, forbidden})
		resp := buildWorkloadLogsResponse([]string{"p1", "p2"}, []string{"app"}, fanOut, 2, 1000, true, false, false)

		g.Expect(resp.Tagged).To(BeTrue())
		g.Expect(resp.ContainerTagged).To(BeFalse())
		g.Expect(resp.Total).To(Equal(2))
		g.Expect(resp.Streamed).To(Equal(1))
		g.Expect(resp.Partial).To(BeTrue())
		g.Expect(resp.Forbidden).To(Equal(1))
		g.Expect(resp.Pod).To(Equal("p1,p2"))
		g.Expect(resp.Logs).To(ContainSubstring("hi"))
	})

	t.Run("full coverage is not partial", func(t *testing.T) {
		g := NewWithT(t)
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p2", Container: "app"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00Z a\n", "2026-01-01T00:00:01Z b\n"}, []error{nil, nil})
		resp := buildWorkloadLogsResponse([]string{"p1", "p2"}, []string{"app"}, fanOut, 2, 1000, true, false, false)

		g.Expect(resp.Streamed).To(Equal(2))
		g.Expect(resp.Partial).To(BeFalse())
		g.Expect(resp.Forbidden).To(Equal(0))
	})

	t.Run("truncation forces partial even when every requested pod streamed", func(t *testing.T) {
		g := NewWithT(t)
		// total counts requested pods (1) and that pod streamed, but the fan-out
		// was capped, so the response must still flag the result as partial.
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00Z x\n"}, []error{nil})
		resp := buildWorkloadLogsResponse([]string{"p1"}, []string{"app"}, fanOut, 1, 1000, true, false, true)

		g.Expect(resp.Streamed).To(Equal(1))
		g.Expect(resp.Total).To(Equal(1))
		g.Expect(resp.Partial).To(BeTrue())
	})

	t.Run("all-containers single pod tags the container, not the pod", func(t *testing.T) {
		g := NewWithT(t)
		// One pod, two containers: ContainerTagged is set and each line is prefixed
		// with its container so the client scopes folding per container.
		targets := []podlogs.LogTarget{{Pod: "p1", Container: "app"}, {Pod: "p1", Container: "envoy"}}
		fanOut := collectLogStreams(targets, []string{"2026-01-01T00:00:00.123456789Z a\n", "2026-01-01T00:00:01.987654321Z e\n"}, []error{nil, nil})
		resp := buildWorkloadLogsResponse([]string{"p1"}, []string{"app", "envoy"}, fanOut, 1, 1000, false, true, false)

		g.Expect(resp.Tagged).To(BeFalse())
		g.Expect(resp.ContainerTagged).To(BeTrue())
		g.Expect(resp.Logs).To(Equal("app 2026-01-01T00:00:00.123456789Z a\nenvoy 2026-01-01T00:00:01.987654321Z e\n"))
	})
}

func TestWorkloadLogsHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/logs", nil)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestWorkloadLogsHandler_UserActionsDisabled(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          &fluxcdv1.WebConfigSpec{},
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=kube-system&name=sensitive-pod&container=app", nil)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("User actions are disabled"))
}

func TestWorkloadLogsHandler_MissingParams(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name  string
		query string
	}{
		{name: "missing both", query: ""},
		{name: "missing name", query: "namespace=default"},
		{name: "missing namespace", query: "name=test-pod"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/workload/logs?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.WorkloadLogsHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required query parameters"))
		})
	}
}

func TestWorkloadLogsHandler_InvalidParams(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name    string
		query   string
		message string
	}{
		{name: "invalid tailLines", query: "namespace=default&name=test-pod&tailLines=abc", message: "Invalid tailLines parameter"},
		{name: "negative tailLines", query: "namespace=default&name=test-pod&tailLines=-5", message: "Invalid tailLines parameter"},
		{name: "invalid previous", query: "namespace=default&name=test-pod&previous=maybe", message: "Invalid previous parameter"},
		{name: "invalid sinceTime", query: "namespace=default&name=test-pod&sinceTime=not-a-time", message: "Invalid sinceTime parameter"},
		{name: "since without separator", query: "namespace=default&name=test-pod&pod=other&since=pod-a", message: "Invalid since parameter"},
		{name: "since with empty pod", query: "namespace=default&name=test-pod&pod=other&since==2026-01-01T00:00:00Z", message: "Invalid since parameter"},
		{name: "since with bad timestamp", query: "namespace=default&name=test-pod&pod=other&since=pod-a=not-a-time", message: "Invalid since parameter"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/workload/logs?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.WorkloadLogsHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring(tc.message))
		})
	}
}

func TestWorkloadLogsHandler_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// A user without the pods/log permission must be rejected with 403 by the
	// API server when the impersonated client attempts to read the logs.
	username := "logs-forbidden-user"
	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Logs Forbidden User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=default&name=test-pod&container=app", nil).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestWorkloadLogsHandler_AllContainersForbidden(t *testing.T) {
	g := NewWithT(t)

	// The all-containers path (repeated container params) is governed by the same
	// pods/log RBAC as the single-container path: a user without it gets 403 once
	// every container stream fails.
	username := "logs-forbidden-multi-user"
	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Logs Forbidden Multi User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=default&name=test-pod&container=app&container=sidecar", nil).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestWorkloadLogsHandler_AllPodsForbidden(t *testing.T) {
	g := NewWithT(t)

	// The all-pods path (the primary name plus repeated pod params) is governed by
	// the same pods/log RBAC. With more than one pod the failure is workload-scoped
	// (no single pod is named), so the 403 message does not pin to one pod.
	username := "logs-forbidden-allpods-user"
	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())
	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Logs Forbidden All Pods User"},
		Impersonation: imp,
	}, userClient)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/workload/logs?namespace=default&name=pod-a&pod=pod-b&container=app", nil).WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadLogsHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
	// Workload-scoped message: it does not name a single pod.
	g.Expect(rec.Body.String()).To(ContainSubstring("workload pod logs"))
}

func TestGetWorkloadStatus_ViewLogsCapability(t *testing.T) {
	g := NewWithT(t)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-logs",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-logs"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-logs"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:1.25"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// baseRules grant enough access to read the workload and list its pods,
	// but deliberately omit the pods/log subresource.
	baseRules := []rbacv1.PolicyRule{
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list"}},
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
	}

	t.Run("with pods/log permission", func(t *testing.T) {
		g := NewWithT(t)

		rules := append([]rbacv1.PolicyRule{}, baseRules...)
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: []string{"get"},
		})
		userCtx := bindWorkloadLogsUser(t, g, "logs-reader-user", rules)

		workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-logs", "default", true)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionViewLogs))
	})

	t.Run("without pods/log permission", func(t *testing.T) {
		g := NewWithT(t)

		userCtx := bindWorkloadLogsUser(t, g, "logs-noreader-user", baseRules)

		workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-logs", "default", true)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload.UserActions).NotTo(ContainElement(fluxcdv1.UserActionViewLogs))
	})
}

// bindWorkloadLogsUser creates a namespaced Role with the given rules in the
// default namespace, binds it to username, and returns an impersonated user
// context for use with the handler.
func bindWorkloadLogsUser(t *testing.T, g *WithT, username string, rules []rbacv1.PolicyRule) context.Context {
	t.Helper()

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: username + "-role", Namespace: "default"},
		Rules:      rules,
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, role) })

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: username + "-binding", Namespace: "default"},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{{Kind: "User", Name: username}},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	t.Cleanup(func() { _ = testClient.Delete(ctx, roleBinding) })

	imp := user.Impersonation{Username: username, Groups: []string{"system:authenticated"}}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	return user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: username},
		Impersonation: imp,
	}, userClient)
}

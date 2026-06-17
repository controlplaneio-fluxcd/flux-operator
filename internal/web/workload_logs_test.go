// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestTrimPartialLogLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "newline terminated", in: "line one\nline two\n", want: "line one\nline two\n"},
		{name: "drops partial trailing line", in: "line one\nline two\npar", want: "line one\nline two\n"},
		{name: "single partial line kept", in: "partial", want: "partial"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(trimPartialLogLine(tt.in)).To(Equal(tt.want))
		})
	}
}

func TestTrimPartialFirstLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "drops partial leading fragment", in: "tial\nline two\nline three\n", want: "line two\nline three\n"},
		{name: "single line with no newline kept", in: "only-line", want: "only-line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(trimPartialFirstLine(tt.in)).To(Equal(tt.want))
		})
	}
}

func TestTailLogBytes(t *testing.T) {
	tests := []struct {
		name             string
		in               string
		limit            int
		want             string
		wantPartialFirst bool
	}{
		{name: "under limit returns all", in: "line1\nline2\n", limit: 100, want: "line1\nline2\n", wantPartialFirst: false},
		{name: "exact limit not truncated", in: "abcd", limit: 4, want: "abcd", wantPartialFirst: false},
		// The cut lands exactly after "line1\n", so the first retained line is
		// complete and must NOT be reported as partial (or the caller drops it).
		{name: "over limit cut on line boundary keeps complete first line", in: "line1\nline2\nline3\n", limit: 12, want: "line2\nline3\n", wantPartialFirst: false},
		{name: "over limit cutting mid-line", in: "line1\nline2\n", limit: 8, want: "1\nline2\n", wantPartialFirst: true},
		{name: "empty stream", in: "", limit: 16, want: "", wantPartialFirst: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// strings.Reader hands out small reads, exercising the chunked loop.
			got, partialFirst, err := tailLogBytes(strings.NewReader(tt.in), tt.limit)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(string(got)).To(Equal(tt.want))
			g.Expect(partialFirst).To(Equal(tt.wantPartialFirst))
		})
	}
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

		workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-logs", "default", false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionViewLogs))
	})

	t.Run("without pods/log permission", func(t *testing.T) {
		g := NewWithT(t)

		userCtx := bindWorkloadLogsUser(t, g, "logs-noreader-user", baseRules)

		workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-logs", "default", false)
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

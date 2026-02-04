// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestWorkloadActionHandler_MethodNotAllowed(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Test with GET method (should fail)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload/action", nil)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("Method not allowed"))
}

func TestWorkloadActionHandler_InvalidJSON(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBufferString("invalid json"))
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Invalid request body"))
}

func TestWorkloadActionHandler_MissingFields(t *testing.T) {
	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name    string
		request WorkloadActionRequest
	}{
		{
			name:    "missing kind",
			request: WorkloadActionRequest{Namespace: "default", Name: "test", Action: "restart"},
		},
		{
			name:    "missing namespace",
			request: WorkloadActionRequest{Kind: "Deployment", Name: "test", Action: "restart"},
		},
		{
			name:    "missing name",
			request: WorkloadActionRequest{Kind: "Deployment", Namespace: "default", Action: "restart"},
		},
		{
			name:    "missing action",
			request: WorkloadActionRequest{Kind: "Deployment", Namespace: "default", Name: "test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			body, _ := json.Marshal(tc.request)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
			rec := httptest.NewRecorder()

			handler.WorkloadActionHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required fields"))
		})
	}
}

func TestWorkloadActionHandler_UnsupportedKind(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := WorkloadActionRequest{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "test",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("Unsupported workload kind"))
}

func TestWorkloadActionHandler_UnsupportedAction(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test",
		Action:    "invalid-action",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
	g.Expect(rec.Body.String()).To(ContainSubstring("not supported"))
}

func TestWorkloadActionHandler_ActionsDisabled_NoAuth(t *testing.T) {
	g := NewWithT(t)

	// Test with no authentication configured
	handler := &Handler{
		conf: &fluxcdv1.WebConfigSpec{
			UserActions: &fluxcdv1.UserActionsSpec{},
		},
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	g.Expect(rec.Body.String()).To(ContainSubstring("User actions are disabled"))
}

func TestWorkloadActionHandler_Restart_Deployment_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-restart",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1PodTemplateSpec("test"),
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

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-workload-restart",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp WorkloadActionResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
	g.Expect(resp.Message).To(ContainSubstring("Rollout restart triggered"))

	// Verify annotation was set on pod template
	var updated appsv1.Deployment
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), &updated)).To(Succeed())
	g.Expect(updated.Spec.Template.Annotations).To(HaveKey("kubectl.kubernetes.io/restartedAt"))
}

func TestWorkloadActionHandler_Restart_StatefulSet_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a StatefulSet for testing
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-restart-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-sts"},
			},
			Template: corev1PodTemplateSpec("test-sts"),
		},
	}
	g.Expect(testClient.Create(ctx, statefulset)).To(Succeed())
	defer testClient.Delete(ctx, statefulset)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := WorkloadActionRequest{
		Kind:      "StatefulSet",
		Namespace: "default",
		Name:      "test-workload-restart-sts",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp WorkloadActionResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())

	// Verify annotation was set on pod template
	var updated appsv1.StatefulSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(statefulset), &updated)).To(Succeed())
	g.Expect(updated.Spec.Template.Annotations).To(HaveKey("kubectl.kubernetes.io/restartedAt"))
}

func TestWorkloadActionHandler_Restart_DaemonSet_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a DaemonSet for testing
	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-restart-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-ds"},
			},
			Template: corev1PodTemplateSpec("test-ds"),
		},
	}
	g.Expect(testClient.Create(ctx, daemonset)).To(Succeed())
	defer testClient.Delete(ctx, daemonset)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	actionReq := WorkloadActionRequest{
		Kind:      "DaemonSet",
		Namespace: "default",
		Name:      "test-workload-restart-ds",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp WorkloadActionResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())

	// Verify annotation was set on pod template
	var updated appsv1.DaemonSet
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(daemonset), &updated)).To(Succeed())
	g.Expect(updated.Spec.Template.Annotations).To(HaveKey("kubectl.kubernetes.io/restartedAt"))
}

func TestWorkloadActionHandler_WorkloadNotFound(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Try to restart a workload in a non-existent namespace
	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "non-existent-namespace-12345",
		Name:      "non-existent-workload",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	// Server-Side Apply may return different errors for non-existent namespaces
	// The important thing is that it fails (either 404 or 500)
	g.Expect(rec.Code).To(Or(Equal(http.StatusNotFound), Equal(http.StatusInternalServerError)))
}

func TestWorkloadActionHandler_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-forbidden",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1PodTemplateSpec("test"),
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

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-workload-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-workload-forbidden",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("Permission denied"))
}

func TestWorkloadActionHandler_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-rbac-success",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1PodTemplateSpec("test"),
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC for the test user to perform restart action on deployments
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-workload-restarter",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "patch", "restart"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRole)).To(Succeed())
	defer testClient.Delete(ctx, clusterRole)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-workload-restarter-binding",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: "workload-restarter-user",
			},
		},
	}
	g.Expect(testClient.Create(ctx, clusterRoleBinding)).To(Succeed())
	defer testClient.Delete(ctx, clusterRoleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create a user session with restart access
	imp := user.Impersonation{
		Username: "workload-restarter-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Workload Restarter User"},
		Impersonation: imp,
	}, userClient)

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-workload-rbac-success",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))

	var resp WorkloadActionResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp.Success).To(BeTrue())
}

func TestWorkloadActionHandler_ResponseContentType(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-content-type",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: corev1PodTemplateSpec("test"),
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

	actionReq := WorkloadActionRequest{
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "test-workload-content-type",
		Action:    "restart",
	}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadActionHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
}

func TestWorkloadActionHandler_AllSupportedKinds(t *testing.T) {
	supportedKinds := []string{"Deployment", "StatefulSet", "DaemonSet"}

	for _, kind := range supportedKinds {
		t.Run(kind, func(t *testing.T) {
			g := NewWithT(t)

			// Create workload based on kind (use lowercase names for K8s)
			name := "test-workload-all-" + strings.ToLower(kind)
			switch kind {
			case "Deployment":
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": name},
						},
						Template: corev1PodTemplateSpec(name),
					},
				}
				g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
				defer testClient.Delete(ctx, deployment)
			case "StatefulSet":
				statefulset := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: "default",
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": name},
						},
						Template: corev1PodTemplateSpec(name),
					},
				}
				g.Expect(testClient.Create(ctx, statefulset)).To(Succeed())
				defer testClient.Delete(ctx, statefulset)
			case "DaemonSet":
				daemonset := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: "default",
					},
					Spec: appsv1.DaemonSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": name},
						},
						Template: corev1PodTemplateSpec(name),
					},
				}
				g.Expect(testClient.Create(ctx, daemonset)).To(Succeed())
				defer testClient.Delete(ctx, daemonset)
			}

			handler := &Handler{
				conf:          oauthConfig(),
				kubeClient:    kubeClient,
				version:       "v1.0.0",
				statusManager: "test-status-manager",
				namespace:     "flux-system",
			}

			actionReq := WorkloadActionRequest{
				Kind:      kind,
				Namespace: "default",
				Name:      name,
				Action:    "restart",
			}
			body, _ := json.Marshal(actionReq)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/action", bytes.NewBuffer(body))
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.WorkloadActionHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusOK))

			var resp WorkloadActionResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Success).To(BeTrue())
		})
	}
}

// corev1PodTemplateSpec creates a minimal pod template spec for testing.
func corev1PodTemplateSpec(appLabel string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": appLabel},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "nginx:latest",
				},
			},
		},
	}
}

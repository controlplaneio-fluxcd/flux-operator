// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetWorkloadsStatus_Privileged_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-success",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-success"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-success"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadsStatus without any user session (privileged)
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "default", Name: "test-workload-success"},
	}
	results := handler.GetWorkloadsStatus(ctx, workloads)

	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-workload-success"))
	g.Expect(results[0].Namespace).To(Equal("default"))
	g.Expect(results[0].Kind).To(Equal("Deployment"))
	g.Expect(results[0].Status).NotTo(Equal("NotFound"))
}

func TestGetWorkloadsStatus_Privileged_NotFound(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadsStatus with a non-existent workload
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "default", Name: "nonexistent-deployment"},
	}
	results := handler.GetWorkloadsStatus(ctx, workloads)

	// Should return result with NotFound status and user-friendly message
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("nonexistent-deployment"))
	g.Expect(results[0].Namespace).To(Equal("default"))
	g.Expect(results[0].Kind).To(Equal("Deployment"))
	g.Expect(results[0].Status).To(Equal("NotFound"))
	g.Expect(results[0].StatusMessage).To(Equal("Workload not found in the cluster"))
}

func TestGetWorkloadsStatus_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-forbidden",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-forbidden"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-forbidden"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create an unprivileged user session (no RBAC permissions)
	imp := user.Impersonation{
		Username: "unprivileged-workloads-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadsStatus with the unprivileged user context
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "default", Name: "test-workload-forbidden"},
	}
	results := handler.GetWorkloadsStatus(userCtx, workloads)

	// Should return result with NotFound status and forbidden message
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-workload-forbidden"))
	g.Expect(results[0].Status).To(Equal("NotFound"))
	g.Expect(results[0].StatusMessage).To(Equal("User does not have access to the workload or for listing its pods"))
}

func TestGetWorkloadsStatus_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-rbac-success",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-rbac-success"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-rbac-success"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC for the test user to access deployments and pods
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "daemonsets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-reader-binding",
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
				Name: "workloads-reader-user",
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

	// Create a user session with RBAC access
	imp := user.Impersonation{
		Username: "workloads-reader-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Workloads Reader User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadsStatus with the user context
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "default", Name: "test-workload-rbac-success"},
	}
	results := handler.GetWorkloadsStatus(userCtx, workloads)

	// Should return the workload successfully
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-workload-rbac-success"))
	g.Expect(results[0].Status).NotTo(Equal("NotFound"))
}

func TestGetWorkloadsStatus_WithNamespaceScopedRBAC_ForbiddenInOtherNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create a unique namespace for this test
	otherNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workloads-ns-test",
		},
	}
	g.Expect(testClient.Create(ctx, otherNS)).To(Succeed())
	defer testClient.Delete(ctx, otherNS)

	// Create a Deployment in the other namespace
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-other-ns",
			Namespace: "workloads-ns-test",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-other-ns"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-other-ns"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC for the test user in default namespace only
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-ns-scoped-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "daemonsets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-ns-scoped-reader-binding",
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
				Name: "workloads-ns-scoped-user",
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

	// Create a user session with namespace-scoped access
	imp := user.Impersonation{
		Username: "workloads-ns-scoped-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Scoped User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadsStatus for workload in workloads-ns-test (user only has access to default)
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "workloads-ns-test", Name: "test-workload-other-ns"},
	}
	results := handler.GetWorkloadsStatus(userCtx, workloads)

	// Should return forbidden message
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-workload-other-ns"))
	g.Expect(results[0].Status).To(Equal("NotFound"))
	g.Expect(results[0].StatusMessage).To(Equal("User does not have access to the workload or for listing its pods"))
}

func TestGetWorkloadsStatus_WithDeploymentAccessButNoPodAccess_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-no-pod-access",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-no-pod-access"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-no-pod-access"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC for the test user with deployment access but NO pod access
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-no-pod-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "daemonsets"},
				Verbs:     []string{"get", "list"},
			},
			// Note: No pod access here
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-no-pod-reader-binding",
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
				Name: "workloads-no-pod-user",
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

	// Create a user session with deployment access but no pod access
	imp := user.Impersonation{
		Username: "workloads-no-pod-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "No Pod Access User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadsStatus - user can get deployment but not list pods
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "default", Name: "test-workload-no-pod-access"},
	}
	results := handler.GetWorkloadsStatus(userCtx, workloads)

	// Should return forbidden message since pod listing fails
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-workload-no-pod-access"))
	g.Expect(results[0].Status).To(Equal("NotFound"))
	g.Expect(results[0].StatusMessage).To(Equal("User does not have access to the workload or for listing its pods"))
}

func TestGetWorkloadsStatus_MixedResults(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-mixed-exists",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-mixed-exists"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-mixed-exists"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC for the test user with access to deployments and pods in default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-mixed-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "daemonsets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workloads-mixed-reader-binding",
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
				Name: "workloads-mixed-user",
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

	// Create a user session
	imp := user.Impersonation{
		Username: "workloads-mixed-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Mixed User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadsStatus with mixed scenarios:
	// 1. Workload that exists and user has access
	// 2. Workload that doesn't exist
	workloads := []WorkloadItem{
		{Kind: "Deployment", Namespace: "default", Name: "test-workload-mixed-exists"},
		{Kind: "Deployment", Namespace: "default", Name: "nonexistent"},
	}
	results := handler.GetWorkloadsStatus(userCtx, workloads)

	// Should return results for all 2 items with appropriate messages
	g.Expect(results).To(HaveLen(2))

	// 1. Existing workload with access - should succeed
	g.Expect(results[0].Name).To(Equal("test-workload-mixed-exists"))
	g.Expect(results[0].Status).NotTo(Equal("NotFound"))

	// 2. Non-existent workload - should show not found message
	g.Expect(results[1].Name).To(Equal("nonexistent"))
	g.Expect(results[1].Status).To(Equal("NotFound"))
	g.Expect(results[1].StatusMessage).To(Equal("Workload not found in the cluster"))
}

func TestGetWorkloadsStatus_EmptyList(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadsStatus with empty list
	workloads := []WorkloadItem{}
	results := handler.GetWorkloadsStatus(ctx, workloads)

	// Should return empty result
	g.Expect(results).To(BeEmpty())
}

func TestGetWorkloadsStatus_StatefulSet(t *testing.T) {
	g := NewWithT(t)

	// Create a StatefulSet for testing
	replicas := int32(1)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-statefulset"},
			},
			ServiceName: "test-statefulset",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-statefulset"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, statefulSet)).To(Succeed())
	defer testClient.Delete(ctx, statefulSet)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadsStatus for StatefulSet
	workloads := []WorkloadItem{
		{Kind: "StatefulSet", Namespace: "default", Name: "test-statefulset"},
	}
	results := handler.GetWorkloadsStatus(ctx, workloads)

	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-statefulset"))
	g.Expect(results[0].Kind).To(Equal("StatefulSet"))
	g.Expect(results[0].Status).NotTo(Equal("NotFound"))
}

func TestGetWorkloadsStatus_DaemonSet(t *testing.T) {
	g := NewWithT(t)

	// Create a DaemonSet for testing
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-daemonset",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-daemonset"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-daemonset"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, daemonSet)).To(Succeed())
	defer testClient.Delete(ctx, daemonSet)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadsStatus for DaemonSet
	workloads := []WorkloadItem{
		{Kind: "DaemonSet", Namespace: "default", Name: "test-daemonset"},
	}
	results := handler.GetWorkloadsStatus(ctx, workloads)

	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("test-daemonset"))
	g.Expect(results[0].Kind).To(Equal("DaemonSet"))
	g.Expect(results[0].Status).NotTo(Equal("NotFound"))
}

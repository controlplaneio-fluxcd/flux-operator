// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetWorkloadStatus_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-priv",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-priv"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-priv"},
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

	// Call GetWorkloadStatus without any user session (privileged)
	workload, err := handler.GetWorkloadStatus(ctx, "Deployment", "test-workload-priv", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Kind).To(Equal("Deployment"))
	g.Expect(workload.Name).To(Equal("test-workload-priv"))
	g.Expect(workload.Namespace).To(Equal("default"))
	g.Expect(workload.ContainerImages).To(ContainElement("nginx:latest"))
}

func TestGetWorkloadStatus_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-unpriv",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-unpriv"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-unpriv"},
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
		Username: "unprivileged-workload-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadStatus with the unprivileged user context
	_, err = handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-unpriv", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error, got: %v", err)
}

func TestGetWorkloadStatus_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-rbac",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-rbac"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-rbac"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25",
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
			Name:      "test-workload-status-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
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
			Name:      "test-workload-status-reader-binding",
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
				Name: "workload-status-user",
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
		Username: "workload-status-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Workload Status User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadStatus with the user context
	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-rbac", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Name).To(Equal("test-workload-rbac"))
	g.Expect(workload.ContainerImages).To(ContainElement("nginx:1.25"))
}

func TestGetWorkloadStatus_WithGroupRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-group-rbac",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-group-rbac"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-group-rbac"},
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

	// Create RBAC for the test group
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-group-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
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
			Name:      "test-workload-group-reader-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "Group",
				Name: "workload-readers",
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

	// Create a user session with group membership
	imp := user.Impersonation{
		Username: "workload-group-user",
		Groups:   []string{"workload-readers"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Workload Group User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadStatus with the user context
	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-group-rbac", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Name).To(Equal("test-workload-group-rbac"))
}

func TestGetWorkloadStatus_WithNamespaceScopedRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing in default namespace
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ns-scoped",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ns-scoped"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ns-scoped"},
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
			Name:      "test-workload-ns-scoped-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
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
			Name:      "test-workload-ns-scoped-reader-binding",
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
				Name: "workload-ns-scoped-user",
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
		Username: "workload-ns-scoped-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "NS Scoped User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadStatus in default namespace - should succeed
	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-ns-scoped", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Name).To(Equal("test-workload-ns-scoped"))
}

func TestGetWorkloadStatus_WithNamespaceScopedRBAC_ForbiddenInOtherNamespace(t *testing.T) {
	g := NewWithT(t)

	// Create a unique namespace for this test
	otherNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "workload-other-ns-test",
		},
	}
	g.Expect(testClient.Create(ctx, otherNS)).To(Succeed())
	defer testClient.Delete(ctx, otherNS)

	// Create a Deployment in the other namespace
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-other-ns",
			Namespace: "workload-other-ns-test",
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

	// Create RBAC for the test user with access only in the default namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-default-only-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
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
			Name:      "test-workload-default-only-reader-binding",
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
				Name: "workload-default-only-user",
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

	// Create a user session with access only in default namespace
	imp := user.Impersonation{
		Username: "workload-default-only-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Default Only User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadStatus in other namespace - should be forbidden
	_, err = handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-other-ns", "workload-other-ns-test")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error when accessing workload in unauthorized namespace, got: %v", err)
}

func TestGetWorkloadStatus_WithDeploymentAccessButNoPodAccess_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-no-pods",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-no-pods"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-no-pods"},
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
			Name:      "test-workload-no-pods-reader",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list"},
			},
			// Note: No pod access here
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-no-pods-reader-binding",
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
				Name: "workload-no-pods-user",
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
		Username: "workload-no-pods-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "No Pods Access User"},
		Impersonation: imp,
	}, userClient)

	// Call GetWorkloadStatus - user can get deployment but not list pods
	_, err = handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-no-pods", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error when user cannot list pods, got: %v", err)
}

func TestGetWorkloadStatus_NotFound(t *testing.T) {
	g := NewWithT(t)

	// Create the handler
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadStatus for a non-existent deployment
	_, err := handler.GetWorkloadStatus(ctx, "Deployment", "non-existent-deployment", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected not found error, got: %v", err)
}

func TestGetWorkloadStatus_StatefulSet(t *testing.T) {
	g := NewWithT(t)

	// Create a StatefulSet for testing
	replicas := int32(1)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset-status",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-statefulset-status"},
			},
			ServiceName: "test-statefulset-status",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-statefulset-status"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7",
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

	// Call GetWorkloadStatus for StatefulSet
	workload, err := handler.GetWorkloadStatus(ctx, "StatefulSet", "test-statefulset-status", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Kind).To(Equal("StatefulSet"))
	g.Expect(workload.Name).To(Equal("test-statefulset-status"))
	g.Expect(workload.ContainerImages).To(ContainElement("redis:7"))
}

func TestGetWorkloadStatus_DaemonSet(t *testing.T) {
	g := NewWithT(t)

	// Create a DaemonSet for testing
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-daemonset-status",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-daemonset-status"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-daemonset-status"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "fluentd",
							Image: "fluentd:v1.16",
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

	// Call GetWorkloadStatus for DaemonSet
	workload, err := handler.GetWorkloadStatus(ctx, "DaemonSet", "test-daemonset-status", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Kind).To(Equal("DaemonSet"))
	g.Expect(workload.Name).To(Equal("test-daemonset-status"))
	g.Expect(workload.ContainerImages).To(ContainElement("fluentd:v1.16"))
}

func TestGetWorkloadStatus_CronJob(t *testing.T) {
	// Create the handler with kubeClientCache which has the field index for Jobs
	handler := &Handler{
		kubeClient:    kubeClientCache,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	t.Run("idle cronjob without active jobs", func(t *testing.T) {
		g := NewWithT(t)

		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cronjob-idle",
				Namespace: "default",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "*/5 * * * *",
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:    "backup",
										Image:   "busybox:1.36",
										Command: []string{"/bin/sh", "-c", "echo hello"},
									},
								},
							},
						},
					},
				},
			},
		}
		g.Expect(testClient.Create(ctx, cronJob)).To(Succeed())
		defer testClient.Delete(ctx, cronJob)

		workload, err := handler.GetWorkloadStatus(ctx, "CronJob", "test-cronjob-idle", "default")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload).NotTo(BeNil())
		g.Expect(workload.Kind).To(Equal("CronJob"))
		g.Expect(workload.Name).To(Equal("test-cronjob-idle"))
		g.Expect(workload.ContainerImages).To(ContainElement("busybox:1.36"))
		g.Expect(workload.Status).To(Equal("Idle"))
		g.Expect(workload.StatusMessage).To(Equal("*/5 * * * *"))
	})

	t.Run("suspended cronjob", func(t *testing.T) {
		g := NewWithT(t)

		suspended := true
		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cronjob-suspended",
				Namespace: "default",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "0 0 * * *",
				Suspend:  &suspended,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:    "backup",
										Image:   "busybox:1.36",
										Command: []string{"/bin/sh", "-c", "echo hello"},
									},
								},
							},
						},
					},
				},
			},
		}
		g.Expect(testClient.Create(ctx, cronJob)).To(Succeed())
		defer testClient.Delete(ctx, cronJob)

		workload, err := handler.GetWorkloadStatus(ctx, "CronJob", "test-cronjob-suspended", "default")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload).NotTo(BeNil())
		g.Expect(workload.Status).To(Equal("Suspended"))
		g.Expect(workload.StatusMessage).To(Equal("CronJob is suspended"))
	})

	t.Run("cronjob with active jobs", func(t *testing.T) {
		g := NewWithT(t)

		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cronjob-active",
				Namespace: "default",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "*/10 * * * *",
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								RestartPolicy: corev1.RestartPolicyOnFailure,
								Containers: []corev1.Container{
									{
										Name:    "worker",
										Image:   "busybox:1.36",
										Command: []string{"/bin/sh", "-c", "sleep 60"},
									},
								},
							},
						},
					},
				},
			},
		}
		g.Expect(testClient.Create(ctx, cronJob)).To(Succeed())
		defer testClient.Delete(ctx, cronJob)

		// Update status separately (status is a subresource)
		cronJob.Status.Active = []corev1.ObjectReference{
			{Name: "test-cronjob-active-12345", Namespace: "default"},
		}
		g.Expect(testClient.Status().Update(ctx, cronJob)).To(Succeed())

		workload, err := handler.GetWorkloadStatus(ctx, "CronJob", "test-cronjob-active", "default")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload).NotTo(BeNil())
		g.Expect(workload.Status).To(Equal("Progressing"))
		g.Expect(workload.StatusMessage).To(Equal("Active jobs: 1"))
	})
}

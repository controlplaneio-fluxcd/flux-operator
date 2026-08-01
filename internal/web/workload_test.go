// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestGetWorkloadStatus_Privileged(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-priv",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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

	// Create the handler without auth (no user actions)
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call GetWorkloadStatus without any user session (privileged)
	workload, err := handler.GetWorkloadStatus(ctx, "Deployment", "test-workload-priv", "default", false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Kind).To(Equal("Deployment"))
	g.Expect(workload.Name).To(Equal("test-workload-priv"))
	g.Expect(workload.Namespace).To(Equal("default"))
	g.Expect(workload.ContainerImages).To(ContainElement("nginx:latest"))
	// Without auth configured, UserActions should be empty
	g.Expect(workload.UserActions).To(BeEmpty())
}

func TestGetWorkloadStatus_Lightweight_SkipsPodsAndActions(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-lightweight",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-lightweight"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-lightweight"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create a pod managed by the deployment (envtest has no controllers, so we
	// create it explicitly) so a detailed call has a pod to list and the
	// lightweight call can be shown to skip it.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-lightweight-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "test-workload-lightweight"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "test-workload-lightweight-abc123",
					UID:        "test-lightweight-uid",
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, pod)).To(Succeed())
	defer testClient.Delete(ctx, pod)

	// RBAC granting the user restart on deployments and full pod access, so a
	// detailed call would populate both UserActions and Pods.
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lightweight-role",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "restart"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "delete"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lightweight-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "lightweight-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "lightweight-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Lightweight User"},
		Impersonation: imp,
	}, userClient)

	// Lightweight (detailed=false): the status is computed but the pod listing
	// and the per-action RBAC checks are skipped.
	lightweight, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-lightweight", "default", false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(lightweight.Status).NotTo(BeEmpty())
	g.Expect(lightweight.Pods).To(BeEmpty())
	g.Expect(lightweight.UserActions).To(BeEmpty())

	// Detailed (detailed=true): the managed pods and the user actions are computed.
	detailed, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-lightweight", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(detailed.Pods).To(HaveLen(1))
	g.Expect(detailed.Pods[0].Name).To(Equal("test-workload-lightweight-pod"))
	g.Expect(detailed.UserActions).To(ContainElement(fluxcdv1.UserActionRestart))
}

func TestGetWorkloadStatus_SamplesOnlyOnBatchPath(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment whose selector matches the buffered pod below
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-batch-samples",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-batch-samples"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-batch-samples"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	mc := NewMetricsCollector(nil, time.Minute)
	list := &metricsv1beta1api.PodMetricsList{
		Items: []metricsv1beta1api.PodMetrics{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-workload-batch-samples-pod",
					Namespace: "default",
					Labels:    map[string]string{"app": "test-workload-batch-samples"},
				},
				Containers: []metricsv1beta1api.ContainerMetrics{
					{
						Name: "nginx",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(64<<20, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	mc.ingest(list, time.Now().Truncate(time.Second))

	handler := &Handler{
		kubeClient:    kubeClient,
		metrics:       mc,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// The batch path (detailed=false) carries the top-level series.
	batch, err := handler.GetWorkloadStatus(ctx, "Deployment", "test-workload-batch-samples", "default", false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(batch.Samples).To(HaveLen(1))

	// The detail path (detailed=true) returns the series in metrics.samples
	// via buildWorkloadMetrics and must not duplicate it at the top level.
	detailed, err := handler.GetWorkloadStatus(ctx, "Deployment", "test-workload-batch-samples", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(detailed.Samples).To(BeEmpty())
}

func TestGetWorkloadStatus_UnprivilegedUser_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-unpriv",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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
	_, err = handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-unpriv", "default", false)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error, got: %v", err)
}

func TestGetWorkloadStatus_WithUserRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-rbac",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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
	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-rbac", "default", false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Name).To(Equal("test-workload-rbac"))
	g.Expect(workload.ContainerImages).To(ContainElement("nginx:1.25"))
}

func TestGetWorkloadStatus_WithGroupRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-group-rbac",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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
	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-group-rbac", "default", false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.Name).To(Equal("test-workload-group-rbac"))
}

func TestGetWorkloadStatus_WithNamespaceScopedRBAC_Success(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing in default namespace
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ns-scoped",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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
	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-ns-scoped", "default", false)
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
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-other-ns",
			Namespace: "workload-other-ns-test",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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
	_, err = handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-other-ns", "workload-other-ns-test", false)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue(), "expected forbidden error when accessing workload in unauthorized namespace, got: %v", err)
}

func TestGetWorkloadStatus_WithDeploymentAccessButNoPodAccess_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-no-pods",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
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
	_, err = handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-no-pods", "default", true)
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
	_, err := handler.GetWorkloadStatus(ctx, "Deployment", "non-existent-deployment", "default", false)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected not found error, got: %v", err)
}

func TestGetWorkloadStatus_StatefulSet(t *testing.T) {
	g := NewWithT(t)

	// Create a StatefulSet for testing
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-statefulset-status",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(int32(1)),
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
	workload, err := handler.GetWorkloadStatus(ctx, "StatefulSet", "test-statefulset-status", "default", false)
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
	workload, err := handler.GetWorkloadStatus(ctx, "DaemonSet", "test-daemonset-status", "default", false)
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

		workload, err := handler.GetWorkloadStatus(ctx, "CronJob", "test-cronjob-idle", "default", false)
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

		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cronjob-suspended",
				Namespace: "default",
			},
			Spec: batchv1.CronJobSpec{
				Schedule: "0 0 * * *",
				Suspend:  new(true),
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

		workload, err := handler.GetWorkloadStatus(ctx, "CronJob", "test-cronjob-suspended", "default", false)
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

		workload, err := handler.GetWorkloadStatus(ctx, "CronJob", "test-cronjob-active", "default", false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(workload).NotTo(BeNil())
		g.Expect(workload.Status).To(Equal("Progressing"))
		g.Expect(workload.StatusMessage).To(Equal("Active jobs: 1"))
	})
}

func TestGetWorkloadStatus_UserActions_WithRestartAndDeletePods(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-both",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-both"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-both"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC for the test user: restart on deployments + delete on pods + get/list
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-both-role",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "restart"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "delete"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-both-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-both-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-both-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA Both User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-ua-both", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionRestart))
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionDeletePods))
}

func TestGetWorkloadStatus_UserActions_RestartOnly(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-restart",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-restart"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-restart"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC: restart on deployments + get/list pods (but NOT delete pods)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-restart-role",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "restart"},
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
			Name:      "test-ua-restart-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-restart-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-restart-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA Restart User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-ua-restart", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionRestart))
	g.Expect(workload.UserActions).NotTo(ContainElement(fluxcdv1.UserActionDeletePods))
}

func TestGetWorkloadStatus_UserActions_DeletePodsOnly(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-delpods",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-delpods"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-delpods"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC: get/list deployments + delete pods (but NOT restart)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-delpods-role",
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
				Verbs:     []string{"get", "list", "delete"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-delpods-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-delpods-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-delpods-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA DeletePods User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-ua-delpods", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).NotTo(ContainElement(fluxcdv1.UserActionRestart))
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionDeletePods))
}

func TestGetWorkloadStatus_UserActions_NoActions(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-none",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-none"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-none"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Create RBAC: only get/list, no restart or delete
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-none-role",
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
			Name:      "test-ua-none-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-none-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-none-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA No Actions User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "Deployment", "test-workload-ua-none", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).To(BeEmpty())
}

func TestGetWorkloadStatus_UserActions_DisabledWithoutAuth(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment for testing
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-disabled",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-disabled"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-disabled"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer testClient.Delete(ctx, deployment)

	// Handler without authentication configured (UserActions should be disabled)
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Call with privileged context (system:masters has all permissions)
	workload, err := handler.GetWorkloadStatus(ctx, "Deployment", "test-workload-ua-disabled", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	// Even though the privileged user has all permissions, UserActions should be empty
	// because authentication is not configured
	g.Expect(workload.UserActions).To(BeEmpty())
}

func TestGetWorkloadStatus_UserActions_StatefulSet(t *testing.T) {
	g := NewWithT(t)

	// Create a StatefulSet for testing
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-sts"},
			},
			ServiceName: "test-workload-ua-sts",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-sts"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "redis", Image: "redis:7"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, statefulSet)).To(Succeed())
	defer testClient.Delete(ctx, statefulSet)

	// Create RBAC: restart on statefulsets + delete pods
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-sts-role",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"get", "list", "restart"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "delete"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-sts-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-sts-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-sts-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA StatefulSet User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "StatefulSet", "test-workload-ua-sts", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionRestart))
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionDeletePods))
}

func TestGetWorkloadStatus_UserActions_CronJob(t *testing.T) {
	g := NewWithT(t)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-cron",
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
									Name:    "worker",
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

	// Create RBAC: restart on cronjobs + delete pods
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-cron-role",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"batch"},
				Resources: []string{"cronjobs"},
				Verbs:     []string{"get", "list", "restart"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "delete"},
			},
		},
	}
	g.Expect(testClient.Create(ctx, role)).To(Succeed())
	defer testClient.Delete(ctx, role)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-cron-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-cron-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClientCache,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-cron-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA CronJob User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "CronJob", "test-workload-ua-cron", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionRestart))
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionDeletePods))
}

func TestGetWorkloadStatus_UserActions_DaemonSet(t *testing.T) {
	g := NewWithT(t)

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload-ua-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-workload-ua-ds"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-workload-ua-ds"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "fluentd", Image: "fluentd:v1.16"},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, daemonSet)).To(Succeed())
	defer testClient.Delete(ctx, daemonSet)

	// Create RBAC: restart on daemonsets, no delete pods
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ua-ds-role",
			Namespace: "default",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets"},
				Verbs:     []string{"get", "list", "restart"},
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
			Name:      "test-ua-ds-binding",
			Namespace: "default",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "ua-ds-user"},
		},
	}
	g.Expect(testClient.Create(ctx, roleBinding)).To(Succeed())
	defer testClient.Delete(ctx, roleBinding)

	handler := &Handler{
		conf:          oauthConfig(),
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	imp := user.Impersonation{
		Username: "ua-ds-user",
		Groups:   []string{"system:authenticated"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "UA DaemonSet User"},
		Impersonation: imp,
	}, userClient)

	workload, err := handler.GetWorkloadStatus(userCtx, "DaemonSet", "test-workload-ua-ds", "default", true)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(workload).NotTo(BeNil())
	g.Expect(workload.UserActions).To(ContainElement(fluxcdv1.UserActionRestart))
	g.Expect(workload.UserActions).NotTo(ContainElement(fluxcdv1.UserActionDeletePods))
}

func TestWorkloadHandler_Success_WithParentReconciler(t *testing.T) {
	g := NewWithT(t)

	// Create a ResourceSet as the parent reconciler
	resourceSet := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wh-parent",
			Namespace: "default",
		},
		Spec: fluxcdv1.ResourceSetSpec{
			ResourcesTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\n",
		},
	}
	g.Expect(testClient.Create(ctx, resourceSet)).To(Succeed())
	defer func() { g.Expect(testClient.Delete(ctx, resourceSet)).To(Succeed()) }()

	// Create a Deployment with Flux labels pointing to the parent ResourceSet
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wh-deploy",
			Namespace: "default",
			Labels: map[string]string{
				"app":                                "test-wh-deploy",
				"resourceset.toolkit.fluxcd.io/name": "test-wh-parent",
				"resourceset.toolkit.fluxcd.io/namespace": "default",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-wh-deploy"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-wh-deploy"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25.0",
						},
					},
				},
			},
		},
	}
	g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
	defer func() { g.Expect(testClient.Delete(ctx, deployment)).To(Succeed()) }()

	// Managed pods: one with partial requests/limits, one without any.
	makePod := func(name string, resources corev1.ResourceRequirements) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{"app": "test-wh-deploy"},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "test-wh-deploy-abc123",
						UID:        "test-wh-uid",
					},
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "nginx", Image: "nginx:1.25.0", Resources: resources},
				},
			},
		}
	}
	podWithResources := makePod("test-wh-deploy-pod-a", corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	})
	g.Expect(testClient.Create(ctx, podWithResources)).To(Succeed())
	defer testClient.Delete(ctx, podWithResources)

	podWithoutResources := makePod("test-wh-deploy-pod-b", corev1.ResourceRequirements{})
	g.Expect(testClient.Create(ctx, podWithoutResources)).To(Succeed())
	defer testClient.Delete(ctx, podWithoutResources)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload?kind=Deployment&name=test-wh-deploy&namespace=default", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

	// Decode response
	var result map[string]any
	err := json.NewDecoder(rec.Body).Decode(&result)
	g.Expect(err).NotTo(HaveOccurred())

	// Verify workload fields
	g.Expect(result["apiVersion"]).To(Equal("apps/v1"))
	g.Expect(result["kind"]).To(Equal("Deployment"))

	metadata, ok := result["metadata"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(metadata["name"]).To(Equal("test-wh-deploy"))
	g.Expect(metadata["namespace"]).To(Equal("default"))

	// Verify workloadInfo
	workloadInfo, ok := result["workloadInfo"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "workloadInfo should be present")
	g.Expect(workloadInfo["status"]).NotTo(BeEmpty())
	g.Expect(workloadInfo["createdAt"]).NotTo(BeEmpty())
	g.Expect(workloadInfo["containerImages"]).To(ContainElement("nginx:1.25.0"))

	// Verify reconciler is present and is the parent ResourceSet
	reconciler, ok := workloadInfo["reconciler"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "reconciler should be present in workloadInfo")
	g.Expect(reconciler["kind"]).To(Equal("ResourceSet"))

	reconcilerMeta, ok := reconciler["metadata"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(reconcilerMeta["name"]).To(Equal("test-wh-parent"))
	g.Expect(reconcilerMeta["namespace"]).To(Equal("default"))

	// Verify the per-pod resources serialization: exact JSON field names,
	// unset fields omitted, and the object absent when the pod spec sets
	// no requests/limits at all. Pods are sorted by name.
	pods, ok := workloadInfo["pods"].([]any)
	g.Expect(ok).To(BeTrue(), "pods should be present in workloadInfo")
	g.Expect(pods).To(HaveLen(2))

	podA, ok := pods[0].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(podA["name"]).To(Equal("test-wh-deploy-pod-a"))
	resources, ok := podA["resources"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "resources should be present for the pod with requests/limits")
	g.Expect(resources).To(HaveKeyWithValue("cpuRequests", BeNumerically("~", 0.1, 1e-9)))
	g.Expect(resources).To(HaveKeyWithValue("memoryRequests", BeNumerically("==", 64<<20)))
	g.Expect(resources).To(HaveKeyWithValue("memoryLimits", BeNumerically("==", 128<<20)))
	g.Expect(resources).NotTo(HaveKey("cpuLimits"))

	podB, ok := pods[1].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(podB["name"]).To(Equal("test-wh-deploy-pod-b"))
	g.Expect(podB).NotTo(HaveKey("resources"))
}

func TestWorkloadHandler_NotFluxManaged_ReturnsError(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment WITHOUT Flux labels
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wh-no-flux",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-wh-no-flux"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-wh-no-flux"},
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
	defer func() { g.Expect(testClient.Delete(ctx, deployment)).To(Succeed()) }()

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload?kind=Deployment&name=test-wh-no-flux&namespace=default", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadHandler(rec, req)

	// Not Flux-managed workloads return 500 with error
	g.Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	g.Expect(rec.Body.String()).To(ContainSubstring("not managed by a Flux reconciler"))
}

func TestWorkloadHandler_NotFound_ReturnsEmptyJSON(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload?kind=Deployment&name=non-existent-wh&namespace=default", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.WorkloadHandler(rec, req)

	// Not found returns 200 with empty JSON
	g.Expect(rec.Code).To(Equal(http.StatusOK))
	g.Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
	g.Expect(rec.Body.String()).To(Equal("{}"))
}

func TestWorkloadHandler_Forbidden(t *testing.T) {
	g := NewWithT(t)

	// Create a Deployment with Flux labels
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-wh-forbidden",
			Namespace: "default",
			Labels: map[string]string{
				"app":                                "test-wh-forbidden",
				"resourceset.toolkit.fluxcd.io/name": "some-parent",
				"resourceset.toolkit.fluxcd.io/namespace": "default",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-wh-forbidden"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-wh-forbidden"},
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
	defer func() { g.Expect(testClient.Delete(ctx, deployment)).To(Succeed()) }()

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	// Create an unprivileged user session
	imp := user.Impersonation{
		Username: "unprivileged-wh-user",
		Groups:   []string{"unprivileged-group"},
	}
	userClient, err := kubeClient.GetUserClientFromCache(imp)
	g.Expect(err).NotTo(HaveOccurred())

	userCtx := user.StoreSession(ctx, user.Details{
		Profile:       user.Profile{Name: "Unprivileged User"},
		Impersonation: imp,
	}, userClient)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workload?kind=Deployment&name=test-wh-forbidden&namespace=default", nil)
	req = req.WithContext(userCtx)
	rec := httptest.NewRecorder()

	handler.WorkloadHandler(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusForbidden))
	g.Expect(rec.Body.String()).To(ContainSubstring("do not have access"))
}

func TestWorkloadHandler_MissingParameters(t *testing.T) {
	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	testCases := []struct {
		name  string
		query string
	}{
		{"missing all", ""},
		{"missing kind", "name=test&namespace=default"},
		{"missing name", "kind=Deployment&namespace=default"},
		{"missing namespace", "kind=Deployment&name=test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/workload?"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.WorkloadHandler(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusBadRequest))
			g.Expect(rec.Body.String()).To(ContainSubstring("Missing required parameters"))
		})
	}
}

func TestGetRolledOutAt(t *testing.T) {
	g := NewWithT(t)

	handler := &Handler{kubeClient: kubeClient}

	// getUnstructured fetches a workload the way GetWorkloadStatus does.
	getUnstructured := func(kind, name string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(getWorkloadGVK(kind))
		g.Expect(testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "default"}, obj)).To(Succeed())
		return obj
	}

	podTemplate := func(app string) corev1.PodTemplateSpec {
		return corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": app}},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
			},
		}
	}

	t.Run("Deployment newest ReplicaSet by revision", func(t *testing.T) {
		g := NewWithT(t)

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rolledout-deploy", Namespace: "default"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-rolledout-deploy"}},
				Template: podTemplate("test-rolledout-deploy"),
			},
		}
		g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
		defer testClient.Delete(ctx, deployment)

		ownerRef := metav1.OwnerReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       deployment.Name,
			UID:        deployment.UID,
		}
		var newest *appsv1.ReplicaSet
		for i, revision := range []string{"1", "2"} {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            deployment.Name + []string{"-aaaaaaaaaa", "-bbbbbbbbbb"}[i],
					Namespace:       "default",
					Annotations:     map[string]string{"deployment.kubernetes.io/revision": revision},
					OwnerReferences: []metav1.OwnerReference{ownerRef},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-rolledout-deploy"}},
					Template: podTemplate("test-rolledout-deploy"),
				},
			}
			g.Expect(testClient.Create(ctx, rs)).To(Succeed())
			defer testClient.Delete(ctx, rs)
			newest = rs
		}

		// A same-prefix ReplicaSet with a higher revision that is NOT
		// owned by the Deployment must be ignored: pod references
		// cannot probe sibling objects through the privileged reader.
		forged := &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        deployment.Name + "-cccccccccc",
				Namespace:   "default",
				Annotations: map[string]string{"deployment.kubernetes.io/revision": "99"},
			},
			Spec: appsv1.ReplicaSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-rolledout-deploy"}},
				Template: podTemplate("test-rolledout-deploy"),
			},
		}
		g.Expect(testClient.Create(ctx, forged)).To(Succeed())
		defer testClient.Delete(ctx, forged)

		// Mid-rollout, the pods span both generations; the newest
		// owned revision wins. The ReplicaSet names come from the
		// pods' owner references.
		pods := []WorkloadPodStatus{
			{Name: "old-pod", revisionRef: deployment.Name + "-aaaaaaaaaa"},
			{Name: "new-pod", revisionRef: deployment.Name + "-bbbbbbbbbb"},
			{Name: "forged-pod", revisionRef: forged.Name},
		}

		obj := getUnstructured("Deployment", deployment.Name)
		g.Expect(handler.getRolledOutAt(ctx, obj, pods)).
			To(Equal(newest.CreationTimestamp.UTC().Format(time.RFC3339)))
	})

	t.Run("StatefulSet revision from status", func(t *testing.T) {
		g := NewWithT(t)

		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rolledout-sts", Namespace: "default"},
			Spec: appsv1.StatefulSetSpec{
				ServiceName: "test-rolledout-sts",
				Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-rolledout-sts"}},
				Template:    podTemplate("test-rolledout-sts"),
			},
		}
		g.Expect(testClient.Create(ctx, sts)).To(Succeed())
		defer testClient.Delete(ctx, sts)

		cr := &appsv1.ControllerRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rolledout-sts-5d4f8b9c7",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       sts.Name,
					UID:        sts.UID,
				}},
			},
			Data:     runtime.RawExtension{Raw: []byte("{}")},
			Revision: 1,
		}
		g.Expect(testClient.Create(ctx, cr)).To(Succeed())
		defer testClient.Delete(ctx, cr)

		sts.Status.UpdateRevision = cr.Name
		g.Expect(testClient.Status().Update(ctx, sts)).To(Succeed())

		// The revision is resolved from the status, no pods needed.
		obj := getUnstructured("StatefulSet", sts.Name)
		g.Expect(handler.getRolledOutAt(ctx, obj, nil)).
			To(Equal(cr.CreationTimestamp.UTC().Format(time.RFC3339)))
	})

	t.Run("DaemonSet newest ControllerRevision", func(t *testing.T) {
		g := NewWithT(t)

		ds := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rolledout-ds", Namespace: "default"},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-rolledout-ds"}},
				Template: podTemplate("test-rolledout-ds"),
			},
		}
		g.Expect(testClient.Create(ctx, ds)).To(Succeed())
		defer testClient.Delete(ctx, ds)

		cr := &appsv1.ControllerRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rolledout-ds-6c8d7f9b5",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "DaemonSet",
					Name:       ds.Name,
					UID:        ds.UID,
				}},
			},
			Data:     runtime.RawExtension{Raw: []byte("{}")},
			Revision: 1,
		}
		g.Expect(testClient.Create(ctx, cr)).To(Succeed())
		defer testClient.Delete(ctx, cr)

		// The ControllerRevision name is derived in GetWorkloadPods from
		// the pods' controller-revision-hash label.
		pods := []WorkloadPodStatus{{Name: "ds-pod", revisionRef: cr.Name}}

		obj := getUnstructured("DaemonSet", ds.Name)
		g.Expect(handler.getRolledOutAt(ctx, obj, pods)).
			To(Equal(cr.CreationTimestamp.UTC().Format(time.RFC3339)))
	})

	t.Run("No generation info yields empty", func(t *testing.T) {
		g := NewWithT(t)

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rolledout-none", Namespace: "default"},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-rolledout-none"}},
				Template: podTemplate("test-rolledout-none"),
			},
		}
		g.Expect(testClient.Create(ctx, deployment)).To(Succeed())
		defer testClient.Delete(ctx, deployment)

		obj := getUnstructured("Deployment", deployment.Name)

		// No pods means no generation references.
		g.Expect(handler.getRolledOutAt(ctx, obj, nil)).To(Equal(""))

		// A pod referencing a deleted ReplicaSet resolves to nothing.
		pods := []WorkloadPodStatus{{Name: "orphan", revisionRef: "test-rolledout-none-gone"}}
		g.Expect(handler.getRolledOutAt(ctx, obj, pods)).To(Equal(""))

		// CronJobs have no chartable generation.
		cronObj := &unstructured.Unstructured{}
		cronObj.SetGroupVersionKind(getWorkloadGVK("CronJob"))
		g.Expect(handler.getRolledOutAt(ctx, cronObj, nil)).To(Equal(""))
	})
}

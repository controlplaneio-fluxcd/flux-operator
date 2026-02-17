// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetComponentsStatus_Success(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())

	deploy1 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-controller",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "source-controller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "source-controller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "manager",
							Image: "ghcr.io/fluxcd/source-controller:v1.4.0",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
			Replicas:      1,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	deploy2 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kustomize-controller",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "kustomize-controller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "kustomize-controller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "manager",
							Image: "ghcr.io/fluxcd/kustomize-controller:v1.4.0",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
			Replicas:      1,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// The components method uses unstructured list, so we need a client
	// that can serve unstructured objects from typed objects.
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(deploy1, deploy2).
		Build()
	r := NewFluxStatusReporter(kubeClient, "flux", "flux-operator", "flux-system")

	components, err := r.getComponentsStatus(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(components).To(HaveLen(2))

	// Verify sorting by name.
	g.Expect(components[0].Name).To(Equal("kustomize-controller"))
	g.Expect(components[1].Name).To(Equal("source-controller"))

	// Verify image extraction.
	g.Expect(components[0].Image).To(Equal("ghcr.io/fluxcd/kustomize-controller:v1.4.0"))
	g.Expect(components[1].Image).To(Equal("ghcr.io/fluxcd/source-controller:v1.4.0"))
}

func TestGetComponentsStatus_NoDeployments(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	r := NewFluxStatusReporter(kubeClient, "flux", "flux-operator", "flux-system")

	components, err := r.getComponentsStatus(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(components).To(BeEmpty())
}

func TestGetComponentsStatus_LabelFiltering(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())

	// Matching deployment.
	matching := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-controller",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "source-controller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "source-controller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "manager", Image: "ghcr.io/fluxcd/source-controller:v1.4.0"},
					},
				},
			},
		},
	}

	// Non-matching deployment (different label).
	nonMatching := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-app",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "other",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "other-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "other-app"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "manager", Image: "ghcr.io/other/app:v1.0.0"},
					},
				},
			},
		},
	}

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(matching, nonMatching).
		Build()

	// Verify two deployments exist in the namespace.
	var allDeploys appsv1.DeploymentList
	g.Expect(kubeClient.List(ctx, &allDeploys, client.InNamespace("flux-system"))).To(Succeed())
	g.Expect(allDeploys.Items).To(HaveLen(2))

	r := NewFluxStatusReporter(kubeClient, "flux", "flux-operator", "flux-system")
	components, err := r.getComponentsStatus(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(components).To(HaveLen(1))
	g.Expect(components[0].Name).To(Equal("source-controller"))
}

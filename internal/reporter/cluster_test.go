// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestCountNodes_ListsMetadataOnly(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	var listTypes []string
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
		).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				listTypes = append(listTypes, fmt.Sprintf("%T %s", list, list.GetObjectKind().GroupVersionKind().Kind))
				return c.List(ctx, list, opts...)
			},
		}).
		Build()
	r := NewFluxStatusReporter(kubeClient, "flux", "flux-operator", "flux-system")

	count, err := r.countNodes(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(count).To(Equal(2))

	// The nodes must be listed as metadata, a typed list would make the
	// operator cache full Node objects.
	g.Expect(listTypes).To(Equal([]string{"*v1.PartialObjectMetadataList NodeList"}))
}

func TestCountNodes_Error(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return fmt.Errorf("nodes unavailable")
			},
		}).
		Build()
	r := NewFluxStatusReporter(kubeClient, "flux", "flux-operator", "flux-system")

	_, err := r.countNodes(ctx)
	g.Expect(err).To(MatchError(ContainSubstring("failed to list nodes: nodes unavailable")))
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetEvents(t *testing.T) {
	mockEventList := &corev1.EventList{
		Items: []corev1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event1",
					Namespace: "flux-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "fluxcd.controlplane.io/v1",
					Kind:       "FluxInstance",
					Name:       "flux",
					Namespace:  "flux-system",
				},
				Type:    corev1.EventTypeWarning,
				Reason:  "ReconciliationFailed",
				Message: "Reconciliation failed with unknown version",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event2",
					Namespace: "flux-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "fluxcd.controlplane.io/v1",
					Kind:       "FluxInstance",
					Name:       "flux",
					Namespace:  "flux-system",
				},
				Type:    corev1.EventTypeWarning,
				Reason:  "ReconciliationFailed",
				Message: "Reconciliation failed with unknown distribution",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "event3",
					Namespace: "flux-system",
				},
				InvolvedObject: corev1.ObjectReference{
					APIVersion: "fluxcd.controlplane.io/v1",
					Kind:       "ResourceSet",
					Name:       "infra",
					Namespace:  "flux-system",
				},
				Type:    corev1.EventTypeNormal,
				Reason:  "ReconciliationSucceeded",
				Message: "Reconciliation succeeded with version 1.0.0",
			},
		},
	}

	objKindIndexer := func(obj client.Object) []string {
		e, _ := obj.(*corev1.Event)
		return []string{e.InvolvedObject.Kind}
	}

	objNameIndexer := func(obj client.Object) []string {
		e, _ := obj.(*corev1.Event)
		return []string{e.InvolvedObject.Name}
	}

	reasonIndexer := func(obj client.Object) []string {
		e, _ := obj.(*corev1.Event)
		return []string{e.Reason}
	}

	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithLists(mockEventList).
			WithIndex(&corev1.Event{}, "involvedObject.kind", objKindIndexer).
			WithIndex(&corev1.Event{}, "involvedObject.name", objNameIndexer).
			WithIndex(&corev1.Event{}, "reason", reasonIndexer).
			Build(),
	}

	tests := []struct {
		testName    string
		matchLen    int
		matchResult string

		kind      string
		name      string
		namespace string
	}{
		{
			testName:    "match single event",
			matchResult: "Reconciliation succeeded",
			matchLen:    1,

			kind:      "ResourceSet",
			name:      "infra",
			namespace: "flux-system",
		},
		{
			testName:    "match multiple events",
			matchResult: "Reconciliation failed",
			matchLen:    2,

			kind:      "FluxInstance",
			name:      "flux",
			namespace: "flux-system",
		},
		{
			testName: "match no events",
			matchLen: 0,

			kind:      "FluxInstance",
			name:      "flux1",
			namespace: "flux-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			events, err := kubeClient.GetEvents(
				context.Background(),
				tt.kind,
				tt.name,
				tt.namespace,
				"",
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(events).To(HaveLen(tt.matchLen))
			for _, event := range events {
				g.Expect(event.Message).To(ContainSubstring(tt.matchResult))
			}
		})
	}
}

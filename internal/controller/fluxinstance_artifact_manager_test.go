// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestArtifactAnnotationsChangedPredicate_Update(t *testing.T) {
	for _, tt := range []struct {
		name   string
		oldObj client.Object
		newObj client.Object
		result bool
	}{
		{
			name:   "false if old object is nil",
			oldObj: nil,
			newObj: &fluxcdv1.FluxInstance{},
			result: false,
		},
		{
			name:   "false if new object is nil",
			oldObj: &fluxcdv1.FluxInstance{},
			newObj: nil,
			result: false,
		},
		{
			name: "true if object was disabled and is now enabled",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcile": "disabled",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcile": "enabled",
					},
				},
			},
			result: true,
		},
		{
			name: "true if object was disabled and no longer has the reconcile annotation",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcile": "disabled",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{},
			result: true,
		},
		{
			name: "true if artifact interval changed",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcileArtifactEvery": "20m",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcileArtifactEvery": "2m",
					},
				},
			},
			result: true,
		},
		{
			name:   "true if artifact interval changed adding annotation",
			oldObj: &fluxcdv1.FluxInstance{},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcileArtifactEvery": "2m",
					},
				},
			},
			result: true,
		},
		{
			name: "true if artifact interval changed removing annotation",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcileArtifactEvery": "2m",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{},
			result: true,
		},
		{
			name: "false if unrelated annotation was removed",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/unrelated": "SomeValue",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{},
			result: false,
		},
		{
			name:   "false if unrelated annotation was added",
			oldObj: &fluxcdv1.FluxInstance{},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/unrelated": "SomeValue",
					},
				},
			},
			result: false,
		},
		{
			name: "false if unrelated annotation changed",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/unrelated": "SomeValue",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/unrelated": "SomeOtherValue",
					},
				},
			},
			result: false,
		},
		{
			name:   "false if the artifact interval annotation with the default value was added",
			oldObj: &fluxcdv1.FluxInstance{},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcileArtifactEvery": "10m",
					},
				},
			},
			result: false,
		},
		{
			name: "false if the artifact interval annotation with the default value was removed",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcileArtifactEvery": "10m",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{},
			result: false,
		},
		{
			name:   "false if the reconcile annotation with the default value was added",
			oldObj: &fluxcdv1.FluxInstance{},
			newObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcile": "enabled",
					},
				},
			},
			result: false,
		},
		{
			name: "false if the reconcile annotation with the default value was removed",
			oldObj: &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"fluxcd.controlplane.io/reconcile": "enabled",
					},
				},
			},
			newObj: &fluxcdv1.FluxInstance{},
			result: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			predicate := ArtifactReconciliationConfigurationChangedPredicate{}

			result := predicate.Update(event.UpdateEvent{
				ObjectOld: tt.oldObj,
				ObjectNew: tt.newObj,
			})
			g.Expect(result).To(Equal(tt.result))
		})
	}
}

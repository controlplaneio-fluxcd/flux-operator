// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package client

import (
	"context"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestExport(t *testing.T) {
	mockNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
	}

	mockSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: "flux-system",
		},
		Data: map[string][]byte{
			"username": []byte("flux"),
			"password": []byte("password"),
		},
	}

	mockInstance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/name": "flux",
			},
			Generation: 1,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.3.x",
				Registry: "ghcr.io/fluxcd",
			},
			Components: []fluxcdv1.Component{
				"source-controller",
				"kustomize-controller",
				"helm-controller",
			},
			Cluster: &fluxcdv1.Cluster{
				Domain:                      "cluster.local",
				Multitenant:                 true,
				TenantDefaultServiceAccount: "flux",
				NetworkPolicy:               true,
				Type:                        "kubernetes",
			},
		},
	}

	mockInstance.Status = fluxcdv1.FluxInstanceStatus{
		Conditions: []metav1.Condition{
			{
				Type:    meta.ReadyCondition,
				Status:  metav1.ConditionTrue,
				Reason:  "ReconciliationSucceeded",
				Message: "Reconciliation finished in 52s",
				LastTransitionTime: metav1.Time{
					Time: metav1.Now().Add(-52 * time.Second),
				},
				ObservedGeneration: 1,
			},
		},
		LastAppliedRevision:   "v2.3.0@sha256:1057d9a5afdbed028350a4a4921b6f9a81e567a85a5e2b133244511be578fc75",
		LastAttemptedRevision: "v2.3.0@sha256:1057d9a5afdbed028350a4a4921b6f9a81e567a85a5e2b133244511be578fc75",
		Components: []fluxcdv1.ComponentImage{
			{
				Name:       "source-controller",
				Repository: "ghcr.io/fluxcd/source-controller",
				Tag:        "v1.3.0",
				Digest:     "sha256:161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc",
			},
			{
				Name:       "kustomize-controller",
				Repository: "ghcr.io/fluxcd/kustomize-controller",
				Tag:        "v1.3.0",
				Digest:     "sha256:48a032574dd45c39750ba0f1488e6f1ae36756a38f40976a6b7a588d83acefc1",
			},
			{
				Name:       "helm-controller",
				Repository: "ghcr.io/fluxcd/helm-controller",
				Tag:        "v1.0.1",
				Digest:     "sha256:a67a037faa850220ff94d8090253732079589ad9ff10b6ddf294f3b7cd0f3424",
			},
		},
	}

	kubeClient := KubeClient{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithObjects(mockNamespace, mockInstance, mockSecret).
			WithStatusSubresource(&fluxcdv1.FluxInstance{}).
			Build(),
	}

	tests := []struct {
		testName    string
		matchResult string
		matchErr    string
		emptyResult bool

		apiVersion  string
		kind        string
		name        string
		namespace   string
		selector    string
		maskSecrets bool
		limit       int
	}{
		{
			testName:    "match kind",
			matchResult: "1057d9a5afdbed028350a4a4921b6f9a81e567a85a5e2b133244511be578fc75",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
		},
		{
			testName:    "match selector",
			matchResult: "161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			selector:   "app.kubernetes.io/name=flux",
		},
		{
			testName:    "mask secret",
			matchResult: "password: '****'",

			apiVersion:  "v1",
			kind:        "Secret",
			maskSecrets: true,
		},
		{
			testName:    "unmask secret",
			matchResult: "password: cGFzc3dvcmQ=",

			apiVersion: "v1",
			kind:       "Secret",
		},
		{
			testName:    "no match for kind",
			emptyResult: true,

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstanceNotFound",
		},
		{
			testName:    "no match for selector",
			emptyResult: true,

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			selector:   "app.kubernetes.io/name=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			gvk, err := kubeClient.ParseGroupVersionKind(tt.apiVersion, tt.kind)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := kubeClient.Export(
				context.Background(),
				[]schema.GroupVersionKind{gvk},
				tt.name,
				tt.namespace,
				tt.selector,
				tt.limit,
				tt.maskSecrets,
			)
			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(ContainSubstring(tt.matchResult))
			}

			if tt.emptyResult {
				g.Expect(result).To(BeEmpty())
			}
		})
	}
}

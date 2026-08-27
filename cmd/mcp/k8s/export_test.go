// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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
			Annotations: map[string]string{
				corev1.LastAppliedConfigAnnotation: `{"apiVersion":"v1","kind":"Secret","data":{"password":"cGFzc3dvcmQ="}}`,
				"example.com/owner":                "flux",
			},
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

	mockEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux.1",
			Namespace: "flux-system",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      fluxcdv1.FluxInstanceKind,
			Name:      "flux",
			Namespace: "flux-system",
		},
		Type:    corev1.EventTypeWarning,
		Reason:  "ReconciliationFailed",
		Message: "Reconciliation failed with timeout",
	}

	mockHelmRelease := makeHelmRelease("flux-system", "helm.toolkit.fluxcd.io/v2", "flux-system", []any{
		map[string]any{
			"name":      "my-release",
			"chartName": "podinfo",
			"version":   int64(1),
			"namespace": "flux-system",
		},
	}, false)

	mockReport := &fluxcdv1.FluxReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxReportSpec{
			Distribution: fluxcdv1.FluxDistributionStatus{
				Entitlement: "Unknown",
				Status:      "Installed",
				Version:     "v2.3.0",
			},
		},
	}

	// The fake client does not support the field selectors used to list events,
	// so the events are served by an interceptor that also counts the lookups.
	eventLookups := 0
	listEvents := func(ctx context.Context, c ctrlclient.WithWatch, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
		if el, ok := list.(*corev1.EventList); ok {
			eventLookups++
			el.Items = []corev1.Event{*mockEvent}
			return nil
		}
		return c.List(ctx, list, opts...)
	}

	// The Helm storage lookups are counted by intercepting the Secret reads.
	helmLookups := 0
	getSecret := func(ctx context.Context, c ctrlclient.WithWatch, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
		if _, ok := obj.(*corev1.Secret); ok {
			helmLookups++
		}
		return c.Get(ctx, key, obj, opts...)
	}

	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithObjects(mockNamespace, mockInstance, mockSecret, mockHelmRelease, mockReport).
			WithStatusSubresource(&fluxcdv1.FluxInstance{}).
			WithInterceptorFuncs(interceptor.Funcs{List: listEvents, Get: getSecret}).
			Build(),
	}

	tests := []struct {
		testName     string
		matchResult  string
		matchResults []string
		missResults  []string
		matchErr     string
		emptyResult  bool
		eventLookups int
		helmLookups  int
		docCount     int

		apiVersion  string
		kind        string
		name        string
		namespace   string
		selector    string
		maskSecrets bool
		limit       int
		fields      []string
	}{
		{
			testName:     "match kind",
			eventLookups: 1,
			matchResult:  "1057d9a5afdbed028350a4a4921b6f9a81e567a85a5e2b133244511be578fc75",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
		},
		{
			testName:     "match selector",
			eventLookups: 1,
			matchResult:  "161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			selector:   "app.kubernetes.io/name=flux",
		},
		{
			testName:    "mask secret",
			matchResult: "password: '****'",
			missResults: []string{"cGFzc3dvcmQ=", "last-applied-configuration"},

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
			testName:    "remove kubectl annotation",
			matchResult: "example.com/owner: flux",
			missResults: []string{"last-applied-configuration"},

			apiVersion: "v1",
			kind:       "Secret",
		},
		{
			testName:     "include events by default",
			eventLookups: 1,
			matchResults: []string{"events:", "Reconciliation failed with timeout"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
		},
		{
			testName: "project fields",
			matchResults: []string{
				"apiVersion: fluxcd.controlplane.io/v1",
				"kind: FluxInstance",
				"name: flux",
				"namespace: flux-system",
				"version: 2.3.x",
				"lastAppliedRevision:",
			},
			missResults: []string{
				"components:",
				"registry:",
				"conditions:",
				"lastAttemptedRevision:",
				"events:",
				"labels:",
			},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"spec.distribution.version", "status.lastAppliedRevision"},
		},
		{
			testName:     "project fields with prefix covering events",
			eventLookups: 1,
			matchResults: []string{"conditions:", "lastAttemptedRevision:", "events:", "Reconciliation failed with timeout"},
			missResults:  []string{"spec:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"status"},
		},
		{
			testName:     "project fields ignores unknown paths",
			matchResults: []string{"kind: FluxInstance", "name: flux"},
			missResults:  []string{"spec:", "status:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"spec.unknown", ".status.conditions.type"},
		},
		{
			testName: "project fields with jsonpath filter and wildcard",
			matchResults: []string{
				"status.conditions[?(@.type==\"Ready\")].message: Reconciliation finished in 52s",
				"status.components[*].name:",
				"- source-controller",
				"- kustomize-controller",
				"- helm-controller",
			},
			missResults: []string{"events:", "repository:", "lastAppliedRevision:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"{.status.conditions[?(@.type==\"Ready\")].message}", ".status.components[*].name"},
		},
		{
			testName:     "project fields with jsonpath index",
			matchResults: []string{"status.components[0].repository: ghcr.io/fluxcd/source-controller"},
			missResults:  []string{"kustomize-controller", "events:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"status.components[0].repository"},
		},
		{
			testName:     "project fields with jsonpath on events",
			eventLookups: 1,
			matchResults: []string{"status.events[*].message: Reconciliation failed with timeout"},
			missResults:  []string{"conditions:", "lastTimestamp:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"status.events[*].message"},
		},
		{
			testName:     "project fields with recursive descent covers events",
			eventLookups: 1,
			matchResults: []string{"..message:", "- Reconciliation finished in 52s", "- Reconciliation failed with timeout"},
			missResults:  []string{"events:", "conditions:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			fields:     []string{"..message"},
		},
		{
			testName:     "project fields keeps secret masked",
			matchResults: []string{"password: '****'", "username: '****'"},
			missResults:  []string{"annotations:", "cGFzc3dvcmQ="},

			apiVersion:  "v1",
			kind:        "Secret",
			maskSecrets: true,
			fields:      []string{"data"},
		},
		{
			testName:     "include helm inventory by default",
			eventLookups: 1,
			helmLookups:  1,
			matchResults: []string{"kind: HelmRelease", "chartName: podinfo", "inventory: []"},

			apiVersion: "helm.toolkit.fluxcd.io/v2",
			kind:       "HelmRelease",
		},
		{
			testName:     "project fields skips helm inventory",
			matchResults: []string{"kind: HelmRelease", "chartName: podinfo"},
			missResults:  []string{"inventory:", "events:", "storageNamespace:"},

			apiVersion: "helm.toolkit.fluxcd.io/v2",
			kind:       "HelmRelease",
			fields:     []string{"status.history"},
		},
		{
			testName:     "project fields includes helm inventory when covered",
			helmLookups:  1,
			matchResults: []string{"kind: HelmRelease", "inventory: []"},
			missResults:  []string{"history:", "events:"},

			apiVersion: "helm.toolkit.fluxcd.io/v2",
			kind:       "HelmRelease",
			fields:     []string{"status.inventory"},
		},
		{
			testName:     "project fields skips report metrics",
			docCount:     1,
			matchResults: []string{"kind: FluxReport", "version: v2.3.0"},
			missResults:  []string{"entitlement:", "events:"},

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxReport",
			fields:     []string{"spec.distribution.version"},
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
			eventLookups = 0
			helmLookups = 0

			gvk, err := kubeClient.ParseGroupVersionKind(tt.apiVersion, tt.kind)
			g.Expect(err).NotTo(HaveOccurred())

			fieldPaths, err := ParseFieldPaths(tt.fields)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := kubeClient.Export(
				context.Background(),
				[]schema.GroupVersionKind{gvk},
				tt.name,
				tt.namespace,
				tt.selector,
				tt.limit,
				tt.maskSecrets,
				fieldPaths,
			)
			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(ContainSubstring(tt.matchResult))
			}

			for _, matchResult := range tt.matchResults {
				g.Expect(result).To(ContainSubstring(matchResult))
			}

			g.Expect(eventLookups).To(Equal(tt.eventLookups))
			g.Expect(helmLookups).To(Equal(tt.helmLookups))

			if tt.docCount > 0 {
				g.Expect(strings.Count(result, "---\n")).To(Equal(tt.docCount))
			}

			for _, missResult := range tt.missResults {
				g.Expect(result).NotTo(ContainSubstring(missResult))
			}

			if tt.emptyResult {
				g.Expect(result).To(BeEmpty())
			}
		})
	}
}

func TestExportReturnsListError(t *testing.T) {
	g := NewWithT(t)
	forbiddenErr := apierrors.NewForbidden(
		schema.GroupResource{Resource: "secrets"},
		"",
		errors.New("list denied"),
	)
	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(context.Context, ctrlclient.WithWatch, ctrlclient.ObjectList, ...ctrlclient.ListOption) error {
					return forbiddenErr
				},
			}).
			Build(),
	}

	result, err := kubeClient.Export(
		context.Background(),
		[]schema.GroupVersionKind{{Version: "v1", Kind: "Secret"}},
		"",
		"",
		"",
		0,
		true,
		nil,
	)

	g.Expect(result).To(BeEmpty())
	g.Expect(err).To(MatchError(ContainSubstring("failed to list /v1, Kind=Secret")))
	g.Expect(apierrors.IsForbidden(err)).To(BeTrue())
}

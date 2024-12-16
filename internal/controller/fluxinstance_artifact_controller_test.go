// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

func TestFluxInstanceArtifactReconciler(t *testing.T) {
	const cpLatestManifestsURL = "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest"

	g := NewWithT(t)

	latestArtifactRevision, err := builder.HeadArtifact(context.Background(), cpLatestManifestsURL)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(latestArtifactRevision).To(HavePrefix("sha256:"))
	g.Expect(strings.TrimPrefix(latestArtifactRevision, "sha256:")).To(HaveLen(64))

	for _, tt := range []struct {
		name                        string
		delete                      bool
		annotations                 map[string]string
		manifestsURL                string
		lastArtifactRevision        string
		result                      ctrl.Result
		err                         error
		shouldRequestReconciliation bool
	}{
		{
			name:                        "requests reconciliation when digest is different",
			manifestsURL:                cpLatestManifestsURL,
			lastArtifactRevision:        "",
			result:                      ctrl.Result{RequeueAfter: 10 * time.Minute},
			shouldRequestReconciliation: true,
		},
		{
			name:                        "does not request reconciliation when up-to-date",
			manifestsURL:                cpLatestManifestsURL,
			lastArtifactRevision:        latestArtifactRevision,
			result:                      ctrl.Result{RequeueAfter: 10 * time.Minute},
			shouldRequestReconciliation: false,
		},
		{
			name:                        "uses interval from annotation",
			annotations:                 map[string]string{"fluxcd.controlplane.io/reconcileArtifactEvery": "2m"},
			manifestsURL:                cpLatestManifestsURL,
			lastArtifactRevision:        latestArtifactRevision,
			result:                      ctrl.Result{RequeueAfter: 2 * time.Minute},
			shouldRequestReconciliation: false,
		},
		{
			name:                        "does not request reconciliation when on deletion",
			delete:                      true,
			manifestsURL:                cpLatestManifestsURL,
			lastArtifactRevision:        "",
			result:                      ctrl.Result{},
			shouldRequestReconciliation: false,
		},
		{
			name:                        "does not request reconciliation when disabled",
			annotations:                 map[string]string{"fluxcd.controlplane.io/reconcile": "disabled"},
			manifestsURL:                cpLatestManifestsURL,
			lastArtifactRevision:        "",
			result:                      ctrl.Result{},
			shouldRequestReconciliation: false,
		},
		{
			name:                        "does not request reconciliation when artifact is not specified",
			manifestsURL:                "",
			result:                      ctrl.Result{},
			shouldRequestReconciliation: false,
		},
		{
			name:                        "does not request reconciliation on artifact error",
			manifestsURL:                "oci://not.found/artifact",
			lastArtifactRevision:        "",
			result:                      ctrl.Result{},
			err:                         errors.New("no such host"),
			shouldRequestReconciliation: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := getFluxInstanceArtifactReconciler()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			ns, err := testEnv.CreateNamespace(ctx, "test")
			g.Expect(err).ToNot(HaveOccurred())

			obj := &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:        ns.Name,
					Namespace:   ns.Name,
					Annotations: tt.annotations,
				},
				Spec: getDefaultFluxSpec(),
			}
			obj.Spec.Distribution.Artifact = tt.manifestsURL

			err = testEnv.Create(ctx, obj)
			g.Expect(err).ToNot(HaveOccurred())

			if tt.lastArtifactRevision != "" {
				obj.Status.LastArtifactRevision = tt.lastArtifactRevision
				err := testEnv.Status().Update(ctx, obj)
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.delete {
				obj.Finalizers = append(obj.Finalizers, "test")
				err := testEnv.Update(ctx, obj)
				g.Expect(err).ToNot(HaveOccurred())
				err = testEnv.Delete(ctx, obj)
				g.Expect(err).ToNot(HaveOccurred())
			}

			r, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(obj),
			})
			if tt.err != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err.Error()))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			g.Expect(r).To(Equal(tt.result))

			err = testEnv.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			g.Expect(err).ToNot(HaveOccurred())

			annotations := obj.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			reconcileRequestAnnotation := annotations[meta.ReconcileRequestAnnotation]

			if tt.shouldRequestReconciliation {
				g.Expect(reconcileRequestAnnotation).ToNot(BeEmpty())
				requestedAt, err := time.Parse(time.RFC3339Nano, reconcileRequestAnnotation)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(requestedAt).To(BeTemporally("~", time.Now(), time.Second))
			} else {
				g.Expect(reconcileRequestAnnotation).To(BeEmpty())
			}
		})
	}
}

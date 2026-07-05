// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	cryptotls "crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

// FluxInstanceArtifactReconciler reconciles the distribution artifact of a FluxInstance object
type FluxInstanceArtifactReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	StatusManager string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FluxInstanceArtifactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	obj := &fluxcdv1.FluxInstance{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip reconciliation if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Skip reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		return ctrl.Result{}, nil
	}

	// Skip reconciliation if the object does not have a last artifact revision to avoid race condition.
	if obj.Status.LastArtifactRevision == "" {
		return requeueArtifactAfter(obj), nil
	}

	// Skip reconciliation if the object is not ready.
	if !conditions.IsReady(obj) {
		return requeueArtifactAfter(obj), nil
	}

	// Reconcile the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)
	return r.reconcile(ctx, obj, patcher)
}

func (r *FluxInstanceArtifactReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {

	log := ctrl.LoggerFrom(ctx)

	// Fetch the latest digest of the distribution manifests.
	artifactURL := obj.Spec.Distribution.Artifact
	keyChain, err := GetDistributionKeychain(ctx, r.Client, obj)
	if err != nil {
		msg := fmt.Sprintf("failed to get distribution key chain: %s", err.Error())
		r.Event(obj, corev1.EventTypeWarning, meta.ArtifactFailedReason, msg)
		return ctrl.Result{}, err
	}

	artifactDigest, err := builder.GetArtifactDigest(ctx, artifactURL, keyChain)
	if err != nil {
		msg := fmt.Sprintf("fetch failed: %s", err.Error())
		r.Event(obj, corev1.EventTypeWarning, meta.ArtifactFailedReason, msg)
		return ctrl.Result{}, err
	}
	log.V(1).Info("fetched latest manifests digest", "url", artifactURL, "digest", artifactDigest)

	// Skip reconciliation if the artifact has not changed.
	if artifactDigest == obj.Status.LastArtifactRevision {
		return requeueArtifactAfter(obj), nil
	}

	// The digest has changed, request a reconciliation.
	log.Info("artifact revision changed, requesting a reconciliation",
		"old", obj.Status.LastArtifactRevision, "new", artifactDigest)
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string, 1)
	}
	obj.Annotations[meta.ReconcileRequestAnnotation] = time.Now().Format(time.RFC3339Nano)
	if err := patcher.Patch(ctx, obj, patch.WithFieldOwner(r.StatusManager)); err != nil {
		return ctrl.Result{}, err
	}

	return requeueArtifactAfter(obj), nil
}

// GetDistributionKeychain creates a keychain from the artifactPullSecret secret if provided.
func GetDistributionKeychain(ctx context.Context, kubeClient client.Client, obj *fluxcdv1.FluxInstance) (authn.Keychain, error) {
	artifactPullSecret := obj.Spec.Distribution.ArtifactPullSecret
	if artifactPullSecret == "" {
		return nil, nil
	}

	key := types.NamespacedName{
		Name:      artifactPullSecret,
		Namespace: obj.GetNamespace(),
	}
	var secret corev1.Secret
	if err := kubeClient.Get(ctx, key, &secret); err != nil {
		return nil, err
	}
	return kauth.NewFromPullSecrets(ctx, []corev1.Secret{secret})
}

// requeueArtifactAfter returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotation for artifact reconciliation.
func requeueArtifactAfter(obj *fluxcdv1.FluxInstance) ctrl.Result {
	result := ctrl.Result{}
	if d := obj.GetArtifactInterval(); d > 0 {
		result.RequeueAfter = d
	}
	return result
}

// GetDistributionTransport clones the default transport from remote and when a certSecretRef is specified,
// the returned transport will include the TLS client and/or CA certificates.
// If the insecure flag is set, the transport will skip the verification of the server's certificate.
// based off: https://github.com/fluxcd/source-controller/blob/39b711b111fa906c3db0a424e252a5d12e48646d/internal/controller/ocirepository_controller.go#L1010-L1014
func GetDistributionTransport(ctx context.Context, kubeClient client.Client, obj *fluxcdv1.FluxInstance) (http.RoundTripper, error) {
	transport := remote.DefaultTransport.(*http.Transport).Clone()

	tlsConfig, err := getTLSConfig(ctx, obj, kubeClient)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}

	return transport, nil
}

// getTLSConfig gets the TLS configuration for the transport based on the
// specified secret reference in the FluxInstance object, or the insecure flag.
// based off: https://github.com/fluxcd/source-controller/blob/39b711b111fa906c3db0a424e252a5d12e48646d/internal/controller/ocirepository_controller.go#L1032-L1034
func getTLSConfig(ctx context.Context, obj *fluxcdv1.FluxInstance, kubeClient client.Client) (*cryptotls.Config, error) {
	if obj.Spec.Distribution.CertSecretRef == nil || obj.Spec.Distribution.CertSecretRef.Name == "" {
		if obj.Spec.Distribution.Insecure {
			// NOTE: This is the only place in Flux where InsecureSkipVerify is allowed.
			// This exception is made for OCIRepository to maintain backward compatibility
			// with tools like crane that require insecure connections without certificates.
			// This only applies when no CertSecretRef is provided AND insecure is explicitly set.
			// All other controllers must NOT allow InsecureSkipVerify per our security policy.
			return &cryptotls.Config{
				InsecureSkipVerify: true,
			}, nil
		}
		return nil, nil
	}

	secretName := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Spec.Distribution.CertSecretRef.Name,
	}
	// NOTE: Use WithSystemCertPool to maintain backward compatibility with the existing
	// extend approach (system CAs + user CA) rather than the default replace approach (user CA only).
	// This ensures source-controller continues to work with both system and user-provided CA certificates.
	var tlsOpts = []secrets.TLSConfigOption{secrets.WithSystemCertPool()}
	return secrets.TLSConfigFromSecretRef(ctx, kubeClient, secretName, tlsOpts...)
}

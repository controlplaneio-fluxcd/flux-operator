// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
)

// EntitlementReconciler reconciles entitlements.
type EntitlementReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	EntitlementClient entitlement.Client
	Scheme            *runtime.Scheme
	StatusPoller      *polling.StatusPoller
	StatusManager     string
	WatchNamespace    string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *EntitlementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, namespace); err != nil {
		return ctrl.Result{}, err
	}

	secret, err := r.GetEntitlementSecret(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Reconciling entitlement %s/%s", namespace.Name, secret.Name),
		entitlement.VendorKey, string(secret.Data[entitlement.VendorKey]))

	var token string
	id := string(namespace.UID)

	// Get the token from the secret if it exists.
	if t, found := secret.Data[entitlement.TokenKey]; found {
		token = string(t)
	}

	// Register the usage if the token is missing and update the secret.
	if token == "" {
		token, err = r.EntitlementClient.RegisterUsage(ctx, id)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to register usage for vendor %s: %w",
				r.EntitlementClient.GetVendor(), err)
		}

		if err := r.UpdateEntitlementSecret(ctx, token); err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Entitlement registered", "vendor", r.EntitlementClient.GetVendor())

		// Requeue to verify the token.
		return ctrl.Result{Requeue: true}, nil
	}

	// Verify the token and delete the secret if it is invalid.
	valid, err := r.EntitlementClient.Verify(token, id)
	if !valid {
		if err := r.DeleteEntitlementSecret(ctx, secret); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, fmt.Errorf("failed to verify entitlement: %w", err)
	}

	log.Info("Entitlement verified", "vendor", r.EntitlementClient.GetVendor())
	return ctrl.Result{RequeueAfter: 30 * time.Minute}, nil
}

// EntitlementReconcilerOptions contains options for the reconciler.
type EntitlementReconcilerOptions struct {
	RateLimiter ratelimiter.RateLimiter
}

// SetupWithManager sets up the controller with the Manager and initializes the
// entitlement secret in the watch namespace.
func (r *EntitlementReconciler) SetupWithManager(mgr ctrl.Manager, opts EntitlementReconcilerOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := r.InitEntitlementSecret(ctx); err != nil {
		return err
	}

	ps, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			"kubernetes.io/metadata.name": r.WatchNamespace,
		},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).For(
		&corev1.Namespace{},
		builder.WithPredicates(ps)).
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
		WithOptions(controller.Options{RateLimiter: opts.RateLimiter}).
		Complete(r)
}

// InitEntitlementSecret creates the entitlement secret if it doesn't exist
// and sets the entitlement vendor if it's missing or different.
func (r *EntitlementReconciler) InitEntitlementSecret(ctx context.Context) (*corev1.Secret, error) {
	secretName := fmt.Sprintf("%s-entitlement", r.StatusManager)
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: r.WatchNamespace,
		Name:      secretName,
	}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: r.WatchNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/name":       r.StatusManager,
						"app.kubernetes.io/component":  "entitlement",
						"app.kubernetes.io/managed-by": r.StatusManager,
					},
				},
				Data: map[string][]byte{
					entitlement.VendorKey: []byte(r.EntitlementClient.GetVendor()),
				},
			}
			errNew := r.Client.Create(ctx, newSecret)
			if errNew != nil {
				return nil, fmt.Errorf("failed to create %s: %w", secretName, errNew)
			}
			return newSecret, nil
		} else {
			return nil, fmt.Errorf("failed to init %s: %w", secretName, err)
		}
	}

	exitingVendor, found := secret.Data[entitlement.VendorKey]
	if !found || string(exitingVendor) != r.EntitlementClient.GetVendor() {
		secret.Data = make(map[string][]byte)
		secret.Data[entitlement.VendorKey] = []byte(r.EntitlementClient.GetVendor())
		if err := r.Client.Update(ctx, secret); err != nil {
			return nil, fmt.Errorf("failed to set vendor in %s: %w", secretName, err)
		}
	}

	return secret, nil
}

// GetEntitlementSecret returns the entitlement secret.
// if the secret doesn't exist, it gets initialized.
func (r *EntitlementReconciler) GetEntitlementSecret(ctx context.Context) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)
	secretName := fmt.Sprintf("%s-entitlement", r.StatusManager)
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: r.WatchNamespace,
		Name:      secretName,
	}, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("Entitlement not found, initializing %s/%s", r.WatchNamespace, secretName))
			return r.InitEntitlementSecret(ctx)
		}
		return nil, fmt.Errorf("failed to get %s: %w", secretName, err)
	}

	return secret, nil
}

// UpdateEntitlementSecret updates the token in the entitlement secret.
func (r *EntitlementReconciler) UpdateEntitlementSecret(ctx context.Context, token string) error {
	secret, err := r.GetEntitlementSecret(ctx)
	if err != nil {
		return err
	}

	secret.Data[entitlement.TokenKey] = []byte(token)
	if err := r.Client.Update(ctx, secret); err != nil {
		return fmt.Errorf("failed to update %s: %w", secret.Name, err)
	}

	return nil
}

// DeleteEntitlementSecret deletes the entitlement secret.
func (r *EntitlementReconciler) DeleteEntitlementSecret(ctx context.Context, secret *corev1.Secret) error {
	if err := r.Client.Delete(ctx, secret); err != nil {
		return fmt.Errorf("failed to delete %s: %w", secret.Name, err)
	}

	return nil
}

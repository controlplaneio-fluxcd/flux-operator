// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth/github"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/gitprovider"
)

// ResourceSetInputProviderReconciler reconciles a ResourceSetInputProvider object
type ResourceSetInputProviderReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	StatusManager string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesetinputproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesetinputproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesetinputproviders/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceSetInputProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	// Finalise the reconciliation and report the results.
	defer func() {
		if err := r.finalizeStatus(ctx, obj, patcher); err != nil {
			log.Error(err, "failed to update status")
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}
	}()

	// Uninstall if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		// Release the object to be garbage collected.
		controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)

		// Stop reconciliation as the object is being deleted.
		return ctrl.Result{}, nil
	}

	// Add the finalizer if it does not exist.
	if !controllerutil.ContainsFinalizer(obj, fluxcdv1.Finalizer) {
		log.Info("Adding finalizer", "finalizer", fluxcdv1.Finalizer)
		controllerutil.AddFinalizer(obj, fluxcdv1.Finalizer)
		conditions.MarkUnknown(obj,
			meta.ReadyCondition,
			meta.ProgressingReason,
			"%s", msgInProgress)
		conditions.MarkReconciling(obj,
			meta.ProgressingReason,
			"%s", msgInProgress)
		return ctrl.Result{Requeue: true}, nil
	}

	// Pause reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		log.Error(errors.New("can't reconcile instance"), fluxcdv1.ReconciliationDisabledMessage)
		r.Event(obj, corev1.EventTypeWarning, fluxcdv1.ReconciliationDisabledReason, fluxcdv1.ReconciliationDisabledMessage)
		return ctrl.Result{}, nil
	}

	// Reconcile the object.
	return r.reconcile(ctx, obj, patcher)
}

func (r *ResourceSetInputProviderReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	// Mark the object as reconciling.
	msg := "Reconciliation in progress"
	conditions.MarkUnknown(obj,
		meta.ReadyCondition,
		meta.ProgressingReason,
		"%s", msg)
	conditions.MarkReconciling(obj,
		meta.ProgressingReason,
		"%s", msg)
	if err := r.patch(ctx, obj, patcher); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	// Get the auth data.
	var authData map[string][]byte
	if obj.Spec.SecretRef != nil {
		var err error
		authData, err = r.getSecretData(ctx, obj.Spec.SecretRef.Name, obj.GetNamespace())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get credentials: %w", err)
		}
	}

	// Get the CA certificate.
	certPool, err := r.getCertPool(ctx, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get certificates: %w", err)
	}

	// Create the provider based on the object type.
	provider, err := r.newGitProvider(ctx, obj, certPool, authData)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create provider: %w", err)
	}

	// Get the provider options.
	exportedInputs, err := r.callProvider(ctx, obj, provider)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to call provider: %w", err)
	}

	// Update the object status with the exported inputs.
	data, err := yaml.Marshal(exportedInputs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal exported inputs: %w", err)
	}
	obj.Status.ExportedInputs = exportedInputs
	obj.Status.LastExportedRevision = digest.FromBytes(data).String()

	// Mark the object as ready and set the last applied revision.
	msg = fmt.Sprintf("Reconciliation finished in %s", fmtDuration(reconcileStart))
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		"%s", msg)
	log.Info(msg)
	r.EventRecorder.Event(obj,
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	return requeueAfterResourceSetInputProvider(obj), nil
}

// newGitProvider returns a new Git provider based on the type specified in the ResourceSetInputProvider object.
func (r *ResourceSetInputProviderReconciler) newGitProvider(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	certPool *x509.CertPool,
	authData map[string][]byte) (gitprovider.Interface, error) {
	switch {
	case strings.HasPrefix(obj.Spec.Type, "GitHub"):
		token, err := r.getGitHubToken(ctx, obj, authData)
		if err != nil {
			return nil, err
		}
		return gitprovider.NewGitHubProvider(ctx, gitprovider.Options{
			URL:      obj.Spec.URL,
			CertPool: certPool,
			Token:    token,
		})
	case strings.HasPrefix(obj.Spec.Type, "GitLab"):
		token, err := r.getGitLabToken(obj, authData)
		if err != nil {
			return nil, err
		}
		return gitprovider.NewGitLabProvider(ctx, gitprovider.Options{
			URL:      obj.Spec.URL,
			CertPool: certPool,
			Token:    token,
		})
	default:
		return nil, fmt.Errorf("unsupported type: %s", obj.Spec.Type)
	}
}

// makeGitOptions returns the gitprovider.Options based on the object spec
// filters and URL, using a default limit of 100.
func (r *ResourceSetInputProviderReconciler) makeGitOptions(obj *fluxcdv1.ResourceSetInputProvider) (gitprovider.Options, error) {
	opts := gitprovider.Options{
		URL: obj.Spec.URL,
		Filters: gitprovider.Filters{
			Limit: 100,
		},
	}

	if obj.Spec.Filter != nil {
		if obj.Spec.Filter.Limit > 0 {
			opts.Filters.Limit = obj.Spec.Filter.Limit
		}
		if len(obj.Spec.Filter.Labels) > 0 {
			opts.Filters.Labels = obj.Spec.Filter.Labels
		}
		if obj.Spec.Filter.IncludeBranch != "" {
			inRx, err := regexp.Compile(obj.Spec.Filter.IncludeBranch)
			if err != nil {
				return gitprovider.Options{}, fmt.Errorf("invalid includeBranch regex: %w", err)
			}
			opts.Filters.IncludeBranchRe = inRx
		}
		if obj.Spec.Filter.ExcludeBranch != "" {
			exRx, err := regexp.Compile(obj.Spec.Filter.ExcludeBranch)
			if err != nil {
				return gitprovider.Options{}, fmt.Errorf("invalid excludeBranch regex: %w", err)
			}
			opts.Filters.ExcludeBranchRe = exRx
		}
	}

	return opts, nil
}

func (r *ResourceSetInputProviderReconciler) callProvider(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	provider gitprovider.Interface) ([]fluxcdv1.ResourceSetInput, error) {
	var inputs []fluxcdv1.ResourceSetInput

	opts, err := r.makeGitOptions(obj)
	if err != nil {
		return nil, err
	}

	var results []gitprovider.Result
	switch {
	case strings.HasSuffix(obj.Spec.Type, "Branch"):
		results, err = provider.ListBranches(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list branches: %w", err)
		}
	case strings.HasSuffix(obj.Spec.Type, "Request"):
		results, err = provider.ListRequests(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list requests: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported type: %s", obj.Spec.Type)
	}

	if len(results) > 0 {
		defaults, err := obj.GetDefaultInputs()
		if err != nil {
			return nil, fmt.Errorf("invalid default values: %w", err)
		}

		inputsWithDefaults, err := gitprovider.MakeInputs(results, defaults)
		if err != nil {
			return nil, fmt.Errorf("failed to generate inputs: %w", err)
		}

		for _, input := range inputsWithDefaults {
			inputs = append(inputs, input)
		}
	}

	return inputs, nil
}

// getBasicAuth returns the basic auth credentials by reading the username
// and password from authData.
//
//nolint:unparam
func (r *ResourceSetInputProviderReconciler) getBasicAuth(
	obj *fluxcdv1.ResourceSetInputProvider,
	authData map[string][]byte) (string, string, error) {

	usernameData, ok := authData["username"]
	if !ok {
		return "", "", fmt.Errorf("invalid secret '%s/%s': key 'username' is missing", obj.GetNamespace(), obj.Spec.SecretRef.Name)
	}

	passwordData, ok := authData["password"]
	if !ok {
		return "", "", fmt.Errorf("invalid secret '%s/%s': key 'password' is missing", obj.GetNamespace(), obj.Spec.SecretRef.Name)
	}

	return strings.TrimSpace(string(usernameData)), strings.TrimSpace(string(passwordData)), nil
}

// getGitHubToken returns the appropriate GitHub token by reading the secrets in authData.
func (r *ResourceSetInputProviderReconciler) getGitHubToken(
	ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	authData map[string][]byte) (string, error) {

	if authData == nil {
		return "", nil
	}

	if _, ok := authData[github.AppIDKey]; !ok {
		_, password, err := r.getBasicAuth(obj, authData)
		return password, err
	}

	ghc, err := github.New(github.WithAppData(authData))
	if err != nil {
		return "", err
	}

	tok, err := ghc.GetToken(ctx)
	if err != nil {
		return "", err
	}

	return tok.Token, nil
}

// getGitLabToken returns the appropriate GitLab token by reading the secrets in authData.
func (r *ResourceSetInputProviderReconciler) getGitLabToken(
	obj *fluxcdv1.ResourceSetInputProvider,
	authData map[string][]byte) (string, error) {

	if authData == nil {
		return "", nil
	}

	_, password, err := r.getBasicAuth(obj, authData)
	return password, err
}

// getCertPool returns the x509.CertPool by reading the CA certificate from
// spec.CertSecretRef.
func (r *ResourceSetInputProviderReconciler) getCertPool(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider) (*x509.CertPool, error) {
	if obj.Spec.CertSecretRef == nil {
		return nil, nil
	}

	certData, err := r.getSecretData(ctx, obj.Spec.CertSecretRef.Name, obj.GetNamespace())
	if err != nil {
		return nil, err
	}

	caData, ok := certData["ca.crt"]
	if !ok {
		return nil, fmt.Errorf("invalid secret '%s/%s': key 'ca.crt' is missing", obj.GetNamespace(), obj.Spec.CertSecretRef.Name)
	}

	certPool := x509.NewCertPool()
	ok = certPool.AppendCertsFromPEM(caData)
	if !ok {
		return nil, fmt.Errorf("invalid secret '%s/%s': 'ca.crt' PEM can't be parsed", obj.GetNamespace(), obj.Spec.CertSecretRef.Name)
	}

	return certPool, nil
}

// getSecretData returns the data of the secret by reading it from the API server.
func (r *ResourceSetInputProviderReconciler) getSecretData(ctx context.Context, name, namespace string) (map[string][]byte, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	var secret corev1.Secret
	if err := r.Client.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("failed to read secret '%s/%s': %w", namespace, name, err)
	}
	return secret.Data, nil
}

// finalizeStatus updates the object status and conditions.
func (r *ResourceSetInputProviderReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	patcher *patch.SerialPatcher) error {
	// Set the value of the reconciliation request in status.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.Status.LastHandledReconcileAt = v
	}

	// Set the Reconciling reason to ProgressingWithRetry if the
	// reconciliation has failed.
	if conditions.IsFalse(obj, meta.ReadyCondition) &&
		conditions.Has(obj, meta.ReconcilingCondition) {
		rc := conditions.Get(obj, meta.ReconcilingCondition)
		rc.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, rc)
	}

	// Remove the Reconciling condition.
	if conditions.IsTrue(obj, meta.ReadyCondition) || conditions.IsTrue(obj, meta.StalledCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
	}

	// Patch finalizers, status and conditions.
	return r.patch(ctx, obj, patcher)
}

// patch updates the object status, conditions and finalizers.
func (r *ResourceSetInputProviderReconciler) patch(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	patcher *patch.SerialPatcher) (retErr error) {
	// Configure the runtime patcher.
	ownedConditions := []string{
		meta.ReadyCondition,
		meta.ReconcilingCondition,
		meta.StalledCondition,
	}
	patchOpts := []patch.Option{
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithForceOverwriteConditions{},
		patch.WithFieldOwner(r.StatusManager),
	}

	// Patch the object status, conditions and finalizers.
	if err := patcher.Patch(ctx, obj, patchOpts...); err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
		}
		retErr = kerrors.NewAggregate([]error{retErr, err})
		if retErr != nil {
			return retErr
		}
	}

	return nil
}

// requeueAfterResourceSetInputProvider returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations.
func requeueAfterResourceSetInputProvider(obj *fluxcdv1.ResourceSetInputProvider) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	}

	return result
}

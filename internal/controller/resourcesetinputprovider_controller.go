// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/git/github"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/gitprovider"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/schedule"
)

// ResourceSetInputProviderReconciler reconciles a ResourceSetInputProvider object
type ResourceSetInputProviderReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	Scheme        *runtime.Scheme
	StatusManager string
	TokenCache    *cache.TokenCache
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

		if err := r.recordMetrics(obj); err != nil {
			log.Error(err, "failed to record metrics")
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
		initializeObjectStatus(obj)
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

	// Handle force reconciliation requests.
	force := meta.ShouldHandleForceRequest(obj)

	// Validate schedule.
	scheduler, err := schedule.NewScheduler(obj.Spec.Schedule, obj.GetTimeout())
	if err != nil {
		errMsg := fmt.Sprintf("%s: %v", msgTerminalError, err)
		conditions.MarkFalse(obj, meta.ReadyCondition, fluxcdv1.ReasonInvalidSchedule, "%s", errMsg)
		conditions.MarkStalled(obj, fluxcdv1.ReasonInvalidSchedule, "%s", errMsg)
		log.Error(err, msgTerminalError)
		r.notify(ctx, obj, corev1.EventTypeWarning, fluxcdv1.ReasonInvalidSchedule, errMsg)
		return ctrl.Result{}, nil
	}

	// Check if the object should be reconciled according to the schedule.
	obj.Status.NextSchedule = nil
	if !scheduler.ShouldReconcile(reconcileStart) && !force {
		obj.Status.NextSchedule = scheduler.Next(reconcileStart)
		next := obj.Status.NextSchedule.When.Time
		msg := fmt.Sprintf("Reconciliation skipped, next scheduled at %s", next.Format(time.RFC3339))

		// If the object is reconciling, mark it as ready and delete the reconciling condition.
		// This occurs only at object creation time when the next schedule is in the future.
		if conditions.IsReconciling(obj) && conditions.IsUnknown(obj, meta.ReadyCondition) {
			conditions.Delete(obj, meta.ReconcilingCondition)
			conditions.MarkTrue(obj, meta.ReadyCondition, fluxcdv1.ReasonSkippedDueToSchedule, "%s", msg)
		}

		log.Info(msg)
		r.notify(ctx, obj, corev1.EventTypeNormal, fluxcdv1.ReasonSkippedDueToSchedule, msg)
		return ctrl.Result{
			RequeueAfter: next.Sub(reconcileStart),
		}, nil
	}

	// Mark stalled if the default values in the object spec are invalid.
	defaults, err := obj.GetDefaultInputs()
	if err != nil {
		errMsg := fmt.Sprintf("%s: %v", msgTerminalError, err)
		conditions.MarkFalse(obj, meta.ReadyCondition, fluxcdv1.ReasonInvalidDefaultValues, "%s", errMsg)
		conditions.MarkStalled(obj, fluxcdv1.ReasonInvalidDefaultValues, "%s", errMsg)
		log.Error(err, msgTerminalError)
		r.notify(ctx, obj, corev1.EventTypeWarning, fluxcdv1.ReasonInvalidDefaultValues, errMsg)
		return ctrl.Result{}, nil
	}

	// Mark the object as reconciling.
	conditions.MarkReconciling(obj, meta.ProgressingReason, "%s", msgInProgress)

	var exportedInputs []fluxcdv1.ResourceSetInput

	switch obj.Spec.Type {
	case fluxcdv1.InputProviderStatic:
		// Handle static input provider.
		defaults["id"] = inputs.Checksum(string(obj.GetUID()))
		exportedInput, err := fluxcdv1.NewResourceSetInput(defaults)
		if err != nil {
			errMsg := fmt.Sprintf("%s: %v", msgTerminalError, err)
			conditions.MarkFalse(obj, meta.ReadyCondition, fluxcdv1.ReasonInvalidExportedInputs, "%s", errMsg)
			conditions.MarkStalled(obj, fluxcdv1.ReasonInvalidExportedInputs, "%s", errMsg)
			log.Error(err, msgTerminalError)
			r.notify(ctx, obj, corev1.EventTypeWarning, fluxcdv1.ReasonInvalidExportedInputs, errMsg)
			return ctrl.Result{}, nil
		}
		exportedInputs = append(exportedInputs, exportedInput)
	default:
		// Mark the object as progressing and update the status.
		conditions.MarkUnknown(obj,
			meta.ReadyCondition,
			meta.ProgressingReason,
			"%s", msgInProgress)
		if err := r.patch(ctx, obj, patcher); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
		}

		// Get the auth data.
		var authData map[string][]byte
		if obj.Spec.SecretRef != nil {
			var err error
			authData, err = r.getSecretData(ctx, obj.Spec.SecretRef.Name, obj.GetNamespace())
			if err != nil {
				msg := fmt.Sprintf("failed to get credentials %s", err.Error())
				conditions.MarkFalse(obj,
					meta.ReadyCondition,
					meta.ReconciliationFailedReason,
					"%s", msg)
				r.notify(ctx, obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)
				return ctrl.Result{}, err
			}
		}

		// Get the CA certificate.
		certPool, err := r.getCertPool(ctx, obj)
		if err != nil {
			msg := fmt.Sprintf("failed to get certificates %s", err.Error())
			conditions.MarkFalse(obj,
				meta.ReadyCondition,
				meta.ReconciliationFailedReason,
				"%s", msg)
			r.notify(ctx, obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)
			return ctrl.Result{}, err
		}

		// Create the provider context with timeout.
		providerCtx, cancel := context.WithTimeout(ctx, obj.GetTimeout())
		defer cancel()

		// Create the provider based on the object type.
		provider, err := r.newGitProvider(providerCtx, obj, certPool, authData)
		if err != nil {
			msg := fmt.Sprintf("failed to create provider %s", err.Error())
			conditions.MarkFalse(obj,
				meta.ReadyCondition,
				meta.ReconciliationFailedReason,
				"%s", msg)
			r.notify(ctx, obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)
			return ctrl.Result{}, err
		}

		// Get the provider options.
		exportedInputs, err = r.callProvider(providerCtx, obj, provider, defaults)
		if err != nil {
			msg := fmt.Sprintf("failed to call provider %s", err.Error())
			conditions.MarkFalse(obj,
				meta.ReadyCondition,
				meta.ReconciliationFailedReason,
				"%s", msg)
			r.notify(ctx, obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)
			return ctrl.Result{}, err
		}
	}

	// Update the object status with the exported inputs.
	data, err := yaml.Marshal(exportedInputs)
	if err != nil {
		msg := fmt.Sprintf("failed to marshal exported inputs %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ReconciliationFailedReason,
			"%s", msg)
		r.notify(ctx, obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)
		return ctrl.Result{}, err
	}
	obj.Status.ExportedInputs = exportedInputs
	obj.Status.LastExportedRevision = digest.FromBytes(data).String()

	// Mark the object as ready and set the last applied revision.
	msg := reconcileMessage(reconcileStart)
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		"%s", msg)
	log.Info(msg)
	r.EventRecorder.Event(obj,
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	reconcileEnd := time.Now()
	return requeueAfterResourceSetInputProvider(obj, scheduler, reconcileEnd), nil
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

// restoreSkippedGitProviderResults skip git provider results when matches skip condition.
func (r *ResourceSetInputProviderReconciler) restoreSkippedGitProviderResults(results []gitprovider.Result, obj *fluxcdv1.ResourceSetInputProvider) ([]gitprovider.Result, error) {
	if obj.Spec.Skip == nil || len(obj.Spec.Skip.Labels) == 0 {
		return results, nil
	}

	exportedInputs, err := obj.GetInputs()
	if err != nil {
		return nil, fmt.Errorf("invalid exportedInputs values: %w", err)
	}

	res := make([]gitprovider.Result, 0, len(results))
	for _, result := range results {
		isSkipped := false
		for _, label := range obj.Spec.Skip.Labels {
			// handle the case when the label is prefixed with ! to skip the result if it does not have the label
			if strings.HasPrefix(label, "!") {
				isSkipped = !slices.Contains(result.Labels, label[1:])
			} else {
				isSkipped = slices.Contains(result.Labels, label)
			}
			if isSkipped {
				break
			}
		}

		if isSkipped {
			var exportedInput map[string]any
			for _, ei := range exportedInputs {
				if ei["id"] == result.ID {
					exportedInput = ei
					break
				}
			}

			// when the result is newly added, we completely skip it
			if exportedInput == nil {
				continue
			}

			err := result.OverrideFromExportedInputs(exportedInput)
			if err != nil {
				return nil, fmt.Errorf("failed to override result from exportedInput: %w", err)
			}
		}

		res = append(res, result)
	}

	return res, nil
}

func (r *ResourceSetInputProviderReconciler) callProvider(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider, provider gitprovider.Interface,
	defaults map[string]any) ([]fluxcdv1.ResourceSetInput, error) {

	var inputSet []fluxcdv1.ResourceSetInput

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
		if results, err = r.restoreSkippedGitProviderResults(results, obj); err != nil {
			return nil, err
		}

		inputsWithDefaults, err := gitprovider.MakeInputs(results, defaults)
		if err != nil {
			return nil, fmt.Errorf("failed to generate inputs: %w", err)
		}

		for _, input := range inputsWithDefaults {
			inputSet = append(inputSet, input)
		}
	}

	return inputSet, nil
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

	opts := []github.OptFunc{github.WithAppData(authData)}

	if r.TokenCache != nil {
		opts = append(opts, github.WithCache(r.TokenCache,
			fluxcdv1.ResourceSetInputProviderKind,
			obj.GetName(),
			obj.GetNamespace(),
			cache.OperationReconcile))
	}

	ghc, err := github.New(opts...)
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
	finalizeObjectStatus(obj)

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

func (r *ResourceSetInputProviderReconciler) recordMetrics(obj *fluxcdv1.ResourceSetInputProvider) error {
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		reporter.DeleteMetricsFor(fluxcdv1.ResourceSetInputProviderKind, obj.GetName(), obj.GetNamespace())
		r.TokenCache.DeleteEventsForObject(fluxcdv1.ResourceSetInputProviderKind,
			obj.GetName(), obj.GetNamespace(), cache.OperationReconcile)
		return nil
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	reporter.RecordMetrics(unstructured.Unstructured{Object: rawMap})
	return nil
}

// eventType is always corev1.EventTypeWarning for now, but later when we have
// drift detection we will need to use corev1.EventTypeNormal (the unparam
// linter directive fixes the linter error).
//
//nolint:unparam
func (r *ResourceSetInputProviderReconciler) notify(ctx context.Context, obj *fluxcdv1.ResourceSetInputProvider, eventType, reason, message string) {
	notifier.
		New(ctx, r.EventRecorder, r.Scheme, notifier.WithClient(r.Client)).
		Event(obj, eventType, reason, message)
}

// requeueAfterResourceSetInputProvider returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations, or the next schedule time if a schedule is
// defined and the next interval is not within the schedule window.
func requeueAfterResourceSetInputProvider(obj *fluxcdv1.ResourceSetInputProvider,
	scheduler *schedule.Scheduler, reconcileEnd time.Time) ctrl.Result {

	interval := obj.GetInterval()
	if interval == 0 { // If the interval is zero, the object is disabled.
		return ctrl.Result{}
	}

	requeueAfter := interval

	// Check if next interval is within the schedule window,
	// or if the next schedule is before the next interval.
	if scheduler != nil {
		nextInterval := reconcileEnd.Add(interval)
		nextSchedule := scheduler.Next(reconcileEnd)
		nextScheduleTime := nextSchedule.When.Time
		if !scheduler.ShouldScheduleInterval(nextInterval) || nextScheduleTime.Before(nextInterval) {
			obj.Status.NextSchedule = nextSchedule
			requeueAfter = nextScheduleTime.Sub(reconcileEnd)
		}
	}

	return ctrl.Result{RequeueAfter: requeueAfter}
}

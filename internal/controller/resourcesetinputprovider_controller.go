// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/git/github"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/secrets"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/crane"
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
	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
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
		exportedInput, err := fluxcdv1.NewResourceSetInput(defaults, map[string]any{
			"id": inputs.ID(string(obj.GetUID())),
		})
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

		// Call the external provider to get the exported inputs.
		exportedInputs, err = r.callExternalProvider(ctx, obj, defaults)
		if err != nil {
			msg := err.Error()
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

// callExternalProvider calls the external provider based on the object spec type
// and returns the list of ResourceSetInput objects.
func (r *ResourceSetInputProviderReconciler) callExternalProvider(
	ctx context.Context, obj *fluxcdv1.ResourceSetInputProvider,
	defaults map[string]any) ([]fluxcdv1.ResourceSetInput, error) {

	// Get the auth secret and data.
	var authSecret *corev1.Secret
	var authData map[string][]byte
	if obj.Spec.SecretRef != nil {
		key := types.NamespacedName{
			Name:      obj.Spec.SecretRef.Name,
			Namespace: obj.GetNamespace(),
		}
		var secret corev1.Secret
		if err := r.Client.Get(ctx, key, &secret); err != nil {
			return nil, fmt.Errorf("failed to read secret '%s': %w", key, err)
		}
		authSecret = &secret
		authData = authSecret.Data
	}

	// Get the TLS config.
	var tlsConfig *tls.Config
	if obj.Spec.CertSecretRef != nil {
		key := types.NamespacedName{
			Name:      obj.Spec.CertSecretRef.Name,
			Namespace: obj.GetNamespace(),
		}
		var err error
		const insecure = false
		tlsConfig, err = secrets.TLSConfigFromSecretRef(ctx, r.Client, key, obj.Spec.URL, insecure)
		if err != nil {
			return nil, err
		}
	}

	// Create the provider context with timeout.
	providerCtx, cancel := context.WithTimeout(ctx, obj.GetTimeout())
	defer cancel()

	var exportedInputs []fluxcdv1.ResourceSetInput

	switch {
	// Handle Git providers.
	case strings.HasPrefix(obj.Spec.Type, "Git") || strings.HasPrefix(obj.Spec.Type, "AzureDevOps"):
		// Create the provider based on the object type.
		provider, err := r.newGitProvider(providerCtx, obj, tlsConfig, authData)
		if err != nil {
			return nil, fmt.Errorf("failed to create Git provider: %w", err)
		}

		// Call the provider.
		exportedInputs, err = r.callGitProvider(providerCtx, obj, provider, defaults)
		if err != nil {
			return nil, fmt.Errorf("failed to call Git provider: %w", err)
		}
	// Handle OCI providers.
	case strings.HasSuffix(obj.Spec.Type, "ArtifactTag"):
		var err error
		exportedInputs, err = r.callOCIProvider(providerCtx, obj, tlsConfig, authSecret, defaults)
		if err != nil {
			return nil, fmt.Errorf("failed to call OCI provider: %w", err)
		}
	}

	return exportedInputs, nil
}

// newGitProvider returns a new Git provider based on the type specified in the ResourceSetInputProvider object.
func (r *ResourceSetInputProviderReconciler) newGitProvider(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	tlsConfig *tls.Config,
	authData map[string][]byte) (gitprovider.Interface, error) {

	// For Git providers, we currently return an error if the certSecretRef is set
	// but the ca.crt key is missing.
	// TODO(matheuscscp): Remove this restriction when we support mTLS for Git providers.
	var certPool *x509.CertPool
	if obj.Spec.CertSecretRef != nil && tlsConfig.RootCAs == nil {
		return nil, fmt.Errorf("invalid secret '%s/%s': key 'ca.crt' is missing",
			obj.GetNamespace(), obj.Spec.CertSecretRef.Name)
	}
	if tlsConfig != nil {
		certPool = tlsConfig.RootCAs
	}

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
	case strings.HasPrefix(obj.Spec.Type, "AzureDevOps"):
		token, err := r.getAzureDevOpsToken(ctx, obj, authData)
		if err != nil {
			return nil, err
		}
		return gitprovider.NewAzureDevOpsProvider(ctx, gitprovider.Options{
			URL:      obj.Spec.URL,
			CertPool: certPool,
			Token:    token,
		})
	default:
		return nil, fmt.Errorf("unsupported type: %s", obj.Spec.Type)
	}
}

// makeFilters returns the *filtering.Filters based on the
// object spec filters, using the default limit.
func (r *ResourceSetInputProviderReconciler) makeFilters(
	obj *fluxcdv1.ResourceSetInputProvider) (*filtering.Filters, error) {

	limit := obj.GetFilterLimit()

	if obj.Spec.Filter == nil {
		return &filtering.Filters{Limit: limit}, nil
	}

	filters := &filtering.Filters{
		Limit:  limit,
		Labels: obj.Spec.Filter.Labels,
	}

	// Regular expressions.
	if obj.Spec.Filter.IncludeBranch != "" {
		inRx, err := regexp.Compile(obj.Spec.Filter.IncludeBranch)
		if err != nil {
			return nil, fmt.Errorf("invalid includeBranch regex: %w", err)
		}
		filters.Include = inRx
	}
	if obj.Spec.Filter.IncludeTag != "" {
		inRx, err := regexp.Compile(obj.Spec.Filter.IncludeTag)
		if err != nil {
			return nil, fmt.Errorf("invalid includeTag regex: %w", err)
		}
		filters.Include = inRx
	}
	if obj.Spec.Filter.ExcludeBranch != "" {
		exRx, err := regexp.Compile(obj.Spec.Filter.ExcludeBranch)
		if err != nil {
			return nil, fmt.Errorf("invalid excludeBranch regex: %w", err)
		}
		filters.Exclude = exRx
	}
	if obj.Spec.Filter.ExcludeTag != "" {
		exRx, err := regexp.Compile(obj.Spec.Filter.ExcludeTag)
		if err != nil {
			return nil, fmt.Errorf("invalid excludeTag regex: %w", err)
		}
		filters.Exclude = exRx
	}

	// SemVer.
	if obj.Spec.Filter.Semver != "" {
		constraints, err := semver.NewConstraint(obj.Spec.Filter.Semver)
		if err != nil {
			return nil, fmt.Errorf("invalid semver expression: %w", err)
		}
		filters.SemVer = constraints
	}

	return filters, nil
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

// callGitProvider calls the git provider based on the object spec type
// and returns the list of ResourceSetInput objects.
func (r *ResourceSetInputProviderReconciler) callGitProvider(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider, provider gitprovider.Interface,
	defaults map[string]any) ([]fluxcdv1.ResourceSetInput, error) {

	var inputSet []fluxcdv1.ResourceSetInput

	filters, err := r.makeFilters(obj)
	if err != nil {
		return nil, err
	}

	opts := gitprovider.Options{
		URL:     obj.Spec.URL,
		Filters: *filters,
	}

	var results []gitprovider.Result
	switch {
	case strings.HasSuffix(obj.Spec.Type, "Branch"):
		results, err = provider.ListBranches(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list branches: %w", err)
		}
	case strings.HasSuffix(obj.Spec.Type, "Tag"):
		results, err = provider.ListTags(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
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

		inputSet = append(inputSet, inputsWithDefaults...)
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

	if _, ok := authData[github.KeyAppID]; !ok {
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

// getAzureDevOpsToken returns the appropriate AzureDevOps token by reading the secrets in authData.
func (r *ResourceSetInputProviderReconciler) getAzureDevOpsToken(
	ctx context.Context, obj *fluxcdv1.ResourceSetInputProvider,
	authData map[string][]byte) (string, error) {

	switch {

	// Handle static authentication.
	case len(authData) > 0:
		_, password, err := r.getBasicAuth(obj, authData)
		return password, err

	// Handle workload identity.
	default:

		var opts []auth.Option

		// Configure service account.
		if obj.Spec.ServiceAccountName != "" {
			sa := client.ObjectKey{
				Name:      obj.Spec.ServiceAccountName,
				Namespace: obj.GetNamespace(),
			}
			opts = append(opts, auth.WithServiceAccount(sa, r.Client))
		}

		// Configure token cache.
		if r.TokenCache != nil {
			involvedObject := getInputProviderInvolvedObject(obj)
			opts = append(opts, auth.WithCache(*r.TokenCache, involvedObject))
		}

		// Get token.
		t, err := authutils.GetGitCredentials(ctx, azure.ProviderName, opts...)
		if err != nil {
			return "", err
		}
		return t.BearerToken, nil
	}
}

// callOCIProvider lists the tags of an OCI artifact repository
// and returns the list of ResourceSetInput objects.
func (r *ResourceSetInputProviderReconciler) callOCIProvider(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider, tlsConfig *tls.Config,
	authSecret *corev1.Secret, defaults map[string]any) ([]fluxcdv1.ResourceSetInput, error) {

	repo := strings.TrimPrefix(obj.Spec.URL, "oci://")

	// Build options.
	opts, err := r.buildOCIOptions(ctx, obj, repo, tlsConfig, authSecret)
	if err != nil {
		return nil, err
	}

	// Call OCI server.
	tags, err := crane.ListTags(repo, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list OCI tags for '%s': %w", repo, err)
	}

	// Filter tags.
	filters, err := r.makeFilters(obj)
	if err != nil {
		return nil, err
	}
	tags = filters.Tags(tags)

	// Export inputs.
	res := make([]fluxcdv1.ResourceSetInput, 0, len(tags))
	for _, tag := range tags {
		digest, err := crane.Digest(fmt.Sprintf("%s:%s", repo, tag), opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to get digest for tag '%s' in repository '%s': %w", tag, repo, err)
		}
		input, err := fluxcdv1.NewResourceSetInput(defaults, map[string]any{
			"id":     inputs.ID(tag),
			"tag":    tag,
			"digest": digest,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create ResourceSetInput for tag '%s': %w", tag, err)
		}
		res = append(res, input)
	}

	return res, nil
}

var inputProviderToCloudProvider = map[string]string{
	fluxcdv1.InputProviderACRArtifactTag: azure.ProviderName,
	fluxcdv1.InputProviderECRArtifactTag: aws.ProviderName,
	fluxcdv1.InputProviderGARArtifactTag: gcp.ProviderName,
}

// buildOCIOptions builds the crane options for the OCI artifact providers
// configuring authentication and TLS settings.
func (r *ResourceSetInputProviderReconciler) buildOCIOptions(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider, repo string, tlsConfig *tls.Config,
	authSecret *corev1.Secret) ([]crane.Option, error) {

	opts := []crane.Option{
		crane.WithContext(ctx),
	}

	switch {

	// Configure workload identity for cloud providers.
	case obj.Spec.Type != fluxcdv1.InputProviderOCIArtifactTag:
		var authOpts []auth.Option

		// Configure service account.
		if obj.Spec.ServiceAccountName != "" {
			sa := client.ObjectKey{
				Name:      obj.Spec.ServiceAccountName,
				Namespace: obj.GetNamespace(),
			}
			authOpts = append(authOpts, auth.WithServiceAccount(sa, r.Client))
		}

		// Configure token cache.
		if r.TokenCache != nil {
			involvedObject := getInputProviderInvolvedObject(obj)
			authOpts = append(authOpts, auth.WithCache(*r.TokenCache, involvedObject))
		}

		// Build authenticator.
		provider := inputProviderToCloudProvider[obj.Spec.Type]
		authenticator, err := authutils.GetArtifactRegistryCredentials(ctx, provider, repo, authOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to get artifact registry credentials for '%s', provider '%s': %w",
				repo, provider, err)
		}
		opts = append(opts, crane.WithAuth(authenticator))

	// Configure generic OCI artifact provider.
	default:
		var pullSecrets []corev1.Secret

		// Add pull secret from the object spec.
		if authSecret != nil {
			pullSecrets = append(pullSecrets, *authSecret)
		}

		// Add pull secrets from the service account.
		if obj.Spec.ServiceAccountName != "" {
			key := types.NamespacedName{
				Name:      obj.Spec.ServiceAccountName,
				Namespace: obj.GetNamespace(),
			}
			s, err := secrets.PullSecretsFromServiceAccountRef(ctx, r.Client, key)
			if err != nil {
				return nil, err
			}
			pullSecrets = append(pullSecrets, s...)
		}

		// Configure pull secrets.
		if len(pullSecrets) > 0 {
			keychain, err := kauth.NewFromPullSecrets(ctx, pullSecrets)
			if err != nil {
				return nil, fmt.Errorf("failed to create OCI keychain: %w", err)
			}
			opts = append(opts, crane.WithAuthFromKeychain(keychain))
		}

		// Configure TLS settings.
		if tlsConfig != nil {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = tlsConfig
			opts = append(opts, crane.WithTransport(transport))
		}
	}

	return opts, nil
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

// getInputProviderInvolvedObject returns the involved object for the input provider
// for cache operations.
func getInputProviderInvolvedObject(obj *fluxcdv1.ResourceSetInputProvider) cache.InvolvedObject {
	return cache.InvolvedObject{
		Kind:      fluxcdv1.ResourceSetInputProviderKind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Operation: cache.OperationReconcile,
	}
}

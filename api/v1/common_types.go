// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EnabledValue  = "enabled"
	DisabledValue = "disabled"

	ReconciliationDisabledReason  = "ReconciliationDisabled"
	ReconciliationDisabledMessage = "Reconciliation is disabled"

	HealthCheckExpr = "status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == metadata.generation)"
)

var (
	Finalizer                        = fmt.Sprintf("%s/finalizer", GroupVersion.Group)
	ReconcileAnnotation              = fmt.Sprintf("%s/reconcile", GroupVersion.Group)
	ReconcileEveryAnnotation         = fmt.Sprintf("%s/reconcileEvery", GroupVersion.Group)
	ReconcileArtifactEveryAnnotation = fmt.Sprintf("%s/reconcileArtifactEvery", GroupVersion.Group)
	ReconcileTimeoutAnnotation       = fmt.Sprintf("%s/reconcileTimeout", GroupVersion.Group)
	PruneAnnotation                  = fmt.Sprintf("%s/prune", GroupVersion.Group)
	ForceAnnotation                  = fmt.Sprintf("%s/force", GroupVersion.Group)
	RevisionAnnotation               = fmt.Sprintf("%s/revision", GroupVersion.Group)
	CopyFromAnnotation               = fmt.Sprintf("%s/copyFrom", GroupVersion.Group)
)

// FluxObject is the interface that all Flux objects must implement.
//
// +k8s:deepcopy-gen=false
type FluxObject interface {
	client.Object

	// GetConditions returns a slice of metav1.Condition.
	GetConditions() []metav1.Condition

	// SetConditions sets the status conditions on the object.
	SetConditions([]metav1.Condition)

	// SetLastHandledReconcileAt sets the last handled reconcile time in the status.
	SetLastHandledReconcileAt(value string)

	// IsDisabled returns true if the object has the reconcile annotation set to 'disabled'.
	IsDisabled() bool

	// GetInterval returns the interval at which the object should be reconciled.
	GetInterval() time.Duration
}

// InputProvider is the interface that the ResourceSet
// input providers must implement.
//
// +k8s:deepcopy-gen=false
type InputProvider interface {
	GetInputs() ([]map[string]any, error)
	GetNamespace() string
	GetName() string
	GroupVersionKind() schema.GroupVersionKind
}

// CommonMetadata defines the common labels and annotations.
type CommonMetadata struct {
	// Annotations to be added to the object's metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels to be added to the object's metadata.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// Flux APIs Kind and Group constants.
const (
	FluxKustomizeGroup    = "kustomize.toolkit.fluxcd.io"
	FluxKustomizationKind = "Kustomization"

	FluxHelmGroup       = "helm.toolkit.fluxcd.io"
	FluxHelmReleaseKind = "HelmRelease"

	FluxSourceGroup          = "source.toolkit.fluxcd.io"
	FluxGitRepositoryKind    = "GitRepository"
	FluxBucketKind           = "Bucket"
	FluxOCIRepositoryKind    = "OCIRepository"
	FluxHelmRepositoryKind   = "HelmRepository"
	FluxHelmChartKind        = "HelmChart"
	FluxExternalArtifactKind = "ExternalArtifact"

	FluxNotificationGroup = "notification.toolkit.fluxcd.io"
	FluxAlertKind         = "Alert"
	FluxAlertProviderKind = "Provider"
	FluxReceiverKind      = "Receiver"

	FluxImageGroup                = "image.toolkit.fluxcd.io"
	FluxImageRepositoryKind       = "ImageRepository"
	FluxImagePolicyKind           = "ImagePolicy"
	FluxImageUpdateAutomationKind = "ImageUpdateAutomation"

	FluxSourceExtensionsGroup = "source.extensions.fluxcd.io"
	FluxArtifactGeneratorKind = "ArtifactGenerator"
)

// FluxKindInfo holds information about a Flux resource kind.
type FluxKindInfo struct {
	// Name is the singular name of the resource kind.
	Name string

	// Plural is the plural name of the resource kind used for RBAC.
	Plural string

	// ShortName is a short alias for the resource kind.
	ShortName string

	// Reconcilable indicates if the resource kind supports reconciliation operations.
	Reconcilable bool
}

// FluxOperatorKinds is a list of all supported Flux Operator kinds and their properties.
var FluxOperatorKinds = []FluxKindInfo{
	// Flux lifecycle management
	{Name: FluxInstanceKind, Plural: "fluxinstances", ShortName: "instance", Reconcilable: true},
	{Name: FluxReportKind, Plural: "fluxreports", ShortName: "report", Reconcilable: true},

	// App lifecycle management
	{Name: ResourceSetKind, Plural: "resourcesets", ShortName: "rset", Reconcilable: true},
	{Name: ResourceSetInputProviderKind, Plural: "resourcesetinputproviders", ShortName: "rsip", Reconcilable: true},
}

// FluxKinds is a list of all supported Flux kinds and their properties.
var FluxKinds = []FluxKindInfo{
	// Flux appliers
	{Name: FluxKustomizationKind, Plural: "kustomizations", ShortName: "ks", Reconcilable: true},
	{Name: FluxHelmReleaseKind, Plural: "helmreleases", ShortName: "hr", Reconcilable: true},

	// Flux sources
	{Name: FluxGitRepositoryKind, Plural: "gitrepositories", ShortName: "gitrepo", Reconcilable: true},
	{Name: FluxOCIRepositoryKind, Plural: "ocirepositories", ShortName: "ocirepo", Reconcilable: true},
	{Name: FluxBucketKind, Plural: "buckets", ShortName: "bucket", Reconcilable: true},
	{Name: FluxHelmRepositoryKind, Plural: "helmrepositories", ShortName: "helmrepo", Reconcilable: true},
	{Name: FluxHelmChartKind, Plural: "helmcharts", ShortName: "hc", Reconcilable: true},
	{Name: FluxArtifactGeneratorKind, Plural: "artifactgenerators", ShortName: "ag", Reconcilable: true},
	{Name: FluxExternalArtifactKind, Plural: "externalartifacts", ShortName: "ea", Reconcilable: false},

	// Flux notifications
	{Name: FluxAlertKind, Plural: "alerts", ShortName: "alert", Reconcilable: false},
	{Name: FluxAlertProviderKind, Plural: "providers", ShortName: "provider", Reconcilable: false},
	{Name: FluxReceiverKind, Plural: "receivers", ShortName: "receiver", Reconcilable: true},

	// Flux image automation
	{Name: FluxImageRepositoryKind, Plural: "imagerepositories", ShortName: "imgrepo", Reconcilable: true},
	{Name: FluxImagePolicyKind, Plural: "imagepolicies", ShortName: "imgpol", Reconcilable: true},
	{Name: FluxImageUpdateAutomationKind, Plural: "imageupdateautomations", ShortName: "imgauto", Reconcilable: true},
}

// FluxGroupFor returns the GroupKind for the given kind.
func FluxGroupFor(kind string) (*schema.GroupKind, error) {
	gk := schema.GroupKind{
		Kind: kind,
	}

	switch kind {
	case FluxInstanceKind, FluxReportKind, ResourceSetKind, ResourceSetInputProviderKind:
		gk.Group = GroupVersion.Group
	case FluxGitRepositoryKind, FluxOCIRepositoryKind, FluxBucketKind, FluxHelmChartKind, FluxHelmRepositoryKind, FluxExternalArtifactKind:
		gk.Group = FluxSourceGroup
	case FluxAlertKind, FluxAlertProviderKind, FluxReceiverKind:
		gk.Group = FluxNotificationGroup
	case FluxImageRepositoryKind, FluxImagePolicyKind, FluxImageUpdateAutomationKind:
		gk.Group = FluxImageGroup
	case FluxKustomizationKind:
		gk.Group = FluxKustomizeGroup
	case FluxHelmReleaseKind:
		gk.Group = FluxHelmGroup
	case FluxArtifactGeneratorKind:
		gk.Group = FluxSourceExtensionsGroup
	default:
		return nil, fmt.Errorf("unknown kind %s", kind)
	}
	return &gk, nil
}

// IsFluxAPI checks if the given API version or API group
// is part of the Flux API group.
func IsFluxAPI(apiVersion string) bool {
	return strings.Contains(apiVersion, "fluxcd.")
}

// FindFluxKindInfo searches for a FluxKindInfo in a case-insensitive way.
// It matches both Name and ShortName and returns the corresponding FluxKindInfo.
// Returns an error if the kind is not found in the fluxKinds list.
func FindFluxKindInfo(kind string) (*FluxKindInfo, error) {
	fluxKinds := append(FluxOperatorKinds, FluxKinds...)
	for _, fluxKind := range fluxKinds {
		if strings.EqualFold(fluxKind.Name, kind) {
			return &fluxKind, nil
		}
		if strings.EqualFold(fluxKind.ShortName, kind) {
			return &fluxKind, nil
		}
	}
	return nil, fmt.Errorf("kind %s not found", kind)
}

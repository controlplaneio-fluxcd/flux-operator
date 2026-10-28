// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
)

const (
	DefaultInstanceName = "flux"
	DefaultNamespace    = "flux-system"
	FluxInstanceKind    = "FluxInstance"
	OutdatedReason      = "OutdatedVersion"
)

// FluxInstanceSpec defines the desired state of FluxInstance
// +kubebuilder:validation:XValidation:rule="!has(self.sharding) || !has(self.sharding.storage) || self.sharding.storage != 'persistent' || has(self.storage)",message=".spec.storage must be set when .spec.sharding.storage is 'persistent'"
// +kubebuilder:validation:XValidation:rule="!has(self.cluster) || !has(self.cluster.multitenantObjectIdentity) || !self.cluster.multitenantObjectIdentity || (has(self.cluster.objectIdentityRBAC) && self.cluster.objectIdentityRBAC) || (has(self.cluster.objectLevelWorkloadIdentity) && self.cluster.objectLevelWorkloadIdentity) || (has(self.spiffe) && has(self.spiffe.brokerAPI))",message=".spec.cluster.objectIdentityRBAC or .spec.spiffe.brokerAPI must be set when .spec.cluster.multitenantObjectIdentity is set to true"
type FluxInstanceSpec struct {
	// Distribution specifies the version and container registry to pull images from.
	// +required
	Distribution Distribution `json:"distribution"`

	// Components is the list of controllers to install.
	// Defaults to the core Flux controllers:
	//   - source-controller
	//   - kustomize-controller
	//   - helm-controller
	//   - notification-controller
	// +optional
	Components []Component `json:"components,omitempty"`

	// CommonMetadata specifies the common labels and annotations that are
	// applied to all resources. Any existing label or annotation will be
	// overridden if its key matches a common one.
	// +optional
	CommonMetadata *CommonMetadata `json:"commonMetadata,omitempty"`

	// Cluster holds the specification of the Kubernetes cluster.
	// +optional
	Cluster *Cluster `json:"cluster,omitempty"`

	// SPIFFE holds the configuration for the SPIFFE integration.
	// When set, the operator configures the Flux controllers to use
	// the SPIFFE Workload API and optionally enables object identity
	// and mTLS between the controllers.
	// The flux-operator pod must also be configured with the same Workload API
	// and, when used, Broker API settings, but does not need the inter-controller
	// flags.
	// +optional
	SPIFFE *SPIFFE `json:"spiffe,omitempty"`

	// Sharding holds the specification of the sharding configuration.
	// +optional
	Sharding *Sharding `json:"sharding,omitempty"`

	// Storage holds the specification of the source-controller
	// persistent volume claim.
	// +optional
	Storage *Storage `json:"storage,omitempty"`

	// Kustomize holds a set of patches that can be applied to the
	// Flux installation, to customize the way Flux operates.
	// +optional
	Kustomize *Kustomize `json:"kustomize,omitempty"`

	// Wait instructs the controller to check the health of all the reconciled
	// resources. Defaults to true.
	// +kubebuilder:default:=true
	// +optional
	Wait *bool `json:"wait,omitempty"`

	// MigrateResources instructs the controller to migrate the Flux custom resources
	// from the previous version to the latest API version specified in the CRD.
	// Defaults to true.
	// +kubebuilder:default:=true
	// +optional
	MigrateResources *bool `json:"migrateResources,omitempty"`

	// Sync specifies the source for the cluster sync operation.
	// When set, a Flux source (GitRepository, OCIRepository or Bucket)
	// and Flux Kustomization are created to sync the cluster state
	// with the source repository.
	// +optional
	Sync *Sync `json:"sync,omitempty"`
}

// Distribution specifies the version and container registry to pull images from.
type Distribution struct {
	// Version semver expression e.g. '2.x', '2.3.x'.
	// +required
	Version string `json:"version"`

	// Registry address to pull the distribution images from
	// e.g. 'ghcr.io/fluxcd'.
	// +required
	Registry string `json:"registry"`

	// Variant specifies the Flux distribution flavor stored
	// in the registry.
	// +kubebuilder:validation:Enum=upstream-alpine;enterprise-alpine;enterprise-distroless;enterprise-distroless-fips
	// +optional
	Variant string `json:"variant,omitempty"`

	// ImagePullSecret is the name of the Kubernetes secret
	// to use for pulling images.
	// +optional
	ImagePullSecret string `json:"imagePullSecret,omitempty"`

	// Artifact is the URL to the OCI artifact containing
	// the latest Kubernetes manifests for the distribution,
	// e.g. 'oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest'.
	// +kubebuilder:validation:Pattern="^oci://.*$"
	// +optional
	Artifact string `json:"artifact,omitempty"`

	// ArtifactPullSecret is the name of the Kubernetes secret
	// to use for pulling the Kubernetes manifests for the distribution specified in the Artifact field.
	// +optional
	ArtifactPullSecret string `json:"artifactPullSecret,omitempty"`
}

// Component is the name of a controller to install.
// +kubebuilder:validation:Enum:=source-controller;kustomize-controller;helm-controller;notification-controller;image-reflector-controller;image-automation-controller;source-watcher
type Component string

// ComponentImage represents a container image used by a component.
type ComponentImage struct {
	// Name of the component.
	// +required
	Name string `json:"name"`

	// Repository address of the container image.
	// +required
	Repository string `json:"repository"`

	// Tag of the container image.
	// +required
	Tag string `json:"tag"`

	// Digest of the container image.
	// +optional
	Digest string `json:"digest,omitempty"`
}

// Cluster is the specification for the Kubernetes cluster.
// +kubebuilder:validation:XValidation:rule="!(has(self.objectLevelWorkloadIdentity) && has(self.objectIdentityRBAC))", message="cannot set .objectIdentityRBAC together with a legacy object identity field"
// +kubebuilder:validation:XValidation:rule="!(has(self.multitenantWorkloadIdentity) && self.multitenantWorkloadIdentity && has(self.multitenantObjectIdentity) && self.multitenantObjectIdentity)", message="cannot set .multitenantObjectIdentity together with a legacy multitenant object identity field"
// +kubebuilder:validation:XValidation:rule="(has(self.objectLevelWorkloadIdentity) && self.objectLevelWorkloadIdentity) || (has(self.objectIdentityRBAC) && self.objectIdentityRBAC) || (!has(self.multitenantWorkloadIdentity) || !self.multitenantWorkloadIdentity)", message=".objectIdentityRBAC must be set to true when multitenant workload identity is enabled"
type Cluster struct {
	// Domain is the cluster domain used for generating the FQDN of services.
	// Defaults to 'cluster.local'.
	// +kubebuilder:default:=cluster.local
	// +optional
	Domain string `json:"domain,omitempty"`

	// Multitenant enables the multitenancy lockdown. Defaults to false.
	// +kubebuilder:default:=false
	// +optional
	Multitenant bool `json:"multitenant,omitempty"`

	// MultitenantWorkloadIdentity is now legacy.
	//
	// Legacy: Use MultitenantObjectIdentity instead.
	// +kubebuilder:default:=false
	// +optional
	MultitenantWorkloadIdentity bool `json:"multitenantWorkloadIdentity,omitempty"`

	// MultitenantObjectIdentity enables the multitenancy lockdown for
	// object identity. Defaults to false.
	// +kubebuilder:default:=false
	// +optional
	MultitenantObjectIdentity bool `json:"multitenantObjectIdentity,omitempty"`

	// TenantDefaultServiceAccount is the name of the service account
	// to use as default when the multitenant lockdown is enabled, for
	// kustomize-controller and helm-controller.
	// This field will also be used for multitenant object identity
	// lockdown for source-controller, notification-controller,
	// image-reflector-controller and image-automation-controller.
	// Defaults to the 'default' service account from the tenant namespace.
	// +optional
	TenantDefaultServiceAccount string `json:"tenantDefaultServiceAccount,omitempty"`

	// TenantDefaultDecryptionServiceAccount is the name of the service account
	// to use as default for kustomize-controller SOPS decryption when the
	// multitenant lockdown for object identity is enabled. Defaults to the
	// 'default' service account from the tenant namespace.
	// +optional
	TenantDefaultDecryptionServiceAccount string `json:"tenantDefaultDecryptionServiceAccount,omitempty"`

	// TenantDefaultKubeConfigServiceAccount is the name of the service account
	// to use as default for kustomize-controller and helm-controller remote
	// cluster access via spec.kubeConfig.configMapRef when the multitenant
	// lockdown for object identity is enabled. Defaults to the 'default'
	// service account from the tenant namespace.
	// +optional
	TenantDefaultKubeConfigServiceAccount string `json:"tenantDefaultKubeConfigServiceAccount,omitempty"`

	// ObjectLevelWorkloadIdentity is now legacy.
	//
	// Legacy: Use ObjectIdentityRBAC instead.
	// +optional
	ObjectLevelWorkloadIdentity bool `json:"objectLevelWorkloadIdentity,omitempty"`

	// ObjectIdentityRBAC configures the RBAC required for object identity
	// using Kubernetes service account tokens. Object identity is the new
	// name of the upstream Flux object-level workload identity feature.
	// In Flux v2.6.0 to v2.9.x this enables the
	// ObjectLevelWorkloadIdentity feature gate and leaves the RBAC for
	// creating service account tokens in place. When disabled, the
	// operator removes that RBAC from the Flux controllers' ClusterRole.
	// Starting with Flux v2.10.0, the ObjectLevelWorkloadIdentity feature
	// gate is retired and no longer changes behavior, so the operator no
	// longer passes it to the controllers.
	// Starting with Flux v2.10.0, the RBAC for creating service account
	// tokens is removed from the Flux distribution, and this field
	// controls whether the operator adds it.
	// This feature is only available in Flux v2.6.0 and later.
	// +optional
	ObjectIdentityRBAC bool `json:"objectIdentityRBAC,omitempty"`

	// NetworkPolicy restricts network access to the current namespace.
	// Defaults to true.
	// +kubebuilder:default:=true
	// +optional
	NetworkPolicy bool `json:"networkPolicy"`

	// Type specifies the distro of the Kubernetes cluster.
	// Defaults to 'kubernetes'.
	// +kubebuilder:validation:Enum:=kubernetes;openshift;aws;azure;gcp
	// +kubebuilder:default:=kubernetes
	// +optional
	Type string `json:"type,omitempty"`

	// Size defines the vertical scaling profile of the Flux controllers.
	// The size is used to determine the concurrency and CPU/Memory limits for the Flux controllers.
	// Accepted values are: 'small', 'medium' and 'large'.
	// +kubebuilder:validation:Enum:=small;medium;large
	// +optional
	Size string `json:"size,omitempty"`
}

// SPIFFE holds the configuration for the SPIFFE integration.
type SPIFFE struct {
	// WorkloadAPI holds the configuration for the SPIFFE Workload API.
	// It is required for enabling any of the SPIFFE features and is used
	// to render the go-spiffe environment variable and the SPIFFE CSI volume.
	// +required
	WorkloadAPI WorkloadAPI `json:"workloadAPI"`

	// BrokerAPI holds the configuration for the SPIFFE Broker API.
	// When set, enables the object-level SPIFFE features.
	// +optional
	BrokerAPI *BrokerAPI `json:"brokerAPI,omitempty"`

	// MTLS holds the configuration for mTLS between the Flux controllers.
	// The flux-operator reads the controller SPIFFE IDs from .status.spiffe.
	// +optional
	MTLS *MTLS `json:"mtls,omitempty"`
}

// WorkloadAPI holds the configuration for the SPIFFE Workload API.
// +kubebuilder:validation:XValidation:rule="!has(self.csiDriver) || ((self.endpoint.startsWith('unix:///spiffe-workload-api/') || self.endpoint.startsWith('unix:/spiffe-workload-api/')) && !self.endpoint.matches('(^|/)[.][.](/|$)'))",message="endpoint must be a unix socket under /spiffe-workload-api/ without '..' segments when csiDriver is set, as the CSI volume is mounted at /spiffe-workload-api"
type WorkloadAPI struct {
	// Endpoint is the address of the SPIFFE Workload API endpoint,
	// in the form 'unix:///<absolute path>', 'unix:/<absolute path>'
	// or 'tcp://<ip>:<port>',
	// e.g. 'unix:///spiffe-workload-api/agent.sock'.
	// The flux-operator pod must be configured with the same endpoint
	// through the SPIFFE_ENDPOINT_SOCKET environment variable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern="^(?:unix:(?:///[!$&->@-~]+|/[!$&-.0->@-~][!$&->@-~]*)|tcp://(?:[0-9]{1,3}(?:\\.[0-9]{1,3}){3}|\\[[0-9a-fA-F:.]+\\]):[0-9]{1,5})$"
	// +required
	Endpoint string `json:"endpoint"`

	// CSIDriver is the name of the SPIFFE CSI driver used to mount
	// the Workload API socket into the Flux controller pods.
	// When set, the operator mounts the CSI volume at /spiffe-workload-api
	// and the endpoint must be a unix socket under that path.
	// When omitted, no CSI volume is configured.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9](?:[-a-zA-Z0-9]*[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[-a-zA-Z0-9]*[a-zA-Z0-9])?)*$"
	// +optional
	CSIDriver string `json:"csiDriver,omitempty"`
}

// BrokerAPI holds the configuration for the SPIFFE Broker API.
type BrokerAPI struct {
	// Endpoint is the address of the SPIFFE Broker API endpoint,
	// e.g. 'dns:///spiffe-broker.example.com:443'.
	// The flux-operator pod accepts the same --spiffe-broker-endpoint flag
	// as the Flux controllers. For OLM installations, the endpoint can be
	// set with the SPIFFE_BROKER_ENDPOINT environment variable.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern="^[!-~]+$"
	// +required
	Endpoint string `json:"endpoint"`

	// IDs is the list of SPIFFE IDs or SPIFFE ID prefixes authorized
	// for the SPIFFE Broker API endpoint. SPIFFE IDs ending with a slash
	// are treated as prefixes.
	// The flux-operator pod accepts the same repeatable --spiffe-broker-id
	// flag as the Flux controllers. For OLM installations, the IDs can be
	// set with the SPIFFE_BROKER_IDS environment variable as a comma-separated
	// list.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern="^(?:spiffe://[a-z0-9._-]{1,255}|spiffe://[a-z0-9._-]{1,255}(?:/(?:[a-zA-Z0-9_-]|[.][a-zA-Z0-9_-])[a-zA-Z0-9._-]*)+(?:/)?)$"
	// +required
	IDs []string `json:"ids"`
}

// MTLS holds the configuration for mTLS between the Flux controllers.
type MTLS struct {
	// Enabled enables mTLS between the Flux controllers.
	// +required
	Enabled bool `json:"enabled"`

	// ComponentIDTemplate is the SPIFFE ID template for the Flux controllers.
	// The '{component}' placeholder is replaced with the name of each
	// installed component. A trailing slash indicates a SPIFFE ID prefix.
	// The flux-operator ID is derived from this template with 'flux-operator'
	// as the component name, and is only authorized when the operator has a
	// SPIFFE Workload API source. The template must resolve to the operator
	// pod's actual SPIFFE ID.
	// When the operator has a SPIFFE Workload API source, its ID is added to
	// notification-controller as an authorized notifier. When the operator's
	// embedded Web UI is enabled, it is also added to source-controller and
	// source-watcher as an authorized artifact client for the Web UI artifact
	// downloads. Enabling or disabling the embedded Web UI changes these
	// controller arguments and rolls out the affected controllers.
	// For a standalone Web UI, use ExtraArtifactClients and ExtraNotifiers.
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern="^spiffe://[a-z0-9._-]{1,255}(?:/(?:[a-zA-Z0-9_-]|[.][a-zA-Z0-9_-]|[{]component[}])[a-zA-Z0-9._-]*(?:[{]component[}][a-zA-Z0-9._-]*)*)+(?:/)?$"
	// +kubebuilder:validation:XValidation:rule="self.contains('{component}')",message="componentIDTemplate must contain the '{component}' placeholder"
	// +required
	ComponentIDTemplate string `json:"componentIDTemplate"`

	// Receivers is the list of SPIFFE IDs of external Ingress or Gateway
	// controllers authorized to send webhook requests to notification-controller.
	// Setting at least one ID makes the receiver accept only SPIFFE mTLS clients.
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern="^(?:spiffe://[a-z0-9._-]{1,255}|spiffe://[a-z0-9._-]{1,255}(?:/(?:[a-zA-Z0-9_-]|[.][a-zA-Z0-9_-])[a-zA-Z0-9._-]*)+(?:/)?)$"
	// +optional
	Receivers []string `json:"receivers,omitempty"`

	// ExtraArtifactClients is a list of additional SPIFFE IDs added to
	// source-controller and source-watcher as authorized artifact clients,
	// e.g. the standalone Web UI or the Flux MCP Server.
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern="^(?:spiffe://[a-z0-9._-]{1,255}|spiffe://[a-z0-9._-]{1,255}(?:/(?:[a-zA-Z0-9_-]|[.][a-zA-Z0-9_-])[a-zA-Z0-9._-]*)+(?:/)?)$"
	// +optional
	ExtraArtifactClients []string `json:"extraArtifactClients,omitempty"`

	// ExtraArtifactServers is a list of additional SPIFFE IDs added to
	// kustomize-controller, helm-controller and source-watcher as authorized
	// artifact servers, e.g. an external source controller.
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern="^(?:spiffe://[a-z0-9._-]{1,255}|spiffe://[a-z0-9._-]{1,255}(?:/(?:[a-zA-Z0-9_-]|[.][a-zA-Z0-9_-])[a-zA-Z0-9._-]*)+(?:/)?)$"
	// +optional
	ExtraArtifactServers []string `json:"extraArtifactServers,omitempty"`

	// ExtraNotifiers is a list of additional SPIFFE IDs added to
	// notification-controller as authorized notifiers, e.g. the standalone
	// Web UI or the Flux MCP Server.
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern="^(?:spiffe://[a-z0-9._-]{1,255}|spiffe://[a-z0-9._-]{1,255}(?:/(?:[a-zA-Z0-9_-]|[.][a-zA-Z0-9_-])[a-zA-Z0-9._-]*)+(?:/)?)$"
	// +optional
	ExtraNotifiers []string `json:"extraNotifiers,omitempty"`
}

type Sharding struct {
	// Key is the label key used to shard the resources.
	// +kubebuilder:default:=sharding.fluxcd.io/key
	// +optional
	Key string `json:"key,omitempty"`

	// Shards is the list of shard names.
	// +kubebuilder:validation:MinItems=1
	// +required
	Shards []string `json:"shards"`

	// Storage defines if the source-controller shards
	// should use an emptyDir or a persistent volume claim for storage.
	// Accepted values are 'ephemeral' or 'persistent', defaults to 'ephemeral'.
	// When set to 'persistent', the '.spec.storage' field must be set.
	// +kubebuilder:validation:Enum:=ephemeral;persistent
	// +optional
	Storage string `json:"storage,omitempty"`
}

// Storage is the specification for the persistent volume claim.
type Storage struct {
	// Class is the storage class to use for the PVC.
	// +required
	Class string `json:"class"`

	// Size is the size of the PVC.
	// +required
	Size string `json:"size"`
}

// Kustomize holds a set of patches that can be applied to the
// Flux installation, to customize the way Flux operates.
type Kustomize struct {
	// Strategic merge and JSON patches, defined as inline YAML objects,
	// capable of targeting objects based on kind, label and annotation selectors.
	// +optional
	Patches []kustomize.Patch `json:"patches,omitempty"`
}

// Sync defines the source and path for the Flux instance synchronization.
// +kubebuilder:validation:XValidation:rule="!has(self.provider) || self.provider != 'gcp' || self.kind == 'OCIRepository' || self.kind == 'Bucket'",message="sync.provider 'gcp' is only supported for OCIRepository and Bucket"
// +kubebuilder:validation:XValidation:rule="!has(self.provider) || self.provider != 'github' || self.kind == 'GitRepository'",message="sync.provider 'github' is only supported for GitRepository"
type Sync struct {
	// Name is the name of the Flux source and kustomization resources.
	// When not specified, the name is set to the namespace name of the FluxInstance.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Sync name is immutable"
	// +kubebuilder:validation:MaxLength=63
	// +optional
	Name string `json:"name,omitempty"`

	// Interval is the time between syncs.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +kubebuilder:default:="1m"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Kind is the kind of the source.
	// +kubebuilder:validation:Enum=OCIRepository;GitRepository;Bucket
	// +required
	Kind string `json:"kind"`

	// URL is the source URL, can be a Git repository HTTP/S or SSH address,
	// an OCI repository address or a Bucket endpoint.
	// +required
	URL string `json:"url"`

	// Ref is the source reference, can be a Git ref name e.g. 'refs/heads/main',
	// an OCI tag e.g. 'latest' or a bucket name e.g. 'flux'.
	// +required
	Ref string `json:"ref"`

	// Path is the path to the source directory containing
	// the kustomize overlay or plain Kubernetes manifests.
	// +required
	Path string `json:"path"`

	// PullSecret specifies the Kubernetes Secret containing the
	// authentication credentials for the source.
	// For Git over HTTP/S sources, the secret must contain username and password fields.
	// For Git over SSH sources, the secret must contain known_hosts and identity fields.
	// For OCI sources, the secret must be of type kubernetes.io/dockerconfigjson.
	// For Bucket sources, the secret must contain accesskey and secretkey fields.
	// +optional
	PullSecret string `json:"pullSecret,omitempty"`

	// Provider specifies OIDC provider for source authentication.
	// For OCIRepository and Bucket the provider can be set to 'aws', 'azure' or 'gcp'.
	// For GitRepository the provider can be set to 'aws' (requires Flux 2.9 or later),
	// 'azure' or 'github'.
	// To disable OIDC authentication the provider can be set to 'generic' or left empty.
	// +kubebuilder:validation:Enum=generic;aws;azure;gcp;github
	// +optional
	Provider string `json:"provider,omitempty"`
}

// ResourceInventory contains a list of Kubernetes resource object references
// that have been applied.
type ResourceInventory struct {
	// Entries of Kubernetes resource object references.
	Entries []ResourceRef `json:"entries"`
}

// ResourceRef contains the information necessary to locate a resource within a cluster.
type ResourceRef struct {
	// ID is the string representation of the Kubernetes resource object's metadata,
	// in the format '<namespace>_<name>_<group>_<kind>'.
	ID string `json:"id"`

	// Version is the API version of the Kubernetes resource object's kind.
	Version string `json:"v"`
}

// FluxInstanceStatus defines the observed state of FluxInstance
type FluxInstanceStatus struct {
	meta.ReconcileRequestStatus `json:",inline"`
	meta.ForceRequestStatus     `json:",inline"`

	// Conditions contains the readiness conditions of the object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastAttemptedRevision is the version and digest of the
	// distribution config that was last attempted to reconcile.
	// +optional
	LastAttemptedRevision string `json:"lastAttemptedRevision,omitempty"`

	// LastAppliedRevision is the version and digest of the
	// distribution config that was last reconcile.
	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty"`

	// LastArtifactRevision is the digest of the last pulled
	// distribution artifact.
	// +optional
	LastArtifactRevision string `json:"lastArtifactRevision,omitempty"`

	// Components contains the container images used by the components.
	// +optional
	Components []ComponentImage `json:"components,omitempty"`

	// Inventory contains a list of Kubernetes resource object references
	// last applied on the cluster.
	// +optional
	Inventory *ResourceInventory `json:"inventory,omitempty"`

	// SPIFFE holds the SPIFFE settings of the last applied distribution.
	// +optional
	SPIFFE *SPIFFEStatus `json:"spiffe,omitempty"`

	// History contains the reconciliation history of the FluxInstance
	// as a list of snapshots ordered by the last reconciled time.
	History History `json:"history,omitempty"`
}

// SPIFFEStatus holds the SPIFFE settings of the last applied distribution.
type SPIFFEStatus struct {
	// MTLS holds the applied mTLS settings.
	// +optional
	MTLS *MTLSStatus `json:"mtls,omitempty"`
}

// MTLSStatus holds the applied mTLS settings.
type MTLSStatus struct {
	// Enabled is true when the Flux controllers were deployed with mTLS.
	// +required
	Enabled bool `json:"enabled"`

	// ComponentIDTemplate is the SPIFFE ID template of the Flux controllers.
	// +optional
	ComponentIDTemplate string `json:"componentIDTemplate,omitempty"`

	// ExtraArtifactServers is the list of additional SPIFFE IDs
	// authorized as artifact servers.
	// +optional
	ExtraArtifactServers []string `json:"extraArtifactServers,omitempty"`
}

// GetDistribution returns the distribution specification with defaults.
func (in *FluxInstance) GetDistribution() Distribution {
	return in.Spec.Distribution
}

// GetComponents returns the components to install with defaults.
func (in *FluxInstance) GetComponents() []string {
	components := make([]string, len(in.Spec.Components))
	for i, c := range in.Spec.Components {
		components[i] = string(c)
	}
	if len(components) == 0 {
		components = []string{
			FluxSourceController,
			FluxKustomizeController,
			FluxHelmController,
			FluxNotificationController,
		}
	}

	return components
}

// GetCluster returns the cluster specification with defaults.
func (in *FluxInstance) GetCluster() Cluster {
	cluster := in.Spec.Cluster
	if cluster == nil {
		return Cluster{
			Type:          "kubernetes",
			Domain:        "cluster.local",
			NetworkPolicy: true,
		}
	}

	return *cluster
}

// IsShardingStorageEnabled returns true if 'spec.sharding.storage' is set to 'persistent'.
func (in *FluxInstance) IsShardingStorageEnabled() bool {
	if in.Spec.Sharding == nil {
		return false
	}
	return in.Spec.Sharding.Storage == "persistent"
}

// GetMigrateResources returns the migration configuration with defaults.
func (in *FluxInstance) GetMigrateResources() bool {
	if in.Spec.MigrateResources == nil {
		return true
	}
	return *in.Spec.MigrateResources
}

// GetWait returns the wait configuration with defaults.
func (in *FluxInstance) GetWait() bool {
	if in.Spec.Wait == nil {
		return true
	}
	return *in.Spec.Wait
}

// GetConditions returns the status conditions of the object.
func (in *FluxInstance) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// GetLastHandledReconcileRequest returns the last handled reconcile request.
func (in *FluxInstance) GetLastHandledReconcileRequest() string {
	return in.Status.GetLastHandledReconcileRequest()
}

// GetLastHandledForceRequestStatus returns the last handled force request status.
func (in *FluxInstance) GetLastHandledForceRequestStatus() *string {
	return &in.Status.LastHandledForceAt
}

// SetConditions sets the status conditions on the object.
func (in *FluxInstance) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// SetLastHandledReconcileAt sets the last handled reconcile time in the status.
func (in *FluxInstance) SetLastHandledReconcileAt(value string) {
	in.Status.LastHandledReconcileAt = value
}

// IsDisabled returns true if the object has the reconcile annotation set to 'disabled'.
func (in *FluxInstance) IsDisabled() bool {
	val, ok := in.GetAnnotations()[ReconcileAnnotation]
	return ok && strings.ToLower(val) == DisabledValue
}

// GetInterval returns the interval at which the object should be reconciled.
// If no interval is set, the default is 60 minutes.
func (in *FluxInstance) GetInterval() time.Duration {
	if in.IsDisabled() {
		return 0
	}
	defaultInterval := 60 * time.Minute
	val, ok := in.GetAnnotations()[ReconcileEveryAnnotation]
	if !ok {
		return defaultInterval
	}
	interval, err := time.ParseDuration(val)
	if err != nil {
		return defaultInterval
	}
	return interval
}

// GetArtifactInterval returns the interval at which the distribution artifact should be reconciled.
// If no interval is set, the default is 10 minutes.
func (in *FluxInstance) GetArtifactInterval() time.Duration {
	if in.IsDisabled() {
		return 0
	}
	defaultInterval := 10 * time.Minute
	val, ok := in.GetAnnotations()[ReconcileArtifactEveryAnnotation]
	if !ok {
		return defaultInterval
	}
	interval, err := time.ParseDuration(val)
	if err != nil {
		return defaultInterval
	}
	return interval
}

// GetTimeout returns the timeout for the reconciliation process.
// If no timeout is set, the default is 5 minutes.
func (in *FluxInstance) GetTimeout() time.Duration {
	defaultTimeout := 5 * time.Minute
	val, ok := in.GetAnnotations()[ReconcileTimeoutAnnotation]
	if !ok {
		return defaultTimeout
	}
	timeout, err := time.ParseDuration(val)
	if err != nil {
		return defaultTimeout
	}
	return timeout
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=fluxcd;fluxcd-appliers
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".status.lastAttemptedRevision",description=""

// FluxInstance is the Schema for the fluxinstances API
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'flux'", message="the only accepted name for a FluxInstance is 'flux'"
type FluxInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluxInstanceSpec   `json:"spec,omitempty"`
	Status FluxInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FluxInstanceList contains a list of FluxInstance
type FluxInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluxInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluxInstance{}, &FluxInstanceList{})
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Flux Operator and deploy a Flux instance",
	Long: `The install command provides a quick way to bootstrap a Kubernetes cluster with the Flux Operator and a Flux instance.

The install command performs the following steps:
  1. Downloads the Flux Operator distribution artifact from 'oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests'.
  2. Installs the Flux Operator in the 'flux-system' namespace and waits for it to become ready.
  3. Installs the Flux instance in the 'flux-system' namespace according to the provided configuration.
  4. Configures the pull secret for the instance sync source if credentials are provided.
  4. Configures Flux to bootstrap the cluster from a Git repository or OCI repository if a sync URL is provided.
  5. Configures automatic updates of the Flux Operator from the distribution artifact.

This command is intended for development and testing purposes. For production installations,
it is recommended to follow the installation guide at https://fluxcd.control-plane.io/operator/install/
`,
	Example: `  # Install Flux Operator with custom Flux settings
  flux-operator install \
    --instance-distribution-version=2.7.x \
    --instance-components-extra=source-watcher \
    --instance-cluster-multitenant=true

  # Install and bootstrap from a Git repository
  flux-operator install \
    --instance-sync-url=https://github.com/org/fleet-infra \
    --instance-sync-ref=refs/heads/main \
    --instance-sync-path=./clusters/dev \
    --instance-sync-creds=git:${GITHUB_TOKEN}

  # Install and bootstrap from an OCI repository
  flux-operator install \
    --instance-sync-url=oci://ghcr.io/org/manifests \
    --instance-sync-ref=latest \
    --instance-sync-path=./ \
    --instance-sync-creds=flux:${GITHUB_TOKEN}
`,
	Args: cobra.NoArgs,
	RunE: installCmdRun,
}

type installFlags struct {
	components           []string
	componentsExtra      []string
	distributionVersion  string
	distributionRegistry string
	distributionArtifact string
	clusterType          string
	clusterSize          string
	clusterDomain        string
	clusterMultitenant   bool
	clusterNetworkPolicy bool
	syncURL              string
	syncRef              string
	syncPath             string
	syncCreds            string
	autoUpdate           bool
}

const defaultInstallNamespace = "flux-system"

var installArgs installFlags

func init() {
	installCmd.Flags().StringSliceVar(&installArgs.components, "instance-components",
		[]string{"source-controller", "kustomize-controller", "helm-controller", "notification-controller"},
		"list of Flux components to install (can specify multiple components with a comma-separated list)")
	installCmd.Flags().StringSliceVar(&installArgs.componentsExtra, "instance-components-extra", nil,
		"additional Flux components to install on top of the default set (e.g. image-reflector-controller,image-automation-controller,source-watcher)")
	installCmd.Flags().StringVar(&installArgs.distributionVersion, "instance-distribution-version", "2.x",
		"Flux distribution version")
	installCmd.Flags().StringVar(&installArgs.distributionRegistry, "instance-distribution-registry", "ghcr.io/fluxcd",
		"container registry to pull Flux images from")
	installCmd.Flags().StringVar(&installArgs.distributionArtifact, "instance-distribution-artifact", "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest",
		"OCI artifact containing the Flux distribution manifests")
	installCmd.Flags().StringVar(&installArgs.clusterType, "instance-cluster-type", "kubernetes",
		"cluster type (kubernetes, openshift, aws, azure, gcp)")
	installCmd.Flags().StringVar(&installArgs.clusterSize, "instance-cluster-size", "medium",
		"cluster size profile for vertical scaling of Flux controllers (small, medium, large)")
	installCmd.Flags().StringVar(&installArgs.clusterDomain, "instance-cluster-domain", "cluster.local",
		"cluster domain used for generating the FQDN of services")
	installCmd.Flags().BoolVar(&installArgs.clusterMultitenant, "instance-cluster-multitenant", false,
		"enable multitenant lockdown for Flux controllers")
	installCmd.Flags().BoolVar(&installArgs.clusterNetworkPolicy, "instance-cluster-network-policy", true,
		"restrict network access to the current namespace")
	installCmd.Flags().StringVar(&installArgs.syncURL, "instance-sync-url", "",
		"URL of the source for cluster sync (Git repository URL or OCI repository address)")
	installCmd.Flags().StringVar(&installArgs.syncRef, "instance-sync-ref", "",
		"source reference for cluster sync (Git ref name e.g. 'refs/heads/main' or OCI tag e.g. 'latest')")
	installCmd.Flags().StringVar(&installArgs.syncPath, "instance-sync-path", "./",
		"path to the manifests directory in the source")
	installCmd.Flags().StringVar(&installArgs.syncCreds, "instance-sync-creds", "",
		"credentials for the source in the format username:token (creates a Secret named 'flux-system')")
	installCmd.Flags().BoolVar(&installArgs.autoUpdate, "auto-update", true,
		"enable automatic updates of the Flux Operator from the distribution artifact")

	rootCmd.AddCommand(installCmd)
}

func installCmdRun(cmd *cobra.Command, args []string) error {
	// Increase the default timeout to 5 minutes if not set explicitly
	if rootArgs.timeout == time.Minute {
		rootArgs.timeout = 5 * time.Minute
	}

	now := time.Now()

	// Step 1: Download the distribution artifact and extract the manifests

	rootCmd.Println(`◎`, "Downloading distribution artifact...")
	objects, err := fetchOperatorManifests()
	if err != nil {
		return err
	}
	rootCmd.Println(`✔`, "Download completed")

	// Step 2: Install or upgrade the Flux Operator

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	installed, err := isInstalled(ctx)
	if err != nil {
		return err
	}

	if installed {
		rootCmd.Println(`◎`, "Upgrading Flux Operator in flux-system namespace...")
	} else {
		rootCmd.Println(`◎`, "Installing Flux Operator in flux-system namespace...")
	}
	if err := applyOperatorManifests(ctx, objects); err != nil {
		return err
	}
	rootCmd.Println(`✔`, "Flux Operator has been installed successfully")

	// Step 3: Create or update the sync credentials secret if specified

	if installArgs.syncCreds != "" {
		rootCmd.Println(`◎`, "Configuring sync credentials...")
		if err := applySyncSecret(ctx); err != nil {
			return err
		}
	}

	// Step 4: Install or upgrade the Flux instance

	rootCmd.Println(`◎`, "Installing the Flux instance...")
	if err := applyFluxInstance(ctx); err != nil {
		return err
	}
	rootCmd.Println(`✔`, "Flux has been installed successfully")
	if err := printSyncInfo(ctx); err != nil {
		return err
	}

	// Step 5: Configure automatic updates if enabled

	if installArgs.autoUpdate {
		rootCmd.Println(`◎`, "Configuring automatic updates...")
		if err := applyAutoUpdate(ctx); err != nil {
			return err
		}
	}

	if err := printVersionInfo(ctx); err != nil {
		return err
	}
	rootCmd.Println(`✔`, "Installation completed in", time.Since(now).Round(time.Second).String())
	return nil
}

// fetchOperatorManifests downloads the Flux Operator distribution artifact
// and returns the list of Kubernetes objects from the install manifest.
func fetchOperatorManifests() ([]*unstructured.Unstructured, error) {
	tmpArtifactDir, err := builder.MkdirTempAbs("", "flux-artifact")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpArtifactDir)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	if _, err := builder.PullArtifact(
		ctx,
		installArgs.distributionArtifact,
		tmpArtifactDir,
		authn.DefaultKeychain,
	); err != nil {
		return nil, fmt.Errorf("failed to pull distribution artifact: %w", err)
	}

	installManifest := filepath.Join(tmpArtifactDir, "flux-operator", "install.yaml")
	if _, err := os.Stat(installManifest); os.IsNotExist(err) {
		return nil, fmt.Errorf("invalid distribution artifact, missing flux-operator/install.yaml")
	}

	data, err := os.ReadFile(installManifest)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	objects, err := ssautil.ReadObjects(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("unable to parse flux-operator/install.yaml: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("no Kubernetes objects found in flux-operator/install.yaml")
	}

	return objects, nil
}

// isInstalled checks if the Flux Operator is already installed in the cluster by checking
// for the FluxInstance CRD. If the CRD is managed by Helm, it returns an error.
func isInstalled(ctx context.Context) (bool, error) {
	kubeClient, err := newKubeClient()
	if err != nil {
		return false, fmt.Errorf("unable to create kube client: %w", err)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	err = kubeClient.Get(ctx, types.NamespacedName{
		Name: "fluxinstances.fluxcd.controlplane.io",
	}, crd)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("unable to check if Flux Operator is installed: %w", err)
	}

	// Check if the CRD is managed by Helm
	if managedBy, exists := crd.Labels["app.kubernetes.io/managed-by"]; exists && managedBy == "Helm" {
		return true, fmt.Errorf("the Flux Operator installation is managed by Helm, cannot proceed with installation")
	}

	return true, nil
}

// applyOperatorManifests applies the Flux Operator manifests to the cluster and waits for them to be ready.
func applyOperatorManifests(ctx context.Context, objects []*unstructured.Unstructured) error {
	operatorManager, err := newManager()
	if err != nil {
		return fmt.Errorf("unable to create operator manager: %w", err)
	}

	ssautil.SetCommonMetadata(objects, map[string]string{
		"app.kubernetes.io/name": "flux-operator",
	}, nil)

	changeSet, err := operatorManager.ApplyAllStaged(ctx, objects, ssa.DefaultApplyOptions())
	if err != nil {
		return fmt.Errorf("failed to apply the operator manifests: %w", err)
	}

	for _, entry := range changeSet.Entries {
		rootCmd.Println(`✔`, entry.String())
	}

	rootCmd.Println(`◎`, "Waiting for Flux Operator to be ready...")
	return operatorManager.WaitForSetWithContext(ctx, changeSet.ToObjMetadataSet(), ssa.WaitOptions{
		Interval: 5 * time.Second,
		Timeout:  rootArgs.timeout,
	})
}

// applySyncSecret creates and applies the sync credentials secret to the cluster.
func applySyncSecret(ctx context.Context) error {
	if installArgs.syncURL == "" {
		return fmt.Errorf("--instance-sync-creds requires --instance-sync-url to be set")
	}

	// Parse credentials
	parts := strings.SplitN(installArgs.syncCreds, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid credentials format, expected username:token")
	}
	username := parts[0]
	password := parts[1]

	var secret *corev1.Secret
	var err error

	// Determine source type and create appropriate secret
	if strings.HasPrefix(installArgs.syncURL, "oci://") {
		// Extract server from OCI URL (strip oci:// prefix and take host part)
		server := strings.TrimPrefix(installArgs.syncURL, "oci://")
		if idx := strings.Index(server, "/"); idx > 0 {
			server = server[:idx]
		}
		secret, err = secrets.MakeRegistrySecret(
			defaultInstallNamespace,
			defaultInstallNamespace,
			server,
			username,
			password,
		)
	} else {
		// Git source (HTTP/S or SSH)
		secret, err = secrets.MakeBasicAuthSecret(
			defaultInstallNamespace,
			defaultInstallNamespace,
			username,
			password,
		)
	}

	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return fmt.Errorf("failed to convert secret to unstructured: %w", err)
	}

	secretManager, err := newManager()
	if err != nil {
		return fmt.Errorf("unable to create secret manager: %w", err)
	}

	changeSet, err := secretManager.ApplyAllStaged(ctx, []*unstructured.Unstructured{{Object: rawMap}}, ssa.DefaultApplyOptions())
	if err != nil {
		return fmt.Errorf("failed to apply secret: %w", err)
	}

	for _, entry := range changeSet.Entries {
		rootCmd.Println(`✔`, entry.String())
	}

	return nil
}

// applyFluxInstance generates a FluxInstance from the install flags
// and applies it to the cluster, waiting for it to be ready.
func applyFluxInstance(ctx context.Context) error {
	instance := &fluxcdv1.FluxInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.FluxInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluxcdv1.DefaultInstanceName,
			Namespace: defaultInstallNamespace,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  installArgs.distributionVersion,
				Registry: installArgs.distributionRegistry,
				Artifact: installArgs.distributionArtifact,
			},
			Cluster: &fluxcdv1.Cluster{
				Type:          installArgs.clusterType,
				Size:          installArgs.clusterSize,
				Domain:        installArgs.clusterDomain,
				Multitenant:   installArgs.clusterMultitenant,
				NetworkPolicy: installArgs.clusterNetworkPolicy,
			},
		},
	}

	// Add components if specified
	if len(installArgs.components) > 0 {
		// Combine default components with extra components
		allComponents := installArgs.components
		if len(installArgs.componentsExtra) > 0 {
			allComponents = append(allComponents, installArgs.componentsExtra...)
		}

		components := make([]fluxcdv1.Component, len(allComponents))
		for i, c := range allComponents {
			components[i] = fluxcdv1.Component(c)
		}
		instance.Spec.Components = components
	}

	// Add sync configuration if URL is specified
	if installArgs.syncURL != "" {
		sync := &fluxcdv1.Sync{
			URL:  installArgs.syncURL,
			Ref:  installArgs.syncRef,
			Path: installArgs.syncPath,
		}

		// Determine kind based on URL prefix
		if strings.HasPrefix(installArgs.syncURL, "oci://") {
			sync.Kind = "OCIRepository"
		} else {
			sync.Kind = "GitRepository"
		}

		// Set PullSecret if credentials were provided
		if installArgs.syncCreds != "" {
			sync.PullSecret = defaultInstallNamespace
		}

		instance.Spec.Sync = sync
	}

	// Convert to unstructured
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(instance)
	if err != nil {
		return err
	}

	// Apply the FluxInstance
	instanceManager, err := newManager()
	if err != nil {
		return fmt.Errorf("unable to create instance manager: %w", err)
	}
	changeSet, err := instanceManager.ApplyAllStaged(ctx, []*unstructured.Unstructured{{Object: rawMap}}, ssa.DefaultApplyOptions())
	if err != nil {
		return fmt.Errorf("failed to apply the instance: %w", err)
	}
	for _, entry := range changeSet.Entries {
		rootCmd.Println(`✔`, entry.String())
	}

	rootCmd.Println(`◎`, "Waiting for Flux instance to be ready...")
	_, err = waitForResourceReconciliation(ctx,
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind),
		fluxcdv1.DefaultInstanceName,
		defaultInstallNamespace,
		"", rootArgs.timeout)
	return err
}

// printSyncInfo prints information about the sync status of the Flux instance
// such as sync source URL and status of the last sync operation.
func printSyncInfo(ctx context.Context) error {
	reportName := types.NamespacedName{
		Name:      "flux",
		Namespace: defaultInstallNamespace,
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	report := &fluxcdv1.FluxReport{}
	err = kubeClient.Get(ctx, reportName, report)
	if err != nil {
		return fmt.Errorf("unable to get Flux report: %w", err)
	}

	// Print sync status if available
	if report.Spec.SyncStatus != nil {
		sync := report.Spec.SyncStatus
		rootCmd.Println(`✔`, "Syncing from:", sync.Source)
		rootCmd.Println(`✔`, sync.Status)
	}

	return nil
}

// printVersionInfo prints the version information of the Flux Operator.
func printVersionInfo(ctx context.Context) error {
	reportName := types.NamespacedName{
		Name:      "flux",
		Namespace: defaultInstallNamespace,
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	report := &fluxcdv1.FluxReport{}
	err = kubeClient.Get(ctx, reportName, report)
	if err != nil {
		return fmt.Errorf("unable to get Flux report: %w", err)
	}

	if report.Spec.Operator != nil && report.Spec.Operator.Version != "" {
		rootCmd.Println(`✔`, "Flux Operator version:", report.Spec.Operator.Version)
	}
	if report.Spec.Distribution.Version != "" {
		rootCmd.Println(`✔`, "Flux Distribution version:", report.Spec.Distribution.Version)
	}
	return nil
}

// autoUpdateData holds the data for rendering the auto-update template.
type autoUpdateData struct {
	Namespace   string
	ArtifactURL string
	Multitenant bool
}

// applyAutoUpdate configures automatic updates of the Flux Operator from the distribution artifact.
func applyAutoUpdate(ctx context.Context) error {
	// Strip tag from artifact URL (e.g., "oci://registry/image:tag" -> "oci://registry/image")
	artifactURL := installArgs.distributionArtifact
	if idx := strings.LastIndex(artifactURL, ":"); idx > 6 {
		artifactURL = artifactURL[:idx]
	}

	// Build template data
	data := autoUpdateData{
		Namespace:   defaultInstallNamespace,
		ArtifactURL: artifactURL,
		Multitenant: installArgs.clusterMultitenant,
	}

	// Execute template
	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	if err != nil {
		return fmt.Errorf("unable to parse auto-update template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("unable to execute auto-update template: %w", err)
	}
	autoUpdateYAML := buf.String()

	autoUpdateObjects, err := ssautil.ReadObjects(bytes.NewReader([]byte(autoUpdateYAML)))
	if err != nil {
		return fmt.Errorf("unable to parse auto-update manifest: %w", err)
	}

	autoUpdateManager, err := newManager()
	if err != nil {
		return fmt.Errorf("unable to create auto-update manager: %w", err)
	}

	changeSet, err := autoUpdateManager.ApplyAllStaged(ctx, autoUpdateObjects, ssa.DefaultApplyOptions())
	if err != nil {
		return fmt.Errorf("failed to apply auto-update ResourceSet: %w", err)
	}

	for _, entry := range changeSet.Entries {
		rootCmd.Println(`✔`, entry.String())
	}

	rootCmd.Println(`◎`, "Waiting for ResourceSet to be ready...")
	_, err = waitForResourceReconciliation(ctx,
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind),
		"flux-operator",
		defaultInstallNamespace,
		"", rootArgs.timeout)
	if err != nil {
		return err
	}

	return nil
}

const autoUpdateTmpl = `
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: flux-operator
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/name: flux-operator
    app.kubernetes.io/instance: flux-operator
  annotations:
    fluxcd.controlplane.io/reconcileTimeout: "5m"
spec:
  wait: true
  inputs:
    - url: {{.ArtifactURL}}
      interval: "1h"
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      metadata:
        name: << inputs.provider.name >>
        namespace: << inputs.provider.namespace >>
      spec:
        interval: << inputs.interval | quote >>
        url: << inputs.url | quote >>
        ref:
          tag: latest
    - apiVersion: kustomize.toolkit.fluxcd.io/v1
      kind: Kustomization
      metadata:
        name: << inputs.provider.name >>
        namespace: << inputs.provider.namespace >>
      spec:
        interval: 24h
        retryInterval: 5m
        timeout: 5m
        wait: true
        prune: true
        deletionPolicy: Orphan
        serviceAccountName: << inputs.provider.name >>
        sourceRef:
          kind: OCIRepository
          name: << inputs.provider.name >>
        path: ./flux-operator
        patches:
          - patch: |-
              - op: add
                path: "/spec/template/spec/containers/0/env/-"
                value:
                  name: REPORTING_INTERVAL
                  value: "30s"
{{- if .Multitenant }}
              - op: add
                path: "/spec/template/spec/containers/0/env/-"
                value:
                  name: DEFAULT_SERVICE_ACCOUNT
                  value: "flux-operator"
{{- end }}
            target:
              kind: Deployment
`

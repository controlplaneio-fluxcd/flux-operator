// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/install"
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

  # Install and bootstrap from a Git repository using a GitHub App
  flux-operator install \
    --instance-sync-url=https://github.com/my-org/fleet-infra \
    --instance-sync-ref=refs/heads/main \
    --instance-sync-path=./clusters/dev \
    --instance-sync-gha-app-id=123456 \
    --instance-sync-gha-installation-owner=my-org \
    --instance-sync-gha-private-key-file=./private-key.pem

  # Install and bootstrap from an OCI repository
  flux-operator install \
    --instance-sync-url=oci://ghcr.io/org/manifests \
    --instance-sync-ref=latest \
    --instance-sync-path=./ \
    --instance-sync-creds=flux:${GITHUB_TOKEN}

  # Install using a Flux instance YAML file
  flux-operator install -f flux-instance.yaml

  # Install using a Flux instance from a GitHub URL
  flux-operator install \
    --instance-sync-creds=git:${GITHUB_TOKEN} \
    -f https://github.com/org/repo/blob/main/flux-instance.yaml

  # Install using a Flux instance from a GitLab URL
  flux-operator install \
    --instance-sync-creds=git:${GITLAB_TOKEN} \
    -f https://gitlab.com/org/proj/-/blob/main/flux-instance.yaml?ref_type=heads

  # Install using a Flux instance from an OCI artifact
  flux-operator install \
	-f oci://ghcr.io/org/manifests:latest#clusters/dev/flux-instance.yaml

  # Install using a Flux instance from a GitHub Gist
  flux-operator install -f https://gist.github.com/username/gist-id#file-flux-instance-yaml
`,
	Args: cobra.NoArgs,
	RunE: installCmdRun,
}

type installFlags struct {
	instanceFile             string
	components               []string
	componentsExtra          []string
	distributionVersion      string
	distributionRegistry     string
	distributionArtifact     string
	clusterType              string
	clusterSize              string
	clusterDomain            string
	clusterMultitenant       bool
	clusterNetworkPolicy     bool
	syncURL                  string
	syncRef                  string
	syncPath                 string
	syncCreds                string
	syncGHAAppID             string
	syncGHAInstallationID    string
	syncGHAInstallationOwner string
	syncGHAPrivateKeyFile    string
	syncGHABaseURL           string
	autoUpdate               bool
}

var installArgs installFlags

func init() {
	installCmd.Flags().StringVarP(&installArgs.instanceFile, "instance-file", "f", "",
		"path to Flux instance YAML file (local file or HTTPS URL)")
	installCmd.Flags().StringSliceVar(&installArgs.components, "instance-components",
		[]string{"source-controller", "kustomize-controller", "helm-controller", "notification-controller"},
		"list of Flux components to install (can specify multiple components with a comma-separated list)")
	installCmd.Flags().StringSliceVar(&installArgs.componentsExtra, "instance-components-extra", nil,
		"additional Flux components to install on top of the default set (e.g. image-reflector-controller,image-automation-controller,source-watcher)")
	installCmd.Flags().StringVar(&installArgs.distributionVersion, "instance-distribution-version", "2.x",
		"Flux distribution version")
	installCmd.Flags().StringVar(&installArgs.distributionRegistry, "instance-distribution-registry", "ghcr.io/fluxcd",
		"container registry to pull Flux images from")
	installCmd.Flags().StringVar(&installArgs.distributionArtifact, "instance-distribution-artifact", install.DefaultArtifactURL,
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
	installCmd.Flags().StringVar(&installArgs.syncGHAAppID, "instance-sync-gha-app-id", "",
		"GitHub App ID for the sync source credentials")
	installCmd.Flags().StringVar(&installArgs.syncGHAInstallationID, "instance-sync-gha-installation-id", "",
		"GitHub App Installation ID (optional)")
	installCmd.Flags().StringVar(&installArgs.syncGHAInstallationOwner, "instance-sync-gha-installation-owner", "",
		"GitHub App Installation Owner (organization or user) (optional)")
	installCmd.Flags().StringVar(&installArgs.syncGHAPrivateKeyFile, "instance-sync-gha-private-key-file", "",
		"path to GitHub App private key file")
	installCmd.Flags().StringVar(&installArgs.syncGHABaseURL, "instance-sync-gha-base-url", "",
		"GitHub base URL for GitHub Enterprise Server (optional)")
	installCmd.Flags().BoolVar(&installArgs.autoUpdate, "auto-update", true,
		"enable automatic updates of the Flux Operator from the distribution artifact")

	rootCmd.AddCommand(installCmd)
}

func installCmdRun(cmd *cobra.Command, args []string) error {
	// Set a minimum timeout of 5 minutes
	if rootArgs.timeout < 2*time.Minute {
		rootArgs.timeout = 5 * time.Minute
	}

	now := time.Now()

	timeout := rootArgs.timeout - time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// Step 1: Generate Flux instance from file, URL or flags

	rootCmd.Println(`◎`, "Downloading artifacts...")
	instance, artifactURL, err := makeFluxInstance(ctx)
	if err != nil {
		return err
	}

	// Step 2: Download the distribution artifact and extract the manifests

	objects, err := fetchOperatorManifests(artifactURL)
	if err != nil {
		return err
	}
	rootCmd.Println(`✔`, "Download completed in", time.Since(now).Round(time.Second).String())

	// Step 3: Install or upgrade the Flux Operator

	cfg, err := kubeconfigArgs.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	installerOpts := []install.Option{
		install.WithArtifactURL(artifactURL),
		install.WithCredentials(installArgs.syncCreds),
	}

	if installArgs.hasSyncGHA() {
		privateKey, err := os.ReadFile(installArgs.syncGHAPrivateKeyFile)
		if err != nil {
			return fmt.Errorf("unable to read GitHub App private key file: %w", err)
		}
		installerOpts = append(installerOpts, install.WithGitHubAppCredentials(&install.GitHubAppCredentials{
			AppID:             installArgs.syncGHAAppID,
			InstallationID:    installArgs.syncGHAInstallationID,
			InstallationOwner: installArgs.syncGHAInstallationOwner,
			PrivateKey:        string(privateKey),
			BaseURL:           installArgs.syncGHABaseURL,
		}))
	}

	installer, err := install.NewInstaller(ctx, cfg, installerOpts...)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	isInstalled, err := installer.IsInstalled(ctx)
	if err != nil {
		return err
	}

	if isInstalled {
		rootCmd.Println(`◎`, "Upgrading Flux Operator in flux-system namespace...")
	} else {
		rootCmd.Println(`◎`, "Installing Flux Operator in flux-system namespace...")
	}
	multitenant := instance.Spec.Cluster != nil && instance.Spec.Cluster.Multitenant
	cs, err := installer.ApplyOperator(ctx, objects, multitenant)
	if err != nil {
		return err
	}
	printChangeSet(cs)

	rootCmd.Println(`◎`, "Waiting for Flux Operator to be ready...")
	if err := installer.WaitFor(ctx, cs, timeout); err != nil {
		return err
	}

	rootCmd.Println(`✔`, "Flux Operator has been installed successfully")

	// Step 4: Create or update the sync credentials secret if specified

	if (installArgs.syncCreds != "" || installArgs.hasSyncGHA()) && instance.Spec.Sync != nil {
		rootCmd.Println(`◎`, "Configuring sync credentials...")
		secretName := install.DefaultNamespace

		// Override secret name if specified in the instance
		if instance.Spec.Sync.PullSecret != "" {
			secretName = instance.Spec.Sync.PullSecret
		}

		var csEntry *ssa.ChangeSetEntry
		if installArgs.hasSyncGHA() {
			csEntry, err = installer.ApplyGitHubAppCredentials(ctx, secretName)
		} else {
			csEntry, err = installer.ApplyCredentials(ctx, secretName, instance.Spec.Sync.URL)
		}
		if err != nil {
			return err
		}
		cs := ssa.NewChangeSet()
		cs.Add(*csEntry)
		printChangeSet(cs)
	}

	// Step 5: Install or upgrade the Flux instance

	rootCmd.Println(`◎`, "Installing the Flux instance...")
	cs, err = installer.ApplyInstance(ctx, instance)
	if err != nil {
		return err
	}
	printChangeSet(cs)

	rootCmd.Println(`◎`, "Waiting for Flux instance to be ready...")
	if err := installer.WaitFor(ctx, cs, timeout); err != nil {
		// Print events for debugging
		if events, err := installer.GetEvents(ctx, fluxcdv1.FluxInstanceKind, instance.GetName()); err == nil && len(events) > 0 {
			for _, e := range events {
				rootCmd.Printf("%s -> %s\n",
					e.InvolvedObject,
					strings.TrimSpace(e.Message),
				)
			}
		}
		return fmt.Errorf("timeout waiting for %s/%s/%s to be ready",
			fluxcdv1.FluxInstanceKind, instance.GetNamespace(), instance.GetName())
	}

	rootCmd.Println(`✔`, "Flux has been installed successfully")
	if err := printSyncInfo(ctx); err != nil {
		return err
	}

	// Step 6: Configure automatic updates if enabled

	if installArgs.autoUpdate {
		rootCmd.Println(`◎`, "Configuring automatic updates...")
		cs, err := installer.ApplyAutoUpdate(ctx, multitenant)
		if err != nil {
			return err
		}
		printChangeSet(cs)

		rootCmd.Println(`◎`, "Waiting for auto-update to be ready...")
		if err := installer.WaitFor(ctx, cs, timeout); err != nil {
			// Print events for debugging
			if events, err := installer.GetEvents(ctx, fluxcdv1.ResourceSetKind, "flux-operator"); err == nil && len(events) > 0 {
				for _, e := range events {
					rootCmd.Printf("%s -> %s\n",
						e.InvolvedObject,
						strings.TrimSpace(e.Message),
					)
				}
			}
			return err
		}
	}

	if err := printVersionInfo(ctx); err != nil {
		return err
	}
	rootCmd.Println(`✔`, "Installation completed in", time.Since(now).Round(time.Second).String())
	return nil
}

// makeFluxInstance creates a FluxInstance object from the provided file or command-line flags.
// If a file is provided, it takes precedence over the flags.
// It also returns the artifact URL to be used for downloading the Flux Operator manifests.
func makeFluxInstance(ctx context.Context) (instance *fluxcdv1.FluxInstance, artifactURL string, err error) {
	instance = &fluxcdv1.FluxInstance{}
	artifactURL = installArgs.distributionArtifact
	if filePath := installArgs.instanceFile; filePath != "" {
		var data []byte

		// Check if the file path is a URL
		if strings.HasPrefix(filePath, "https://") ||
			strings.HasPrefix(filePath, "http://") ||
			strings.HasPrefix(filePath, "oci://") {
			// Fetch from URL
			data, err = install.DownloadManifestFromURL(ctx, filePath, authn.DefaultKeychain)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read response body: %w", err)
			}
		} else {
			// Read from local file
			data, err = os.ReadFile(filePath)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read file: %w", err)
			}
		}

		// Unmarshal the FluxInstance
		if err := yaml.Unmarshal(data, instance); err != nil {
			return nil, "", fmt.Errorf("failed to unmarshal FluxInstance: %w", err)
		}

		// Set namespace to flux-system
		instance.Namespace = install.DefaultNamespace

		// Use artifact URL from file if present
		if instance.Spec.Distribution.Artifact != "" {
			artifactURL = instance.Spec.Distribution.Artifact
		}
	} else {
		// No file provided, build from flags
		instance = &fluxcdv1.FluxInstance{
			TypeMeta: metav1.TypeMeta{
				APIVersion: fluxcdv1.GroupVersion.String(),
				Kind:       fluxcdv1.FluxInstanceKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fluxcdv1.DefaultInstanceName,
				Namespace: install.DefaultNamespace,
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
			if installArgs.syncCreds != "" || installArgs.hasSyncGHA() {
				sync.PullSecret = install.DefaultNamespace
			}

			instance.Spec.Sync = sync
		}
	}

	return instance, artifactURL, nil
}

// fetchOperatorManifests downloads the Flux Operator distribution artifact
// and returns the list of Kubernetes objects from the install manifest.
func fetchOperatorManifests(artifactURL string) ([]*unstructured.Unstructured, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	data, err := install.DownloadFileFromArtifact(
		ctx,
		artifactURL,
		"flux-operator/install.yaml",
		authn.DefaultKeychain,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pull distribution artifact: %w", err)
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

// printChangeSet prints the details of the applied changes from the ChangeSet.
func printChangeSet(cs *ssa.ChangeSet) {
	for _, entry := range cs.Entries {
		rootCmd.Println(`✔`, entry.String())
	}
}

// printSyncInfo prints information about the sync status of the Flux instance
// such as sync source URL and status of the last sync operation.
func printSyncInfo(ctx context.Context) error {
	reportName := types.NamespacedName{
		Name:      fluxcdv1.DefaultInstanceName,
		Namespace: install.DefaultNamespace,
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
		Name:      fluxcdv1.DefaultInstanceName,
		Namespace: install.DefaultNamespace,
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

// hasSyncGHA returns true if GitHub App credentials flags are set.
func (f *installFlags) hasSyncGHA() bool {
	return f.syncGHAAppID != ""
}

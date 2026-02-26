// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

var buildInstanceCmd = &cobra.Command{
	Use:   "instance",
	Short: "Build a FluxInstance definition to Kubernetes manifests",
	Long: `The build instance command performs the following steps:
1. Reads the FluxInstance YAML manifest from the specified file.
2. Validates the instance definition and sets default values.
3. Pulls the distribution OCI artifact from the registry using the Docker config file for authentication.
   If not specified, the artifact is pulled from 'oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests'.
   The artifact URL can be overridden with the --distribution-artifact flag.
4. Builds the Flux Kubernetes manifests according to the instance specifications and kustomize patches.
5. Prints the multi-doc YAML containing the Flux Kubernetes manifests to stdout.
`,
	Example: `  # Build the given FluxInstance and print the generated manifests
  flux-operator build instance -f flux.yaml

  # Build using a custom distribution artifact
  flux-operator build instance -f flux.yaml \
    --distribution-artifact oci://ghcr.io/my-org/flux-operator-manifests:latest

  # Pipe the FluxInstance definition to the build command
  cat flux.yaml | flux-operator build instance -f -

  # Build a FluxInstance and print a diff of the generated manifests
  flux-operator build instance -f flux.yaml | \
    kubectl diff --server-side --field-manager=flux-operator -f -
`,
	Args: cobra.NoArgs,
	RunE: buildInstanceCmdRun,
}

type buildInstanceFlags struct {
	filename             string
	distributionArtifact string
}

var buildInstanceArgs buildInstanceFlags

func init() {
	buildInstanceCmd.Flags().StringVarP(&buildInstanceArgs.filename, "filename", "f", "", "Path to the FluxInstance YAML manifest.")
	buildInstanceCmd.Flags().StringVar(&buildInstanceArgs.distributionArtifact, "distribution-artifact", "", "OCI artifact URL of the Flux distribution, takes precedence over the FluxInstance spec.")

	buildCmd.AddCommand(buildInstanceCmd)
}

func buildInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if buildInstanceArgs.filename == "" {
		return errors.New("--filename is required")
	}

	path := buildInstanceArgs.filename
	var err error
	if buildInstanceArgs.filename == "-" {
		path, err = saveReaderToFile(os.Stdin)
		if err != nil {
			return err
		}

		defer os.Remove(path)
	}

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("invalid filename '%s', must point to an existing file", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	var instance fluxcdv1.FluxInstance
	err = yaml.Unmarshal(data, &instance)
	if err != nil {
		return fmt.Errorf("error parsing FluxInstance: %w", err)
	}

	setInstanceDefaults(&instance)
	if buildInstanceArgs.distributionArtifact != "" {
		instance.Spec.Distribution.Artifact = buildInstanceArgs.distributionArtifact
	}
	if err := validateInstance(&instance); err != nil {
		return err
	}

	tmpArtifactDir, err := builder.MkdirTempAbs("", "flux-artifact")
	if err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpArtifactDir)

	ctxPull, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := builder.PullArtifact(
		ctxPull,
		instance.Spec.Distribution.Artifact,
		tmpArtifactDir,
		authn.DefaultKeychain,
	); err != nil {
		return fmt.Errorf("failed to pull distribution artifact: %w", err)
	}
	fluxManifestsDir := filepath.Join(tmpArtifactDir, "flux")

	ver, err := builder.MatchVersion(fluxManifestsDir, instance.Spec.Distribution.Version)
	if err != nil {
		return err
	}

	options := builder.MakeDefaultOptions()
	options.Version = ver
	options.Registry = instance.GetDistribution().Registry
	options.ImagePullSecret = instance.GetDistribution().ImagePullSecret
	options.Namespace = instance.GetNamespace()
	options.Components = instance.GetComponents()
	options.NetworkPolicy = instance.GetCluster().NetworkPolicy
	options.ClusterDomain = instance.GetCluster().Domain

	options.Patches += builder.GetProfileClusterType(instance.GetCluster().Type)
	options.Patches += builder.GetProfileClusterSize(instance.GetCluster().Size)

	if instance.GetCluster().Multitenant {
		options.Patches += builder.GetProfileMultitenant(instance.GetCluster().TenantDefaultServiceAccount)
	}

	if err := options.ValidateAndPatchComponents(); err != nil {
		return err
	}

	if err := options.ValidateAndApplyWorkloadIdentityConfig(instance.GetCluster()); err != nil {
		return fmt.Errorf("failed to validate workload identity configuration: %w", err)
	}

	if instance.Spec.Sharding != nil {
		options.ShardingKey = instance.Spec.Sharding.Key
		options.Shards = instance.Spec.Sharding.Shards
	}

	if instance.Spec.Storage != nil {
		options.ArtifactStorage = &builder.ArtifactStorage{
			Class: instance.Spec.Storage.Class,
			Size:  instance.Spec.Storage.Size,
		}
	}

	if instance.Spec.Sync != nil {
		syncName := instance.GetNamespace()
		if instance.Spec.Sync.Name != "" {
			syncName = instance.Spec.Sync.Name
		}
		options.Sync = &builder.Sync{
			Name:       syncName,
			Kind:       instance.Spec.Sync.Kind,
			Interval:   instance.Spec.Sync.Interval.Duration.String(),
			Ref:        instance.Spec.Sync.Ref,
			PullSecret: instance.Spec.Sync.PullSecret,
			URL:        instance.Spec.Sync.URL,
			Path:       instance.Spec.Sync.Path,
			Provider:   instance.Spec.Sync.Provider,
		}
	}

	if instance.Spec.Kustomize != nil && len(instance.Spec.Kustomize.Patches) > 0 {
		patchesData, err := yaml.Marshal(instance.Spec.Kustomize.Patches)
		if err != nil {
			return fmt.Errorf("failed to parse kustomize patches: %w", err)
		}
		options.Patches += string(patchesData)
	}

	srcDir := filepath.Join(fluxManifestsDir, ver)
	images, err := builder.ExtractComponentImages(srcDir, options)
	if err != nil {
		return fmt.Errorf("failed to extract container images from manifests: %w", err)
	}
	options.ComponentImages = images

	tmpWorkDir, err := builder.MkdirTempAbs("", "flux-instance")
	if err != nil {
		return fmt.Errorf("failed to create tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpWorkDir)

	res, err := builder.Build(srcDir, tmpWorkDir, options)
	if err != nil {
		return err
	}
	objects := res.Objects

	if len(objects) == 0 {
		return fmt.Errorf("no objects were generated")
	}

	if instance.Spec.CommonMetadata != nil {
		ssautil.SetCommonMetadata(objects, instance.Spec.CommonMetadata.Labels, instance.Spec.CommonMetadata.Annotations)
	}

	ssautil.SetCommonMetadata(objects, map[string]string{
		fmt.Sprintf("%s/name", fluxcdv1.GroupVersion.Group):      instance.GetName(),
		fmt.Sprintf("%s/namespace", fluxcdv1.GroupVersion.Group): instance.GetNamespace(),
	}, nil)

	for _, obj := range objects {
		var strBuilder strings.Builder
		strBuilder.WriteString("---\n")
		yml, ymlErr := yaml.Marshal(obj)
		if ymlErr != nil {
			return fmt.Errorf("error marshalling object: %w", ymlErr)
		}
		strBuilder.Write(yml)
		rootCmd.Print(strBuilder.String())
	}

	return nil
}

// setInstanceDefaults emulates the Kubernetes admission by setting default values.
func setInstanceDefaults(instance *fluxcdv1.FluxInstance) {
	if instance.Namespace == "" {
		instance.Namespace = "flux-system"
	}

	if instance.Spec.Distribution.Artifact == "" {
		instance.Spec.Distribution.Artifact = "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest"
	}

	if instance.Spec.Cluster != nil {
		if instance.Spec.Cluster.Type == "" {
			instance.Spec.Cluster.Type = "kubernetes"
		}
		if instance.Spec.Cluster.Domain == "" {
			instance.Spec.Cluster.Domain = "cluster.local"
		}
	}

	if instance.Spec.Sharding != nil {
		if instance.Spec.Sharding.Key == "" {
			instance.Spec.Sharding.Key = "sharding.fluxcd.io/key"
		}
	}

	if instance.Spec.Sync != nil {
		if instance.Spec.Sync.Interval == nil {
			instance.Spec.Sync.Interval = &metav1.Duration{Duration: time.Minute}
		}
	}
}

// validateInstance emulates the Kubernetes admission by verifying required fields.
func validateInstance(instance *fluxcdv1.FluxInstance) error {
	if instance.Spec.Distribution.Version == "" {
		return fmt.Errorf(".spec.distribution.version is required")
	}

	if instance.Spec.Distribution.Registry == "" {
		return fmt.Errorf(".spec.distribution.registry is required")
	}

	if instance.Spec.Sharding != nil {
		if len(instance.Spec.Sharding.Shards) == 0 {
			return fmt.Errorf(".spec.sharding.shards is required")
		}
	}

	if instance.Spec.Storage != nil {
		if instance.Spec.Storage.Class == "" {
			return fmt.Errorf(".spec.storage.class is required")
		}
		if instance.Spec.Storage.Size == "" {
			return fmt.Errorf(".spec.storage.size is required")
		}
	}

	if instance.Spec.Sync != nil {
		if instance.Spec.Sync.Kind == "" {
			return fmt.Errorf(".spec.sync.kind is required")
		}
		if instance.Spec.Sync.URL == "" {
			return fmt.Errorf(".spec.sync.url is required")
		}
		if instance.Spec.Sync.Ref == "" {
			return fmt.Errorf(".spec.sync.ref is required")
		}
		if instance.Spec.Sync.Path == "" {
			return fmt.Errorf(".spec.sync.path is required")
		}
	}

	return nil
}

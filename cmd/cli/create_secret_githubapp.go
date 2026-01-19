// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
)

var createSecretGitHubAppCmd = &cobra.Command{
	Use:   "githubapp [name]",
	Short: "Create a Kubernetes Secret containing GitHub App credentials",
	Example: `  # Create or update a secret with GitHub App credentials for Flux operations
  flux-operator create secret githubapp github-app-auth \
  --namespace=flux-system \
  --app-id=123456 \
  --app-installation-id=78901234 \
  --app-private-key-file=./private-key.pem

  # Create a secret with GitHub Enterprise Server base URL
  flux-operator create secret githubapp github-enterprise-auth \
  --namespace=flux-system \
  --app-id=123456 \
  --app-installation-id=78901234 \
  --app-private-key-file=./private-key.pem \
  --app-base-url=https://github.example.com/api/v3

  # Generate a secret and export it to a YAML file
  flux-operator -n apps create secret githubapp github-app \
  --app-id=123456 \
  --app-installation-id=78901234 \
  --app-private-key-file=./private-key.pem \
  --export > github-app.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretGitHubAppCmdRun,
}

type createSecretGitHubAppFlags struct {
	appID             string
	installationOwner string
	installationID    string
	privateKeyFile    string
	baseURL           string

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretGitHubAppArgs createSecretGitHubAppFlags

func init() {
	createSecretGitHubAppCmd.Flags().StringVar(&createSecretGitHubAppArgs.appID, "app-id", "",
		"GitHub App ID (required)")
	createSecretGitHubAppCmd.Flags().StringVar(&createSecretGitHubAppArgs.installationOwner, "app-installation-owner", "",
		"GitHub App Installation Owner (organization or user) (optional)")
	createSecretGitHubAppCmd.Flags().StringVar(&createSecretGitHubAppArgs.installationID, "app-installation-id", "",
		"GitHub App Installation ID (optional)")
	createSecretGitHubAppCmd.Flags().StringVar(&createSecretGitHubAppArgs.privateKeyFile, "app-private-key-file", "",
		"path to GitHub App private key file (required)")
	createSecretGitHubAppCmd.Flags().StringVar(&createSecretGitHubAppArgs.baseURL, "app-base-url", "",
		"GitHub base URL for GitHub Enterprise Server (optional)")
	createSecretGitHubAppCmd.Flags().StringSliceVar(&createSecretGitHubAppArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretGitHubAppCmd.Flags().StringSliceVar(&createSecretGitHubAppArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretGitHubAppCmd.Flags().BoolVar(&createSecretGitHubAppArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretGitHubAppCmd.Flags().BoolVar(&createSecretGitHubAppArgs.export, "export", false,
		"export resource in YAML format to stdout")

	createSecretCmd.AddCommand(createSecretGitHubAppCmd)
}

func createSecretGitHubAppCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]

	// Read private key file
	privateKey, err := os.ReadFile(createSecretGitHubAppArgs.privateKeyFile)
	if err != nil {
		return fmt.Errorf("unable to read private key file: %w", err)
	}

	// Build the secret
	var opts []secrets.GitHubAppOption
	if owner := createSecretGitHubAppArgs.installationOwner; owner != "" {
		opts = append(opts, secrets.WithGitHubAppInstallationOwner(owner))
	}
	if ID := createSecretGitHubAppArgs.installationID; ID != "" {
		opts = append(opts, secrets.WithGitHubAppInstallationID(ID))
	}
	if u := createSecretGitHubAppArgs.baseURL; u != "" {
		opts = append(opts, secrets.WithGitHubAppBaseURL(u))
	}
	secret, err := secrets.MakeGitHubAppSecret(
		name,
		*kubeconfigArgs.Namespace,
		createSecretGitHubAppArgs.appID,
		string(privateKey),
		opts...,
	)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretGitHubAppArgs.annotations,
		createSecretGitHubAppArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretGitHubAppArgs.export {
		return printSecret(secret)
	}

	// Apply the secret to the cluster
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	err = secrets.Apply(
		ctx,
		kubeClient,
		secret,
		secrets.WithForce(),
		secrets.WithImmutable(createSecretGitHubAppArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

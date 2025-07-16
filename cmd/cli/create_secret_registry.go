// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
)

var createSecretRegistryCmd = &cobra.Command{
	Use:   "registry [name]",
	Short: "Create a Kubernetes Secret containing OCI registry credentials",
	Example: `  # Create or update a secret with GHCR credentials for Flux operations
  echo $GITHUB_TOKEN | flux-operator create secret registry ghcr-auth \
  --namespace=flux-system \
  --server=ghcr.io \
  --username=flux \
  --password-stdin

  # Generate a registry secret and export it to a YAML file
  flux-operator -n apps create secret registry registry-auth \
  --server=registry.example.com \
  --username=admin \
  --password=secret \
  --export > registry-auth.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretRegistryCmdRun,
}

type createSecretRegistryFlags struct {
	username      string
	password      string
	passwordStdin bool
	server        string

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretRegistryArgs createSecretRegistryFlags

func init() {
	createSecretRegistryCmd.Flags().StringVar(&createSecretRegistryArgs.username, "username", "",
		"set the username for registry authentication (required)")
	createSecretRegistryCmd.Flags().StringVar(&createSecretRegistryArgs.password, "password", "",
		"set the password for registry authentication (required if --password-stdin is not used)")
	createSecretRegistryCmd.Flags().BoolVar(&createSecretRegistryArgs.passwordStdin, "password-stdin", false,
		"read the password from stdin")
	createSecretRegistryCmd.Flags().StringVar(&createSecretRegistryArgs.server, "server", "",
		"set the registry server (required)")
	createSecretRegistryCmd.Flags().StringSliceVar(&createSecretRegistryArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretRegistryCmd.Flags().StringSliceVar(&createSecretRegistryArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretRegistryCmd.Flags().BoolVar(&createSecretRegistryArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretRegistryCmd.Flags().BoolVar(&createSecretRegistryArgs.export, "export", false,
		"export resource in YAML format to stdout")
	createSecretCmd.AddCommand(createSecretRegistryCmd)
}

func createSecretRegistryCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]
	password := createSecretRegistryArgs.password

	// Read the password from stdin if the flag is set
	if createSecretRegistryArgs.passwordStdin {
		var input string
		_, err := fmt.Scan(&input)
		if err != nil {
			return fmt.Errorf("unable to read password from stdin: %w", err)
		}
		password = input
	}

	// Build the secret
	secret, err := secrets.MakeRegistrySecret(
		name,
		*kubeconfigArgs.Namespace,
		createSecretRegistryArgs.server,
		createSecretRegistryArgs.username,
		password,
	)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretRegistryArgs.annotations,
		createSecretRegistryArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretRegistryArgs.export {
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
		secrets.WithImmutable(createSecretRegistryArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
)

var createSecretBasicAuthCmd = &cobra.Command{
	Use:   "basic-auth [name]",
	Short: "Create a Kubernetes Secret containing basic auth credentials",
	Example: `  # Create or update a secret with a GitHub PAT for Flux operations
  echo $GITHUB_TOKEN | flux-operator create secret basic-auth github-auth \
  --namespace=flux-system \
  --username=flux \
  --password-stdin

  # Generate a basic auth secret and export it to a YAML file
  flux-operator -n apps create secret basic-auth podinfo-auth \
  --username=admin \
  --password=secret \
  --export > podinfo-auth.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretBasicAuthCmdRun,
}

type createSecretBasicAuthFlags struct {
	username      string
	password      string
	passwordStdin bool

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretBasicAuthArgs createSecretBasicAuthFlags

func init() {
	createSecretBasicAuthCmd.Flags().StringVar(&createSecretBasicAuthArgs.username, "username", "",
		"set the username for basic authentication (required)")
	createSecretBasicAuthCmd.Flags().StringVar(&createSecretBasicAuthArgs.password, "password", "",
		"set the password for basic authentication (required if --password-stdin is not used)")
	createSecretBasicAuthCmd.Flags().BoolVar(&createSecretBasicAuthArgs.passwordStdin, "password-stdin", false,
		"read the password from stdin")
	createSecretBasicAuthCmd.Flags().StringSliceVar(&createSecretBasicAuthArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretBasicAuthCmd.Flags().StringSliceVar(&createSecretBasicAuthArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretBasicAuthCmd.Flags().BoolVar(&createSecretBasicAuthArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretBasicAuthCmd.Flags().BoolVar(&createSecretBasicAuthArgs.export, "export", false,
		"export resource in YAML format to stdout")
	createSecretCmd.AddCommand(createSecretBasicAuthCmd)
}

func createSecretBasicAuthCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]
	password := createSecretBasicAuthArgs.password

	// Read the password from stdin if the flag is set
	if createSecretBasicAuthArgs.passwordStdin {
		var input string
		_, err := fmt.Scan(&input)
		if err != nil {
			return fmt.Errorf("unable to read password from stdin: %w", err)
		}
		password = input
	}

	// Build the secret
	secret, err := secrets.MakeBasicAuthSecret(
		name,
		*kubeconfigArgs.Namespace,
		createSecretBasicAuthArgs.username,
		password,
	)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretBasicAuthArgs.annotations,
		createSecretBasicAuthArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretBasicAuthArgs.export {
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
		secrets.WithImmutable(createSecretBasicAuthArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

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

var createSecretSSHCmd = &cobra.Command{
	Use:   "ssh [name]",
	Short: "Create a Kubernetes Secret containing SSH credentials",
	Example: `  # Create or update a secret with SSH key for Flux operations
  flux-operator create secret ssh git-ssh-auth \
  --namespace=flux-system \
  --private-key-file=./id_rsa \
  --public-key-file=./id_rsa.pub \
  --knownhosts-file=./known_hosts

  # Create a secret with password-protected SSH key
  echo "mysecretpassword" | flux-operator create secret ssh git-ssh-auth \
  --namespace=flux-system \
  --private-key-file=./id_rsa \
  --knownhosts-file=./known_hosts \
  --password-stdin

  # Generate a secret and export it to a YAML file
  flux-operator -n apps create secret ssh git-ssh \
  --private-key-file=./id_rsa \
  --knownhosts-file=./known_hosts \
  --export > ssh-secret.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretSSHCmdRun,
}

type createSecretSSHFlags struct {
	privateKeyFile string
	publicKeyFile  string
	knownHostsFile string
	password       string
	passwordStdin  bool

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretSSHArgs createSecretSSHFlags

func init() {
	createSecretSSHCmd.Flags().StringVar(&createSecretSSHArgs.privateKeyFile, "private-key-file", "",
		"path to SSH private key file (required)")
	createSecretSSHCmd.Flags().StringVar(&createSecretSSHArgs.publicKeyFile, "public-key-file", "",
		"path to SSH public key file (optional)")
	createSecretSSHCmd.Flags().StringVar(&createSecretSSHArgs.knownHostsFile, "knownhosts-file", "",
		"path to SSH known_hosts file (required)")
	createSecretSSHCmd.Flags().StringVar(&createSecretSSHArgs.password, "password", "",
		"password for encrypted SSH private key")
	createSecretSSHCmd.Flags().BoolVar(&createSecretSSHArgs.passwordStdin, "password-stdin", false,
		"read the password from stdin")
	createSecretSSHCmd.Flags().StringSliceVar(&createSecretSSHArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretSSHCmd.Flags().StringSliceVar(&createSecretSSHArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretSSHCmd.Flags().BoolVar(&createSecretSSHArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretSSHCmd.Flags().BoolVar(&createSecretSSHArgs.export, "export", false,
		"export resource in YAML format to stdout")

	createSecretCmd.AddCommand(createSecretSSHCmd)
}

func createSecretSSHCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]
	password := createSecretSSHArgs.password

	// Read the password from stdin if the flag is set
	if createSecretSSHArgs.passwordStdin {
		var input string
		_, err := fmt.Scan(&input)
		if err != nil {
			return fmt.Errorf("unable to read password from stdin: %w", err)
		}
		password = input
	}

	// Read private key file
	privateKey, err := os.ReadFile(createSecretSSHArgs.privateKeyFile)
	if err != nil {
		return fmt.Errorf("unable to read private key file: %w", err)
	}

	// Read public key file if provided
	var publicKey []byte
	if createSecretSSHArgs.publicKeyFile != "" {
		publicKey, err = os.ReadFile(createSecretSSHArgs.publicKeyFile)
		if err != nil {
			return fmt.Errorf("unable to read public key file: %w", err)
		}
	}

	// Read known_hosts file
	knownHosts, err := os.ReadFile(createSecretSSHArgs.knownHostsFile)
	if err != nil {
		return fmt.Errorf("unable to read known_hosts file: %w", err)
	}

	// Build the secret
	secret, err := secrets.MakeSSHSecret(
		name,
		*kubeconfigArgs.Namespace,
		string(privateKey),
		string(publicKey),
		string(knownHosts),
		password,
	)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretSSHArgs.annotations,
		createSecretSSHArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretSSHArgs.export {
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
		secrets.WithImmutable(createSecretSSHArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
)

var createSecretSOPSCmd = &cobra.Command{
	Use:   "sops [name]",
	Short: "Create a Kubernetes Secret containing SOPS decryption keys",
	Example: `  # Create or update a secret with Age private key for Flux SOPS decryption
  flux-operator create secret sops sops-age \
  --namespace=flux-system \
  --age-key-file=./age-key.txt

  # Create a secret with GPG private key for Flux SOPS decryption
  flux-operator create secret sops sops-gpg \
  --namespace=flux-system \
  --gpg-key-file=./private.asc

  # Create a secret with both Age and GPG keys
  flux-operator create secret sops sops-keys \
  --namespace=flux-system \
  --age-key-file=./age-key1.txt \
  --age-key-file=./age-key2.txt \
  --gpg-key-file=./private.asc

  # Create a secret with Age key from stdin
  cat ./age-key.txt | flux-operator create secret sops sops-age \
  --namespace=flux-system \
  --age-key-stdin

  # Create a secret with GPG key from stdin
  cat ./private.asc | flux-operator create secret sops sops-gpg \
  --namespace=flux-system \
  --gpg-key-stdin

  # Generate a secret and export it to a YAML file
  flux-operator -n apps create secret sops sops-keys \
  --age-key-file=./age-key.txt \
  --export > sops-secret.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretSOPSCmdRun,
}

type createSecretSOPSFlags struct {
	ageKeyFiles []string
	gpgKeyFiles []string
	ageKeyStdin bool
	gpgKeyStdin bool

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretSOPSArgs createSecretSOPSFlags

func init() {
	createSecretSOPSCmd.Flags().StringSliceVar(&createSecretSOPSArgs.ageKeyFiles, "age-key-file", nil,
		"path to Age private key file (can be used multiple times)")
	createSecretSOPSCmd.Flags().StringSliceVar(&createSecretSOPSArgs.gpgKeyFiles, "gpg-key-file", nil,
		"path to GPG private key file (can be used multiple times)")
	createSecretSOPSCmd.Flags().BoolVar(&createSecretSOPSArgs.ageKeyStdin, "age-key-stdin", false,
		"read Age private key from stdin")
	createSecretSOPSCmd.Flags().BoolVar(&createSecretSOPSArgs.gpgKeyStdin, "gpg-key-stdin", false,
		"read GPG private key from stdin")
	createSecretSOPSCmd.Flags().StringSliceVar(&createSecretSOPSArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretSOPSCmd.Flags().StringSliceVar(&createSecretSOPSArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretSOPSCmd.Flags().BoolVar(&createSecretSOPSArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretSOPSCmd.Flags().BoolVar(&createSecretSOPSArgs.export, "export", false,
		"export resource in YAML format to stdout")

	createSecretCmd.AddCommand(createSecretSOPSCmd)
}

func createSecretSOPSCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]

	// Check that at least one key is provided
	if len(createSecretSOPSArgs.ageKeyFiles) == 0 && len(createSecretSOPSArgs.gpgKeyFiles) == 0 &&
		!createSecretSOPSArgs.ageKeyStdin && !createSecretSOPSArgs.gpgKeyStdin {
		return fmt.Errorf("at least one Age or GPG key must be provided")
	}

	ageKeys := make([]string, 0)
	gpgKeys := make([]string, 0)

	// Read Age key files
	for _, keyFile := range createSecretSOPSArgs.ageKeyFiles {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("unable to read Age key file %s: %w", keyFile, err)
		}
		ageKeys = append(ageKeys, string(keyData))
	}

	// Read GPG key files
	for _, keyFile := range createSecretSOPSArgs.gpgKeyFiles {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("unable to read GPG key file %s: %w", keyFile, err)
		}
		gpgKeys = append(gpgKeys, string(keyData))
	}

	// Read Age key from stdin
	if createSecretSOPSArgs.ageKeyStdin {
		scanner := bufio.NewScanner(rootCmd.InOrStdin())
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("unable to read Age key from stdin: %w", err)
		}
		ageKeys = append(ageKeys, strings.Join(lines, "\n"))
	}

	// Read GPG key from stdin
	if createSecretSOPSArgs.gpgKeyStdin {
		scanner := bufio.NewScanner(rootCmd.InOrStdin())
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("unable to read GPG key from stdin: %w", err)
		}
		gpgKeys = append(gpgKeys, strings.Join(lines, "\n"))
	}

	// Build the secret
	secret, err := secrets.MakeSOPSSecret(
		name,
		*kubeconfigArgs.Namespace,
		ageKeys,
		gpgKeys,
	)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretSOPSArgs.annotations,
		createSecretSOPSArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretSOPSArgs.export {
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
		secrets.WithImmutable(createSecretSOPSArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

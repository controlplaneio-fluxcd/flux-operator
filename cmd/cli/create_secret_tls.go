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

var createSecretTLSCmd = &cobra.Command{
	Use:   "tls [name]",
	Short: "Create a Kubernetes Secret containing TLS certs",
	Example: `  # Create or update a secret with mTLS certs for Flux operations
  flux-operator create secret tls tls-auth \
  --namespace=flux-system \
  --tls-crt-file=./tls.crt \
  --tls-key-file=./tls.key \
  --ca-crt-file=./ca.crt

  # Generate a secret with a TLS CA cert and export it to a YAML file
  flux-operator -n apps create secret tls tls-ca \
  --ca-crt-file=./ca.crt \
  --export > tls-ca.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretTLSCmdRun,
}

type createSecretTLSFlags struct {
	tlsCrtFile string
	tlsKeyFile string
	caCrtFile  string

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretTLSArgs createSecretTLSFlags

func init() {
	createSecretTLSCmd.Flags().StringVar(&createSecretTLSArgs.tlsCrtFile, "tls-crt-file", "",
		"path to TLS client certificate file")
	createSecretTLSCmd.Flags().StringVar(&createSecretTLSArgs.tlsKeyFile, "tls-key-file", "",
		"path to TLS client private key file")
	createSecretTLSCmd.Flags().StringVar(&createSecretTLSArgs.caCrtFile, "ca-crt-file", "",
		"path to CA certificate file (optional)")
	createSecretTLSCmd.Flags().StringSliceVar(&createSecretTLSArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretTLSCmd.Flags().StringSliceVar(&createSecretTLSArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretTLSCmd.Flags().BoolVar(&createSecretTLSArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretTLSCmd.Flags().BoolVar(&createSecretTLSArgs.export, "export", false,
		"export resource in YAML format to stdout")
	createSecretCmd.AddCommand(createSecretTLSCmd)
}

func createSecretTLSCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]

	tlsOpts := []secrets.TLSSecretOption{}

	// Read client certs if provided
	if createSecretTLSArgs.tlsCrtFile != "" && createSecretTLSArgs.tlsKeyFile != "" {
		tlsCrt, err := os.ReadFile(createSecretTLSArgs.tlsCrtFile)
		if err != nil {
			return fmt.Errorf("unable to read TLS certificate file: %w", err)
		}
		tlsKey, err := os.ReadFile(createSecretTLSArgs.tlsKeyFile)
		if err != nil {
			return fmt.Errorf("unable to read TLS private key file: %w", err)
		}
		tlsOpts = append(tlsOpts, secrets.WithCertKeyPair(tlsCrt, tlsKey))
	}

	// Read CA file if provided
	if createSecretTLSArgs.caCrtFile != "" {
		caCrt, err := os.ReadFile(createSecretTLSArgs.caCrtFile)
		if err != nil {
			return fmt.Errorf("unable to read CA certificate file: %w", err)
		}
		tlsOpts = append(tlsOpts, secrets.WithCAData(caCrt))
	}

	// Build the secret
	secret, err := secrets.MakeTLSSecret(name, *kubeconfigArgs.Namespace, tlsOpts...)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretTLSArgs.annotations,
		createSecretTLSArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretTLSArgs.export {
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
		secrets.WithImmutable(createSecretTLSArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

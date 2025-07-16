// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
)

var createSecretProxyCmd = &cobra.Command{
	Use:   "proxy [name]",
	Short: "Create a Kubernetes Secret containing HTTP/S proxy credentials",
	Example: `  # Create or update a secret with proxy credentials for Flux operations
  echo $PROXY_PASSWORD | flux-operator create secret proxy proxy-auth \
  --namespace=flux-system \
  --address=proxy.example.com:8080 \
  --username=proxyuser \
  --password-stdin

  # Generate a proxy secret and export it to a YAML file
  flux-operator -n apps create secret proxy proxy-auth \
  --address=proxy.example.com:8080 \
  --username=admin \
  --password=secret \
  --export > proxy-auth.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: createSecretProxyCmdRun,
}

type createSecretProxyFlags struct {
	username      string
	password      string
	passwordStdin bool
	address       string

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretProxyArgs createSecretProxyFlags

func init() {
	createSecretProxyCmd.Flags().StringVar(&createSecretProxyArgs.username, "username", "",
		"set the username for proxy authentication")
	createSecretProxyCmd.Flags().StringVar(&createSecretProxyArgs.password, "password", "",
		"set the password for proxy authentication")
	createSecretProxyCmd.Flags().BoolVar(&createSecretProxyArgs.passwordStdin, "password-stdin", false,
		"read the password from stdin")
	createSecretProxyCmd.Flags().StringVar(&createSecretProxyArgs.address, "address", "",
		"set the proxy address (required)")
	createSecretProxyCmd.Flags().StringSliceVar(&createSecretProxyArgs.annotations, "annotation", nil,
		"set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretProxyCmd.Flags().StringSliceVar(&createSecretProxyArgs.labels, "label", nil,
		"set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretProxyCmd.Flags().BoolVar(&createSecretProxyArgs.immutable, "immutable", false,
		"set the immutable flag on the Secret")
	createSecretProxyCmd.Flags().BoolVar(&createSecretProxyArgs.export, "export", false,
		"export resource in YAML format to stdout")
	createSecretCmd.AddCommand(createSecretProxyCmd)
}

func createSecretProxyCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name can be specified")
	}
	name := args[0]
	password := createSecretProxyArgs.password

	// Read the password from stdin if the flag is set
	if createSecretProxyArgs.passwordStdin {
		var input string
		_, err := fmt.Scan(&input)
		if err != nil {
			return fmt.Errorf("unable to read password from stdin: %w", err)
		}
		password = input
	}

	// Build the secret
	secret, err := secrets.MakeProxySecret(
		name,
		*kubeconfigArgs.Namespace,
		createSecretProxyArgs.address,
		createSecretProxyArgs.username,
		password,
	)
	if err != nil {
		return err
	}

	// Set annotations and labels if provided
	if err := setSecretMetadata(
		secret,
		createSecretProxyArgs.annotations,
		createSecretProxyArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	// Export the secret if the export flag is set
	if createSecretProxyArgs.export {
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
		secrets.WithImmutable(createSecretProxyArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied succefuly", secret.GetNamespace(), secret.GetName()))
	return nil
}

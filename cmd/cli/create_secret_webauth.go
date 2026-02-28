// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var createSecretWebAuthCmd = &cobra.Command{
	Use:   "web-auth [name]",
	Short: "Create a Kubernetes Secret containing web UI authentication credentials",
	Example: `  # Create or update a secret with OAuth2 client credentials
  flux-operator create secret web-auth flux-web-auth \
    --namespace=flux-system \
    --client-id=flux-web \
    --client-secret=$client_secret

  # Create a secret with random client secret
  flux-operator create secret web-auth flux-web-client \
    --client-id=flux-web \
    --client-secret-rnd

  # Create a secret with client secret from stdin
  echo $client_secret | flux-operator create secret web-auth flux-web-client \
    --client-id=flux-web \
    --client-secret-stdin

  # Generate a web-auth secret and export it to YAML file
  flux-operator create secret web-auth flux-web-client \
    --client-id=flux-web \
    --client-secret-rnd \
    --export > flux-web-auth.yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: CreateSecretWebAuthCmdRun,
}

type createSecretWebAuthFlags struct {
	clientID          string
	clientSecret      string
	clientSecretStdin bool
	clientSecretRnd   bool

	annotations []string
	labels      []string
	immutable   bool
	export      bool
}

var createSecretWebAuthArgs createSecretWebAuthFlags

func init() {
	createSecretWebAuthCmd.Flags().StringVar(&createSecretWebAuthArgs.clientID, "client-id", "", "set the client ID for OAuth2 authentication (required)")
	createSecretWebAuthCmd.Flags().StringVar(&createSecretWebAuthArgs.clientSecret, "client-secret", "", "set the client secret for OAuth2 authentication (required)")
	createSecretWebAuthCmd.Flags().BoolVar(&createSecretWebAuthArgs.clientSecretStdin, "client-secret-stdin", false, "read the client secret from standard input")
	createSecretWebAuthCmd.Flags().BoolVar(&createSecretWebAuthArgs.clientSecretRnd, "client-secret-rnd", false, "generate a random client secret")
	createSecretWebAuthCmd.Flags().StringSliceVar(&createSecretWebAuthArgs.annotations, "annotation", nil, "set annotations on the resource (can specify multiple annotations with commas: annotation1=value1,annotation2=value2)")
	createSecretWebAuthCmd.Flags().StringSliceVar(&createSecretWebAuthArgs.labels, "label", nil, "set labels on the resource (can specify multiple labels with commas: label1=value1,label2=value2)")
	createSecretWebAuthCmd.Flags().BoolVar(&createSecretWebAuthArgs.immutable, "immutable", false, "set the immutable flag on the Secret")
	createSecretWebAuthCmd.Flags().BoolVar(&createSecretWebAuthArgs.export, "export", false, "export resource in YAML format to stdout")
	createSecretCmd.AddCommand(createSecretWebAuthCmd)
}

func CreateSecretWebAuthCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("a single name must be specified")
	}
	name := args[0]

	if createSecretWebAuthArgs.clientID == "" {
		return fmt.Errorf("--client-id is required")
	}
	clientSecret := createSecretWebAuthArgs.clientSecret

	secretSources := 0
	if createSecretWebAuthArgs.clientSecret != "" {
		secretSources++
	}
	if createSecretWebAuthArgs.clientSecretStdin {
		secretSources++
	}
	if createSecretWebAuthArgs.clientSecretRnd {
		secretSources++
	}
	if secretSources == 0 {
		return fmt.Errorf("one of --client-secret, --client-secret-stdin, or --client-secret-rnd must be specified")
	}
	if secretSources > 1 {
		return fmt.Errorf("only one of --client-secret, --client-secret-stdin, or --client-secret-rnd can be specified")
	}

	if createSecretWebAuthArgs.clientSecretStdin {
		var input string
		_, err := fmt.Scan(&input)
		if err != nil {
			return fmt.Errorf("unable to read client secret from stdin: %w", err)
		}
		clientSecret = input
	}
	if createSecretWebAuthArgs.clientSecretRnd {
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			return fmt.Errorf("unable to generate random client secret: %w", err)
		}
		clientSecret = base64.RawURLEncoding.EncodeToString(randomBytes)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: *kubeconfigArgs.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"client-id":     createSecretWebAuthArgs.clientID,
			"client-secret": clientSecret,
		},
	}
	secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	if err := setSecretMetadata(
		secret,
		createSecretWebAuthArgs.annotations,
		createSecretWebAuthArgs.labels,
	); err != nil {
		return fmt.Errorf("unable to set metadata on secret: %w", err)
	}

	if createSecretWebAuthArgs.export {
		return printSecret(secret)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}
	err = secrets.Apply(
		ctx,
		kubeClient,
		secret,
		secrets.WithForce(),
		secrets.WithImmutable(createSecretWebAuthArgs.immutable),
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied successfully", secret.GetNamespace(), secret.GetName()))
	return nil
}

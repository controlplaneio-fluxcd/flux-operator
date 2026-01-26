// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/runtime/secrets"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

type webConfigFlags struct {
	baseURL           string
	provider          string
	issuerURL         string
	clientID          string
	clientSecret      string
	clientSecretRnd   bool
	clientSecretStdin bool
	export            bool
}

var createSecretWebConfigCmd = &cobra.Command{
	Use:   "web-config [name]",
	Short: "Create a Kubernetes secret with Flux Web configuration",
	Long: `The create secret web-config command generates a Kubernetes secret containing
the configuration for Flux Web authentication.

The secret contains a config.yaml file with OAuth2 authentication settings
that can be mounted into the Flux Web deployment.`,
	Example: `  # Create a web-config secret with OIDC authentication
  flux-operator create secret web-config flux-web-config \
    --namespace=flux-system \
    --base-url=https://flux.example.com \
    --provider=OIDC \
    --issuer-url=https://dex.example.com \
    --client-id=my-client-id \
    --client-secret=my-client-secret

  # Generate a random client secret
  flux-operator create secret web-config flux-web-config \
    --namespace=flux-system \
    --base-url=https://flux.example.com \
    --provider=OIDC \
    --issuer-url=https://dex.example.com \
    --client-id=my-client-id \
    --client-secret-rnd

  # Read client secret from stdin
  echo "my-secret" | flux-operator create secret web-config flux-web-config \
    --namespace=flux-system \
    --base-url=https://flux.example.com \
    --provider=OIDC \
    --issuer-url=https://dex.example.com \
    --client-id=my-client-id \
    --client-secret-stdin`,
	RunE: createSecretWebConfigCmdRun,
}

var webConfigArgs webConfigFlags

func init() {
	createSecretCmd.AddCommand(createSecretWebConfigCmd)

	createSecretWebConfigCmd.Flags().StringVar(&webConfigArgs.baseURL, "base-url", "",
		"base URL where Flux Web is accessible (required)")
	createSecretWebConfigCmd.Flags().StringVar(&webConfigArgs.provider, "provider", "OIDC",
		"OAuth2 provider type (e.g., OIDC, GitHub, Google)")
	createSecretWebConfigCmd.Flags().StringVar(&webConfigArgs.issuerURL, "issuer-url", "",
		"OIDC issuer URL (required for OIDC provider)")
	createSecretWebConfigCmd.Flags().StringVar(&webConfigArgs.clientID, "client-id", "",
		"OAuth2 client ID (required)")
	createSecretWebConfigCmd.Flags().StringVar(&webConfigArgs.clientSecret, "client-secret", "",
		"OAuth2 client secret")
	createSecretWebConfigCmd.Flags().BoolVar(&webConfigArgs.clientSecretRnd, "client-secret-rnd", false,
		"generate a random client secret")
	createSecretWebConfigCmd.Flags().BoolVar(&webConfigArgs.clientSecretStdin, "client-secret-stdin", false,
		"read client secret from stdin")
	createSecretWebConfigCmd.Flags().BoolVar(&webConfigArgs.export, "export", false,
		"export resource in YAML format to stdout")

	_ = createSecretWebConfigCmd.MarkFlagRequired("base-url")
	_ = createSecretWebConfigCmd.MarkFlagRequired("client-id")
}

func createSecretWebConfigCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("secret name is required")
	}
	secretName := args[0]

	if err := validateWebConfigFlags(); err != nil {
		return err
	}

	clientSecret, err := getClientSecret()
	if err != nil {
		return fmt.Errorf("failed to get client secret: %w", err)
	}

	config := fluxcdv1.WebConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.WebConfigGroupVersion.String(),
			Kind:       fluxcdv1.WebConfigKind,
		},
		Spec: fluxcdv1.WebConfigSpec{
			BaseURL: webConfigArgs.baseURL,
			Authentication: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
				OAuth2: &fluxcdv1.OAuth2AuthenticationSpec{
					Provider:     webConfigArgs.provider,
					ClientID:     webConfigArgs.clientID,
					ClientSecret: clientSecret,
					IssuerURL:    webConfigArgs.issuerURL,
				},
			},
		},
	}

	configYAML, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal web config: %w", err)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: *kubeconfigArgs.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"config.yaml": string(configYAML),
		},
	}

	if webConfigArgs.export {
		return printSecret(secret)
	}

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
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Secret %s/%s applied successfully", secret.GetNamespace(), secret.GetName()))
	return nil
}

func validateWebConfigFlags() error {
	if webConfigArgs.provider == "OIDC" && webConfigArgs.issuerURL == "" {
		return fmt.Errorf("--issuer-url is required when using OIDC provider")
	}

	secretMethods := 0
	if webConfigArgs.clientSecret != "" {
		secretMethods++
	}
	if webConfigArgs.clientSecretRnd {
		secretMethods++
	}
	if webConfigArgs.clientSecretStdin {
		secretMethods++
	}

	if secretMethods == 0 {
		return fmt.Errorf("one of --client-secret, --client-secret-rnd, or --client-secret-stdin must be specified")
	}
	if secretMethods > 1 {
		return fmt.Errorf("only one of --client-secret, --client-secret-rnd, or --client-secret-stdin can be specified")
	}

	return nil
}

func getClientSecret() (string, error) {
	if webConfigArgs.clientSecret != "" {
		return webConfigArgs.clientSecret, nil
	}

	if webConfigArgs.clientSecretRnd {
		return generateRandomSecret(32)
	}

	if webConfigArgs.clientSecretStdin {
		var secret string
		_, err := fmt.Scan(&secret)
		if err != nil {
			return "", fmt.Errorf("unable to read secret from stdin: %w", err)
		}

		secretStr := secret
		secretStr = strings.TrimSpace(secretStr)

		if secretStr == "" {
			return "", fmt.Errorf("client secret read from stdin is empty")
		}
		return secretStr, nil
	}

	return "", fmt.Errorf("no client secret method specified")
}

func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

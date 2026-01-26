// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"strings"
	"testing"
)

func TestCreateSecretWebConfig(t *testing.T) {

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		assertFunc func(t *testing.T, output string)
	}{
		{
			name: "create web-config secret with all flags",
			args: []string{
				"create", "secret", "web-config", "test-secret",
				"--namespace=test-namespace",
				"--base-url=https://flux.example.com",
				"--provider=OIDC",
				"--issuer-url=https://dex.example.com",
				"--client-id=test-client-id",
				"--client-secret=test-client-secret",
				"--export",
			},
			wantErr: false,
			assertFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "kind: Secret") {
					t.Error("output should contain 'kind: Secret'")
				}
				if !strings.Contains(output, "name: test-secret") {
					t.Error("output should contain secret name")
				}
				if !strings.Contains(output, "config.yaml") {
					t.Error("output should contain config.yaml key")
				}
				if !strings.Contains(output, "baseURL: https://flux.example.com") {
					t.Error("output should contain base URL")
				}
			},
		},
		{
			name: "fail without base-url",
			args: []string{
				"create", "secret", "web-config", "test-secret",
				"--client-id=test-client-id",
				"--client-secret=test-secret",
			},
			wantErr: true,
		},
		{
			name: "fail without client-id",
			args: []string{
				"create", "secret", "web-config", "test-secret",
				"--base-url=https://flux.example.com",
				"--client-secret=test-secret",
			},
			wantErr: true,
		},
		{
			name: "generate random secret",
			args: []string{
				"create", "secret", "web-config", "test-secret",
				"--namespace=test-namespace",
				"--base-url=https://flux.example.com",
				"--provider=OIDC",
				"--issuer-url=https://dex.example.com",
				"--client-id=test-client-id",
				"--client-secret-rnd",
				"--export",
			},
			wantErr: false,
			assertFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, "clientSecret:") {
					t.Error("output should contain clientSecret field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global flags between tests
			webConfigArgs = webConfigFlags{}

			output, err := executeCommand(tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}

			if tt.assertFunc != nil && !tt.wantErr {
				tt.assertFunc(t, output)
			}
		})
	}
}

func TestGenerateRandomSecret(t *testing.T) {
	secret, err := generateRandomSecret(32)
	if err != nil {
		t.Fatalf("failed to generate random secret: %v", err)
	}

	if secret == "" {
		t.Error("generated secret is empty")
	}

	if strings.ContainsAny(secret, " \n\t") {
		t.Error("generated secret contains whitespace")
	}

	secret2, err := generateRandomSecret(32)
	if err != nil {
		t.Fatalf("failed to generate second random secret: %v", err)
	}

	if secret == secret2 {
		t.Error("two random secrets are identical (very unlikely)")
	}
}

func TestValidateWebConfigFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   webConfigFlags
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid OIDC configuration",
			flags: webConfigFlags{
				baseURL:      "https://flux.example.com",
				provider:     "OIDC",
				issuerURL:    "https://dex.example.com",
				clientID:     "test-client",
				clientSecret: "test-secret",
			},
			wantErr: false,
		},
		{
			name: "OIDC without issuer URL",
			flags: webConfigFlags{
				baseURL:      "https://flux.example.com",
				provider:     "OIDC",
				clientID:     "test-client",
				clientSecret: "test-secret",
			},
			wantErr: true,
			errMsg:  "issuer-url is required",
		},
		{
			name: "no secret method",
			flags: webConfigFlags{
				baseURL:   "https://flux.example.com",
				provider:  "OIDC",
				issuerURL: "https://dex.example.com",
				clientID:  "test-client",
			},
			wantErr: true,
			errMsg:  "one of --client-secret",
		},
		{
			name: "multiple secret methods",
			flags: webConfigFlags{
				baseURL:         "https://flux.example.com",
				provider:        "OIDC",
				issuerURL:       "https://dex.example.com",
				clientID:        "test-client",
				clientSecret:    "test-secret",
				clientSecretRnd: true,
			},
			wantErr: true,
			errMsg:  "only one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global flags
			webConfigArgs = tt.flags

			err := validateWebConfigFlags()

			if (err != nil) != tt.wantErr {
				t.Errorf("validateWebConfigFlags() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error message = %v, should contain %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestGetClientSecret(t *testing.T) {
	tests := []struct {
		name      string
		flags     webConfigFlags
		wantErr   bool
		checkFunc func(t *testing.T, secret string)
	}{
		{
			name: "direct secret",
			flags: webConfigFlags{
				clientSecret: "my-direct-secret",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, secret string) {
				if secret != "my-direct-secret" {
					t.Errorf("expected 'my-direct-secret', got '%s'", secret)
				}
			},
		},
		{
			name: "random secret",
			flags: webConfigFlags{
				clientSecretRnd: true,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, secret string) {
				if len(secret) == 0 {
					t.Error("random secret is empty")
				}
				if len(secret) < 40 {
					t.Errorf("random secret too short: %d chars", len(secret))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webConfigArgs = tt.flags

			secret, err := getClientSecret()

			if (err != nil) != tt.wantErr {
				t.Errorf("getClientSecret() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, secret)
			}
		})
	}
}

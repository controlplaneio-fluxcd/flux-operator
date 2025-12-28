// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestLoad(t *testing.T) {
	for _, tt := range []struct {
		name            string
		content         string
		createFile      bool
		wantErr         string
		expectedVersion string
		validate        func(g Gomega, spec *ConfigSpec)
	}{
		{
			name:            "empty filename returns defaults",
			content:         "",
			createFile:      false,
			wantErr:         "",
			expectedVersion: "no-config",
			validate: func(g Gomega, spec *ConfigSpec) {
				g.Expect(spec).NotTo(BeNil())
			},
		},
		{
			name:       "non-existent file returns error",
			content:    "",
			createFile: false,
			wantErr:    "no such file or directory",
		},
		{
			name: "valid config file returns parsed config",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    type: Anonymous
    anonymous:
      username: test-user
`,
			createFile:      true,
			wantErr:         "",
			expectedVersion: "static-file",
			validate: func(g Gomega, spec *ConfigSpec) {
				g.Expect(spec.Authentication).NotTo(BeNil())
				g.Expect(spec.Authentication.Type).To(Equal(AuthenticationTypeAnonymous))
				g.Expect(spec.Authentication.Anonymous.Username).To(Equal("test-user"))
			},
		},
		{
			name:       "invalid YAML returns error",
			content:    "invalid: yaml: content: [",
			createFile: true,
			wantErr:    "invalid configuration in web config file",
		},
		{
			name: "valid YAML but invalid config returns error",
			content: `apiVersion: wrong/v1
kind: Config
spec: {}
`,
			createFile: true,
			wantErr:    "invalid configuration in web config file",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var filename string
			if tt.name == "empty filename returns defaults" {
				filename = ""
			} else if tt.createFile {
				tmpDir := t.TempDir()
				filename = filepath.Join(tmpDir, "config.yaml")
				err := os.WriteFile(filename, []byte(tt.content), 0644)
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				filename = "/non/existent/path/config.yaml"
			}

			spec, err := Load(filename)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(spec).NotTo(BeNil())
				g.Expect(spec.Version).To(Equal(tt.expectedVersion))
				if tt.validate != nil {
					tt.validate(g, spec)
				}
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name     string
		content  string
		wantErr  string
		validate func(g Gomega, spec *ConfigSpec)
	}{
		{
			name: "valid YAML with Anonymous auth",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    type: Anonymous
    anonymous:
      username: test-user
      groups:
        - group1
        - group2
`,
			wantErr: "",
			validate: func(g Gomega, spec *ConfigSpec) {
				g.Expect(spec.Authentication).NotTo(BeNil())
				g.Expect(spec.Authentication.Type).To(Equal(AuthenticationTypeAnonymous))
				g.Expect(spec.Authentication.Anonymous.Username).To(Equal("test-user"))
				g.Expect(spec.Authentication.Anonymous.Groups).To(Equal([]string{"group1", "group2"}))
				// defaults should be applied
				g.Expect(spec.Authentication.SessionDuration).NotTo(BeNil())
				g.Expect(spec.Authentication.UserCacheSize).To(Equal(100))
			},
		},
		{
			name: "valid YAML with OAuth2 auth",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://status.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: my-client-id
      clientSecret: my-secret
      issuerURL: https://issuer.example.com
      scopes:
        - openid
        - profile
`,
			wantErr: "",
			validate: func(g Gomega, spec *ConfigSpec) {
				g.Expect(spec.BaseURL).To(Equal("https://status.example.com"))
				g.Expect(spec.Authentication).NotTo(BeNil())
				g.Expect(spec.Authentication.Type).To(Equal(AuthenticationTypeOAuth2))
				g.Expect(spec.Authentication.OAuth2.Provider).To(Equal(OAuth2ProviderOIDC))
				g.Expect(spec.Authentication.OAuth2.ClientID).To(Equal("my-client-id"))
				g.Expect(spec.Authentication.OAuth2.IssuerURL).To(Equal("https://issuer.example.com"))
				g.Expect(spec.Authentication.OAuth2.Scopes).To(Equal([]string{"openid", "profile"}))
				// defaults should be applied
				g.Expect(spec.Authentication.OAuth2.Profile).NotTo(BeNil())
				g.Expect(spec.Authentication.OAuth2.Impersonation).NotTo(BeNil())
			},
		},
		{
			name:    "invalid YAML syntax",
			content: "invalid: yaml: [content",
			wantErr: "yaml:",
		},
		{
			name: "valid YAML wrong apiVersion",
			content: `apiVersion: wrong/v1
kind: Config
spec: {}
`,
			wantErr: "expected apiVersion 'web.fluxcd.controlplane.io/v1'",
		},
		{
			name: "valid YAML wrong kind",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: WrongKind
spec: {}
`,
			wantErr: "expected apiVersion 'web.fluxcd.controlplane.io/v1' and kind 'Config'",
		},
		{
			name: "valid YAML invalid auth config",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    type: InvalidType
`,
			wantErr: "invalid authentication type 'InvalidType'",
		},
		{
			name: "OAuth2 without baseURL",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: client-id
      clientSecret: secret
      issuerURL: https://issuer.example.com
`,
			wantErr: "baseURL must be set when OAuth2 authentication is configured",
		},
		{
			name: "config with insecure mode",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  insecure: true
  authentication:
    type: Anonymous
    anonymous:
      username: test-user
`,
			wantErr: "",
			validate: func(g Gomega, spec *ConfigSpec) {
				g.Expect(spec.Insecure).To(BeTrue())
			},
		},
		{
			name: "config with custom claims processor",
			content: `apiVersion: web.fluxcd.controlplane.io/v1
kind: Config
spec:
  baseURL: https://status.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: client-id
      clientSecret: secret
      issuerURL: https://issuer.example.com
      variables:
        - name: domain
          expression: "claims.email.split('@')[1]"
      validations:
        - expression: "variables.domain == 'example.com'"
          message: "Only example.com users allowed"
      profile:
        name: claims.preferred_username
      impersonation:
        username: claims.sub
        groups: claims.roles
`,
			wantErr: "",
			validate: func(g Gomega, spec *ConfigSpec) {
				g.Expect(spec.Authentication.OAuth2.Variables).To(HaveLen(1))
				g.Expect(spec.Authentication.OAuth2.Variables[0].Name).To(Equal("domain"))
				g.Expect(spec.Authentication.OAuth2.Validations).To(HaveLen(1))
				g.Expect(spec.Authentication.OAuth2.Profile.Name).To(Equal("claims.preferred_username"))
				g.Expect(spec.Authentication.OAuth2.Impersonation.Username).To(Equal("claims.sub"))
				g.Expect(spec.Authentication.OAuth2.Impersonation.Groups).To(Equal("claims.roles"))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			spec, err := parse([]byte(tt.content))
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(spec).NotTo(BeNil())
				if tt.validate != nil {
					tt.validate(g, spec)
				}
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

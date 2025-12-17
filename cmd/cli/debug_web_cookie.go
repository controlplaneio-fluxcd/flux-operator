// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
)

var debugWebCookieCmd = &cobra.Command{
	Use:   "cookie",
	Short: "Debug Flux Operator Web UI cookies",
	Long: `Interactively debug the Web UI cookies.

This command will decode and print the Web UI cookies
for inspection while hiding sensitive information.

The command requires the cookie to be provided via
standard input. It is expected to be a base64-encoded
JSON object.`,
	Args: cobra.NoArgs,
	RunE: debugWebCookieCmdRun,
}

func init() {
	debugWebCmd.AddCommand(debugWebCookieCmd)
}

func debugWebCookieCmdRun(*cobra.Command, []string) error {
	ctx := ctrl.SetupSignalHandler()

	// Read cookie from stdin.
	fmt.Print("Paste cookie value and press enter:\n\n")
	var input string
	if _, err := fmt.Scan(&input); err != nil {
		return fmt.Errorf("failed to read cookie from stdin: %w", err)
	}

	// Decode cookie.
	b, err := base64.RawURLEncoding.DecodeString(input)
	if err != nil {
		return fmt.Errorf("failed to decode base64 cookie: %w", err)
	}
	var content map[string]any
	if err := json.Unmarshal(b, &content); err != nil {
		return fmt.Errorf("failed to unmarshal cookie JSON: %w", err)
	}

	// Save values before redacting.
	accessToken, hasAccessToken := content["accessToken"].(string)

	// Print cookie values.
	fmt.Print("\nCookie values:\n\n")
	for k := range content {
		if strings.HasSuffix(strings.ToLower(k), "token") {
			content[k] = "******"
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(content)

	// Print access token information.
	if !hasAccessToken || accessToken == "" {
		return nil
	}
	fmt.Print("\nAccess token information:\n\n")
	var claims jwt.MapClaims
	if _, _, err := jwt.NewParser().ParseUnverified(accessToken, &claims); err != nil {
		fmt.Println("The access token does not seem to be a JWT.")
		return nil
	}
	defer enc.Encode(claims)
	issuer, err := claims.GetIssuer()
	if err != nil {
		fmt.Print("The JWT access token does not seem to have an issuer.\n\n")
		return nil
	}
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		fmt.Print("The JWT access token issuer does not seem to be an OIDC issuer.\n\n")
		return nil
	}
	audiences, err := claims.GetAudience()
	if err != nil {
		fmt.Print("The JWT access token does not seem to have an audience.\n\n")
		return nil
	}
	if len(audiences) == 0 {
		fmt.Print("The JWT access token does not seem to have an audience.\n\n")
		return nil
	}
	if len(audiences) > 1 {
		fmt.Printf("The JWT access token has multiple audiences; using the first one for verification.\n\n")
	}
	_, err = p.VerifierContext(ctx, &oidc.Config{ClientID: audiences[0]}).Verify(ctx, accessToken)
	if err != nil {
		fmt.Print("The JWT access token could not be verified as an OIDC token.\n\n")
		return nil
	}

	return nil
}

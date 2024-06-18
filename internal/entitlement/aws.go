// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package entitlement

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacemetering"
	"github.com/golang-jwt/jwt/v4"
)

const (
	// awsMarketplaceProductCode is the AWS Marketplace
	// product code of ControlPlane Enterprise for Flux CD.
	awsMarketplaceProductCode = "272knt6mdmtctbck10givwm1h"
	// awsMarketplacePublicKeyVersion is the AWS Marketplace
	// public key version of ControlPlane Enterprise for Flux CD.
	awsMarketplacePublicKeyVersion = 1
	// awsMarketplacePublicKey is the AWS Marketplace
	// public key of ControlPlane Enterprise for Flux CD.
	awsMarketplacePublicKey = `-----BEGIN PUBLIC KEY-----
MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEAnFKsHLJY5om6uta6LfG/
3tfnLO08ZRHeZrromJSEyG+/zdmVJ7s8thk4JQzNxt8fUzgMvKdPuWl17vYay18P
pCiypg6EbihzIO3VOQCGp1bOSnHrvUlyoyhDuNnG8213DFbl4+MmVwEkI4F25sUq
56uwmZyHc77ZsjvvFs0pcJ0VQ+DhG0LSjUMmtukeh2VQ29yuQCiKCML4JOkwIRuP
cmetwvbgn1ViFWwSrhE2i/cNjBzXdd1kz23rmLM4rx4LctsUSAIP3I5YRy4wLUiG
q+M3YOAcfxQP3t5cjN7rRfyE/bUz+BvipKEPCoDMKmbbNRyX9WYPeIrRsW4HJWGj
GO0dKDZJJwhMa5TM5zVwepfLeGakxprL7j+0EGFWvf8M0+qZ9OGgEFNVwDVu2BoH
d1prsH3fI7CTCztrIBgCwtqBhQ5wxzlrnBrDy4WA+CwLFhW77Tghw1E62Vcpj/v5
vLhgbv3IpMBX2ugEWgeB2i0yWYCpheC8lkbgI90SY+GFAgMBAAE=
-----END PUBLIC KEY-----`
)

// AmazonClient is an entitlement client for the
// ControlPlane Enterprise for Flux CD AWS Marketplace product.
// https://aws.amazon.com/marketplace/pp/prodview-ndm54wno7tayg
type AmazonClient struct {
	Vendor string
	mc     *marketplacemetering.Client
}

// NewAmazonClient creates a new AmazonClient using the default
// AWS configuration and the current region.
func NewAmazonClient(vendor string) (*AmazonClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithEC2IMDSRegion())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	return &AmazonClient{
		Vendor: vendor,
		mc:     marketplacemetering.NewFromConfig(cfg),
	}, nil
}

// RegisterUsage registers the usage with AWS Marketplace
// metering service and returns a JWT token.
func (c *AmazonClient) RegisterUsage(ctx context.Context, id string) (string, error) {
	input := &marketplacemetering.RegisterUsageInput{
		ProductCode:      aws.String(awsMarketplaceProductCode),
		PublicKeyVersion: aws.Int32(awsMarketplacePublicKeyVersion),
		Nonce:            aws.String(id),
	}

	output, err := c.mc.RegisterUsage(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to register usage with AWS Marketplace: %w", err)
	}

	return aws.ToString(output.Signature), nil
}

// Verify verifies the JWT token is signed with the AWS Marketplace public key
// and checks the product code, nonce and public key version claims.
func (c *AmazonClient) Verify(token, id string) (bool, error) {
	t, err := jwt.ParseWithClaims(token, jwt.MapClaims{}, func(_ *jwt.Token) (any, error) {
		return jwt.ParseRSAPublicKeyFromPEM([]byte(awsMarketplacePublicKey))
	})
	if err != nil {
		return false, fmt.Errorf("AWS Marketplace invalid token: %w", err)
	}

	if !t.Valid {
		return false, fmt.Errorf("AWS Marketplace invalid token")
	}

	claims := t.Claims.(jwt.MapClaims)
	switch {
	case claims["productCode"] != awsMarketplaceProductCode:
		return false, fmt.Errorf("AWS Marketplace product code mismatch: %s", claims["productCode"])
	case claims["nonce"] != id:
		return false, fmt.Errorf("AWS Marketplace nonce mismatch: %s", claims["nonce"])
	case claims["publicKeyVersion"] != float64(awsMarketplacePublicKeyVersion):
		return false, fmt.Errorf("AWS Marketplace public key version mismatch: %f", claims["publicKeyVersion"])
	}

	return true, nil
}

// GetVendor returns the vendor name.
func (c *AmazonClient) GetVendor() string {
	return c.Vendor
}

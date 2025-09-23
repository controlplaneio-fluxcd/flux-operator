// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package lkm (License Key Management) provides cryptographic license key
// management and attestation functionality for the Flux distribution.
//
// The lkm package is built on industry-standard cryptographic primitives:
//
//   - Ed25519 digital signatures compliant with FIPS 186-5
//   - ECDH-ES key agreement for secure key exchange
//   - SHA-256 hashing for secure data integrity checks
//   - JSON Web Key (JWK) format for interoperable key management
//   - JSON Web Token (JWT) format following RFC 7519 for standardized claims
//   - JSON Web Encryption (JWE) for secure data transmission
//   - UUID v6 for unique, chronologically sortable identifiers
//
// The package supports creating and managing software licenses with the
// following features:
//
//   - EdDSA-based license signing and verification using Ed25519 keys
//   - JWT-based license keys with standard header and payload structure
//   - Time-based license expiration with customizable validity periods
//   - Capability-based access control for fine-grained feature management
//   - Support for license key revocation and ledger management
//
// The package provides attestation functionality for verifying the integrity
// and authenticity of software artifacts and Kubernetes manifests:
//
//   - OCI artifacts and container image attestation with SHA-256 digest verification
//   - Manifest attestation using directory hash checksums
//   - Cryptographic signing and verification of attestation claims
//   - Support for offline verification workflows
//
// The package provides secure secrets exchange with the following features:
//
//   - Generating ECC (Elliptic Curve) public/private key pairs
//   - Formatting public keys as JWKs for distribution
//   - Encrypting payloads into the JWE compact format using ECDH-ES key agreement
//   - Decrypting JWE tokens using the recipient's private ECC key
//
// The lkm package is designed to serve both the Flux Operator command-line interface
// and the Kubernetes controller, enabling secure license management and artifact
// attestation for the Flux distribution.
package lkm

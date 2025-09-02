# Flux Operator License Key Management (LKM)

Flux Operator LKM provides cryptographic license key management and attestation functionality
for the Flux distribution. LKM is built on industry-standard cryptographic primitives:

- **Ed25519 digital signatures** compliant with FIPS 186-5 for license signing
- **ECDH-ES key agreement** for secure key exchange
- **SHA-256 hashing** for data integrity verification
- **JSON Web Key (JWK)** format for interoperable key management
- **JSON Web Token (JWT)** format following RFC 7519 for standardized claims
- **JSON Web Encryption (JWE)** for secure data transmission
- **UUID v6** for unique, chronologically sortable identifiers

## Core Capabilities

LKM serves both the Flux Operator command-line interface and the Kubernetes controller,
enabling comprehensive license management and artifact attestation across the Flux ecosystem.

### License Management

- EdDSA-based license signing and verification using Ed25519 keys
- JWT-based license keys with standard header and payload structure
- Time-based license expiration with customizable validity periods
- Capability-based access control for fine-grained feature management
- License key revocation and ledger management

### Attestation Services

- OCI artifacts and container image attestation with SHA-256 digest verification
- Manifest attestation using directory hash checksums
- Cryptographic signing and verification of attestation claims
- Support for offline verification workflows

### Secure Secrets Exchange

- ECC (Elliptic Curve) public/private key pair generation
- JWK formatting for public key distribution
- JWE compact format encryption using ECDH-ES key agreement
- JWE token decryption using recipient's private ECC key

## Flux Operator CLI Distro Operations

Build the CLI binary from source code:

```shell
make cli-build
```

Create the directory structure for the distro operations:

```shell
mkdir -p \
bin/distro/keys \
bin/distro/licenses \
bin/distro/attestations \
bin/distro/secrets
```

### Bootstrap the JSON Web Key Sets

Generate encryption and signing keys:

```shell
bin/flux-operator-cli distro keygen enc fluxcd.control-plane.io \
--output-dir=bin/distro/keys

bin/flux-operator-cli distro keygen sig https://fluxcd.control-plane.io \
--output-dir=bin/distro/keys
```

Export the JWKS to env variables:

```shell
export FLUX_DISTRO_ENC_PRIVATE_JWKS=$(cat bin/distro/keys/*enc-private.jwks)
export FLUX_DISTRO_ENC_PUBLIC_JWKS=$(cat bin/distro/keys/*enc-public.jwks)
export FLUX_DISTRO_SIG_PRIVATE_JWKS=$(cat bin/distro/keys/*sig-private.jwks)
export FLUX_DISTRO_SIG_PUBLIC_JWKS=$(cat bin/distro/keys/*sig-public.jwks)
```

### Working with licenses

Create a license:

```shell
bin/flux-operator-cli distro sign license-key \
--customer="ControlPlane Group Limited" \
--duration=365 \
--capabilities="feature1,feature2" \
--output=bin/distro/licenses/cp-license.jwt
```

Verify a license:

```shell
bin/flux-operator-cli distro verify license-key \
bin/distro/licenses/cp-license.jwt
```

Revoke a license:

```shell
bin/flux-operator-cli distro revoke license-key \
bin/distro/licenses/cp-license.jwt \
--output=bin/distro/licenses/revocations.json
```

Verify a license against the revocation set:

```shell
bin/flux-operator-cli distro verify license-key \
bin/distro/licenses/cp-license.jwt \
--revoked-set=bin/distro/licenses/revocations.json
```

### Working with attestations

Create an attestation for artifacts:

```shell
bin/flux-operator-cli distro sign artifacts \
--attestation=bin/distro/attestations/flux-v2.6.4.jwt \
--url=ghcr.io/fluxcd/source-controller:v1.6.2 \
--url=ghcr.io/fluxcd/kustomize-controller:v1.6.1 \
--url=ghcr.io/fluxcd/notification-controller:v1.6.0 \
--url=ghcr.io/fluxcd/helm-controller:v1.3.0 \
--url=ghcr.io/fluxcd/image-reflector-controller:v0.35.2 \
--url=ghcr.io/fluxcd/image-automation-controller:v0.41.2
```

Verify the attestation of artifacts:

```shell
bin/flux-operator-cli distro verify artifacts \
--attestation=bin/distro/attestations/flux-v2.6.4.jwt \
--url=ghcr.io/fluxcd/source-controller:v1.6.2 \
--url=ghcr.io/fluxcd/kustomize-controller:v1.6.1 \
--url=ghcr.io/fluxcd/notification-controller:v1.6.0 \
--url=ghcr.io/fluxcd/helm-controller:v1.3.0 \
--url=ghcr.io/fluxcd/image-reflector-controller:v0.35.2 \
--url=ghcr.io/fluxcd/image-automation-controller:v0.41.2
```

Create an attestation for manifests:

```shell
bin/flux-operator-cli distro sign manifests api/ \
--attestation=bin/distro/attestations/api-source.jwt
```

Verify the attestation of manifests:

```shell
bin/flux-operator-cli distro verify manifests api/ \
--attestation=bin/distro/attestations/api-source.jwt
```

### Working with secrets

Encrypt a token:

```shell
bin/flux-operator-cli distro encrypt token \
--input=bin/distro/licenses/cp-license.jwt \
--output=bin/distro/secrets/cp-license.jwe
```

Decrypt a token:

```shell
bin/flux-operator-cli distro decrypt token \
--input=bin/distro/secrets/cp-license.jwe \
--output=bin/distro/secrets/cp-license.jwt
```

Encrypt manifests:

```shell
bin/flux-operator-cli distro encrypt manifests api/ \
--output=bin/distro/secrets/api-source.jwe
```

Decrypt manifests:

```shell
bin/flux-operator-cli distro decrypt manifests \
bin/distro/secrets/api-source.jwe \
--output-dir=bin/distro/secrets/api-source/
```

### Clean up

Unset the environment variables:

```shell
unset FLUX_DISTRO_ENC_PRIVATE_JWKS
unset FLUX_DISTRO_ENC_PUBLIC_JWKS
unset FLUX_DISTRO_SIG_PRIVATE_JWKS
unset FLUX_DISTRO_SIG_PUBLIC_JWKS
```

Remove the distro directory

```shell
rm -rf bin/distro
```

# Import tools
FROM ghcr.io/fluxcd/flux-cli:v2.9.2@sha256:816466603e4b2e30b4fce6ecc0df49c255660deb15f625ec84c9a0dc3c55b1dd AS flux-cli
FROM ghcr.io/fluxcd/flux-schema:v0.10.2@sha256:4283d470c8c1777722266f96e3c5bd5daad2751ea4102dfed7e99fcec21d554c AS flux-schema
FROM ghcr.io/fluxcd/flux-mirror:v0.8.0@sha256:432117c67f83ae3c78d2fd63a4023876b51eaeafbde6e9efb0b42788d86e7695 AS flux-mirror
FROM registry.k8s.io/kubectl:v1.36.2@sha256:b0d792e0d8dfb9bb1b922b78b23137e2a34bb6f9667640353a9d2aadd1fd7761 AS kubectl

# Build the Flux Operator CLI binary using Docker's Debian image.
FROM --platform=${BUILDPLATFORM} golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION

WORKDIR /workspace

# Copy the Go Modules manifests.
COPY go.mod go.mod
COPY go.sum go.sum

# Cache the Go Modules
RUN go mod download

# Copy the Go sources.
COPY cmd/cli/ cmd/cli/
COPY api/ api/
COPY internal/ internal/

# Build the Flux Operator CLI binary.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.VERSION=${VERSION}" \
    -a -o /usr/local/bin/flux-operator ./cmd/cli/

# Verify the Flux Operator CLI binary.
RUN flux-operator version --client

# Distribute the binaries using Google's Distroless image
# that contains busybox for running shell scripts and debugging.
FROM gcr.io/distroless/static:debug-nonroot

# Copy the license.
COPY LICENSE /licenses/LICENSE

# Copy the catalog for offline schema validation.
COPY --from=flux-schema --chown=nonroot:nonroot /catalog /catalog

# Copy the binaries.
COPY --from=builder --chown=nonroot:nonroot /usr/local/bin/flux-operator /usr/local/bin/flux-operator
COPY --from=flux-cli --chown=nonroot:nonroot /usr/local/bin/flux /usr/local/bin/flux
COPY --from=flux-schema --chown=nonroot:nonroot /usr/local/bin/flux-schema /usr/local/bin/flux-schema
COPY --from=flux-mirror --chown=nonroot:nonroot /usr/local/bin/flux-mirror /usr/local/bin/flux-mirror
COPY --from=kubectl --chown=nonroot:nonroot /bin/kubectl /usr/local/bin/kubectl

ENTRYPOINT ["flux-operator"]

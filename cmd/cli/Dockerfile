# Import tools
FROM ghcr.io/fluxcd/flux-cli:v2.8.6@sha256:8dac34106b9d684ce07b13c9762b5132813c3d477cfd724e9e13cc5944ad771e AS flux-cli
FROM ghcr.io/fluxcd/flux-schema:v0.3.0@sha256:92913be498fc0398d8dc2eb749444b81d1f2da3b0e7503177f7a5447ef907a31 AS flux-schema
FROM registry.k8s.io/kubectl:v1.36.0@sha256:63e5e76b746bb8b891a94968d380736c035e768dab0d56ddad1f50c750a4be39 AS kubectl

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
COPY --from=kubectl --chown=nonroot:nonroot /bin/kubectl /usr/local/bin/kubectl

ENTRYPOINT ["flux-operator"]

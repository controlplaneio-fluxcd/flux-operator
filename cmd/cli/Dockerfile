# Import tools
FROM ghcr.io/fluxcd/flux-cli:v2.8.7@sha256:4bd0394705637ad5dfd63d70893a56c980d2e438def0b8a4579d54fad9be9593 AS flux-cli
FROM ghcr.io/fluxcd/flux-schema:v0.3.0@sha256:92913be498fc0398d8dc2eb749444b81d1f2da3b0e7503177f7a5447ef907a31 AS flux-schema
FROM registry.k8s.io/kubectl:v1.36.1@sha256:d08f476d04d0e30f426f06bc6ff6c38913aaa4591943046b77e2f74a72d3611c AS kubectl

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

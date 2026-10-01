# Import tools
FROM ghcr.io/fluxcd/flux-cli:v2.9.5@sha256:704d5529535570a495249b643159c93a9aa88ffca07813c974a54b7ba6edbd86 AS flux-cli
FROM ghcr.io/fluxcd/flux-schema:v0.14.0@sha256:61af433da855d2407cffd748563d352ecaecd80da8c3a2d94163ae5d8abd966a AS flux-schema
FROM ghcr.io/fluxcd/flux-mirror:v0.9.0@sha256:24b5e45f794767b90d90a2edc4249f67ba9c27bf528f6cfe5853a6cf586329bf AS flux-mirror
FROM registry.k8s.io/kubectl:v1.37.0@sha256:5ed410ebac5dc976cc717098994dcdb29bbbd38f6bd65f582311f5be4ba719cf AS kubectl

# Build the Flux Operator CLI binary using Docker's Debian image.
FROM --platform=${BUILDPLATFORM} golang:1.27 AS builder
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

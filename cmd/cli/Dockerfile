# Build the Flux Operator CLI binary using Docker's Debian image.
FROM --platform=${BUILDPLATFORM} golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG KUBECTL_VER=1.35.1
WORKDIR /workspace

RUN apt-get -y install curl

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

# Download the kubectl binary.
RUN curl -sL https://dl.k8s.io/release/v${KUBECTL_VER}/bin/${TARGETOS}/${TARGETARCH}/kubectl \
    -o /usr/local/bin/kubectl && chmod +x /usr/local/bin/kubectl

# Verify the kubectl binary.
RUN kubectl version --client

# Distribute the binaries using Google's Distroless image.
FROM gcr.io/distroless/static:nonroot

# Copy the license.
COPY LICENSE /licenses/LICENSE

# Copy the binaries.
COPY --from=builder --chmod=777 /usr/local/bin/flux-operator /usr/local/bin/
COPY --from=builder --chmod=777 /usr/local/bin/kubectl /usr/local/bin/

# Run the binaries under a non-root user.
USER 65532:65532
ENTRYPOINT ["flux-operator"]

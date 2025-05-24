# Build the Flux MCP server binary using Docker's Debian image.
FROM --platform=${BUILDPLATFORM} golang:1.24 AS builder
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
COPY cmd/mcp/ cmd/mcp/
COPY api/ api/
COPY internal/ internal/

# Build the server binary.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.VERSION=${VERSION}" -a -o flux-operator-mcp ./cmd/mcp/

# Run the server binary using Google's Distroless image.
FROM gcr.io/distroless/static:nonroot
WORKDIR /

# Copy the license.
COPY LICENSE /licenses/LICENSE

# Copy the server binary.
COPY --from=builder /workspace/flux-operator-mcp .

# Run the server under a non-root user.
USER 65532:65532
ENTRYPOINT ["/flux-operator-mcp"]

# Build the operator binary using the Docker's Debian image.
FROM --platform=${BUILDPLATFORM} golang:1.24 AS builder
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
WORKDIR /workspace

# Copy the Go Modules manifests.
COPY go.mod go.mod
COPY go.sum go.sum

# Cache the Go Modules
RUN go mod download

# Copy the Go sources.
COPY cmd/operator/main.go cmd/operator/main.go
COPY api/ api/
COPY internal/ internal/

# Build the operator binary.
RUN CGO_ENABLED=0 GOFIPS140=latest GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.VERSION=${VERSION}" -trimpath -a -o flux-operator cmd/operator/main.go

# Run the operator binary using Google's Distroless image.
FROM gcr.io/distroless/static:nonroot
WORKDIR /

# Copy the license.
COPY LICENSE /licenses/LICENSE

# Copy the manifests data.
COPY config/data/ /data/

# Copy the operator binary.
COPY --from=builder /workspace/flux-operator .

# Run the operator as a non-root user.
USER 65532:65532
ENTRYPOINT ["/flux-operator"]

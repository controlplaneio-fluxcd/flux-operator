ARG VERSION

FROM --platform=${BUILDPLATFORM} ghcr.io/controlplaneio-fluxcd/flux-operator:${VERSION} AS distroless
FROM --platform=${BUILDPLATFORM} registry.access.redhat.com/ubi8-micro

### Required OpenShift Labels
LABEL name="flux-operator" \
      vendor="ControlPlane" \
      version="${VERSION}" \
      release="1" \
      summary="This is the flux-operator image." \
      description="This image contains the operator of ControlPlane Enterprise for Flux."

WORKDIR /

# Copy the license.
COPY LICENSE /licenses/LICENSE.txt

# Copy the manifests data.
COPY config/data/ /data/

# Copy the operator binary.
COPY --from=distroless flux-operator .

# Run the operator as the nobody user.
USER 65534:65534

ENTRYPOINT ["/flux-operator"]

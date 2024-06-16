#!/usr/bin/env bash

# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

VERSION=$1
ARCH=""
case $(uname -m) in
    x86_64)   ARCH="x86_64" ;;
    aarch64)  ARCH="aarch64" ;;
    arm64)    ARCH="aarch64" ;;
    *)        echo "Unsupported architecture"
              exit 1
              ;;
esac
REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
OCI_IMAGE_PREFIX="ghcr.io/controlplaneio-fluxcd/openshift-flux-operator"
DEST_DIR="${REPOSITORY_ROOT}/bin/olm"

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

if [ ! -d "${DEST_DIR}/${VERSION}" ]; then
  fatal "${DEST_DIR}/${VERSION} does not exist"
fi

# build catalog image
docker build -t ${OCI_IMAGE_PREFIX}-catalog:bundle-${VERSION} \
-f "${DEST_DIR}/test/bundle.Dockerfile" "${DEST_DIR}/${VERSION}"

# push catalog image
docker push ${OCI_IMAGE_PREFIX}-catalog:bundle-${VERSION}

# build opm image
docker build -t opm --build-arg ARCH=$ARCH -f "${DEST_DIR}/test/opm.Dockerfile" "${DEST_DIR}"

# build index image
docker run --rm --privileged \
  -v /var/lib/docker:/var/lib/docker \
  -v /var/run/docker.sock:/var/run/docker.sock \
  opm:latest index add \
  --container-tool docker \
  --bundles ${OCI_IMAGE_PREFIX}-catalog:bundle-${VERSION} \
  --tag ${OCI_IMAGE_PREFIX}-index:v${VERSION}

# push index image
docker push ${OCI_IMAGE_PREFIX}-index:v${VERSION}

info "OLM catalog pushed to ${OCI_IMAGE_PREFIX}-catalog:bundle-${VERSION}"
info "OLM index pushed to ${OCI_IMAGE_PREFIX}-index:v${VERSION}"

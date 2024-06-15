#!/usr/bin/env bash

# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
DEST_DIR="${REPOSITORY_ROOT}/config/data/flux"
IMG_DIR="${REPOSITORY_ROOT}/config/data/flux-images"

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

vendor() {
  info "extracting manifests for Flux ${1}"
  curl -sLO https://github.com/fluxcd/flux2/releases/download/${1}/manifests.tar.gz
  mkdir -p "${DEST_DIR}/${1}"
  tar xzf manifests.tar.gz -C "${DEST_DIR}/${1}"
  rm -rf manifests.tar.gz
}

for var in "$@"
do
    vendor "$var"
done

info "downloading distro repository"
curl -sLO https://github.com/controlplaneio-fluxcd/distribution/archive/refs/heads/main.tar.gz
tar xzf main.tar.gz -C "${DEST_DIR}"

mkdir -p "${IMG_DIR}"
cp -r ${DEST_DIR}/distribution-main/images/* ${IMG_DIR}/
rm -rf ${DEST_DIR}/distribution-main
rm -rf main.tar.gz
info "flux image manifests copied to flux-images"

info "all manifests extracted to ${DEST_DIR}"

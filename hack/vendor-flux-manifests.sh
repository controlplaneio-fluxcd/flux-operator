#!/usr/bin/env bash

# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
DEST_DIR="${REPOSITORY_ROOT}/config/data/flux"

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

info "all manifests extracted to ${DEST_DIR}"

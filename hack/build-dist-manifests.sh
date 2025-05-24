#!/usr/bin/env bash

# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
DEST_DIR="${REPOSITORY_ROOT}/disto"

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

rm -rf ${DEST_DIR}
mkdir -p ${DEST_DIR}/flux-operator
kustomize build config/default > ${DEST_DIR}/flux-operator/install.yaml
info "operator manifests generated to disto/flux-operator"

mkdir -p ${DEST_DIR}/flux-operator-mcp
kustomize build config/mcp > ${DEST_DIR}/flux-operator-mcp/install.yaml
info "MCP server manifests generated to disto/flux-operator-mcp"

mkdir -p ${DEST_DIR}/flux
cp -r config/data/flux/* ${DEST_DIR}/flux/
info "flux manifests copied to disto/flux"

info "downloading distro repository"
curl -sLO https://github.com/controlplaneio-fluxcd/distribution/archive/refs/heads/main.tar.gz
tar xzf main.tar.gz -C "${DEST_DIR}"

mkdir -p "${DEST_DIR}/flux-images"
cp -r ${DEST_DIR}/distribution-main/images/* ${DEST_DIR}/flux-images/
rm -rf ${DEST_DIR}/distribution-main
rm -rf main.tar.gz
info "flux image manifests copied to disto/flux-images"

info "all manifests generated to disto/"
tree -d ${DEST_DIR}

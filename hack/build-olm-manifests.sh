#!/usr/bin/env bash

# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

VERSION=$1
UBI=${2:-false}
REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
SOUCE_DIR="${REPOSITORY_ROOT}/config/olm"
DEST_DIR="${REPOSITORY_ROOT}/bin/olm"

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

rm -rf ${DEST_DIR}
mkdir -p ${DEST_DIR}
cp -r ${SOUCE_DIR}/* ${DEST_DIR}/

export FLUX_OPERATOR_VERSION=${VERSION}
export FLUX_OPERATOR_TS=$(date +"%Y-%m-%dT%H:%M:%S")
export FLUX_OPERATOR_VARIANT=""

if [ "$UBI" = "true" ]; then
  export FLUX_OPERATOR_VARIANT="-ubi"
fi

cat ${SOUCE_DIR}/bundle/manifests/flux-operator.clusterserviceversion.yaml | \
envsubst > ${DEST_DIR}/bundle/manifests/flux-operator.clusterserviceversion.yaml

cat ${SOUCE_DIR}/bundle/manifests/flux-operator.service.yaml > \
${DEST_DIR}/bundle/manifests/flux-operator.service.yaml

cat ${SOUCE_DIR}/test/olm.yaml | \
envsubst > ${DEST_DIR}/test/olm.yaml

cat ${REPOSITORY_ROOT}/config/crd/bases/fluxcd.controlplane.io_fluxinstances.yaml > \
${DEST_DIR}/bundle/manifests/fluxinstances.fluxcd.controlplane.io.crd.yaml

cat ${REPOSITORY_ROOT}/config/crd/bases/fluxcd.controlplane.io_fluxreports.yaml > \
${DEST_DIR}/bundle/manifests/fluxreports.fluxcd.controlplane.io.crd.yaml

cat ${REPOSITORY_ROOT}/config/crd/bases/fluxcd.controlplane.io_resourcesets.yaml > \
${DEST_DIR}/bundle/manifests/resourcesets.fluxcd.controlplane.io.crd.yaml

cat ${REPOSITORY_ROOT}/config/crd/bases/fluxcd.controlplane.io_resourcesetinputproviders.yaml > \
${DEST_DIR}/bundle/manifests/resourcesetinputproviders.fluxcd.controlplane.io.crd.yaml

mv ${DEST_DIR}/bundle ${DEST_DIR}/${VERSION}
info "OperatorHub bundle created in ${DEST_DIR}/${VERSION}"

#!/usr/bin/env bash

# Copyright 2025 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

VERSION=$1
REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
DEST_DIR="${REPOSITORY_ROOT}/bin"

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

OS=$(echo "${RUNNER_OS}" | tr '[:upper:]' '[:lower:]')
if [[ "$OS" == "macos" ]]; then
  OS="darwin"
fi

ARCH=$(echo "${RUNNER_ARCH}" | tr '[:upper:]' '[:lower:]')
if [[ "$ARCH" == "x64" ]]; then
  ARCH="amd64"
elif [[ "$ARCH" == "x86" ]]; then
  ARCH="386"
fi

mkdir -p "${DEST_DIR}"

DOWNLOAD_URL="https://github.com/operator-framework/operator-sdk/releases/download/${VERSION}/operator-sdk_${OS}_${ARCH}"
EXEC_FILE="operator-sdk-${VERSION}"

info "Downloading operator-sdk ${VERSION} for ${OS}/${ARCH}..."
curl -sL "${DOWNLOAD_URL}" -o "${DEST_DIR}/${EXEC_FILE}"
chmod +x "${DEST_DIR}/${EXEC_FILE}"

${DEST_DIR}/${EXEC_FILE} version

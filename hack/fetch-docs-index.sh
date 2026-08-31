#!/usr/bin/env bash

# Copyright 2026 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

# Downloads the fluxoperator.dev docs search index embedded in the MCP server.
# Set FLUX_DOCS_INDEX_URL to download from a different location.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
INDEX_URL="${FLUX_DOCS_INDEX_URL:-https://fluxoperator.dev/mcp/docs-index-main.json}"
DEST="${REPOSITORY_ROOT}/cmd/mcp/toolbox/docindex/index.json"
MAX_SIZE=10485760

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

command -v curl >/dev/null || fatal "curl is required"
command -v jq >/dev/null || fatal "jq is required"

info "Downloading docs index from ${INDEX_URL}"
curl -fsSL --max-filesize "${MAX_SIZE}" "${INDEX_URL}" -o "${DEST}.tmp" || {
    rm -f "${DEST}.tmp"
    fatal "failed to download docs index from ${INDEX_URL}"
}

jq -e '.schemaVersion == 1 and .miniSearch == null and (.docs | length) > 0' "${DEST}.tmp" >/dev/null || {
    rm -f "${DEST}.tmp"
    fatal "downloaded docs index failed validation"
}

mv "${DEST}.tmp" "${DEST}"
info "$(jq -r '"docs=\(.docs | length) chunks=\(.chunks | length) version=\(.version) generatedAt=\(.generatedAt)"' "${DEST}")"

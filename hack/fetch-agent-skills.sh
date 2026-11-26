#!/usr/bin/env bash

# Copyright 2026 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

# Downloads the Flux agent skills embedded in the MCP server from the
# latest fluxcd/agent-skills GitHub release.
# Set AGENT_SKILLS_VERSION to download a specific release tag (e.g. v0.4.0).
# Set GITHUB_TOKEN to authenticate the GitHub API requests.

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
REPO="fluxcd/agent-skills"
VERSION="${AGENT_SKILLS_VERSION:-}"
DEST_DIR="${REPOSITORY_ROOT}/cmd/mcp/skills/data"
VERSION_FILE="${REPOSITORY_ROOT}/cmd/mcp/skills/VERSION"
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
command -v tar >/dev/null || fatal "tar is required"

if command -v sha256sum >/dev/null; then
    SHA256="sha256sum"
elif command -v shasum >/dev/null; then
    SHA256="shasum -a 256"
else
    fatal "sha256sum or shasum is required"
fi

CURL_AUTH=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
    CURL_AUTH=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

api_url="https://api.github.com/repos/${REPO}/releases/latest"
if [[ -n "${VERSION}" ]]; then
    api_url="https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

info "Fetching release metadata from ${api_url}"
curl -fsSL --max-filesize "${MAX_SIZE}" "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" \
    -H "Accept: application/vnd.github+json" "${api_url}" -o "${TMP_DIR}/release.json" ||
    fatal "failed to fetch release metadata from ${api_url}"

TAG=$(jq -r '.tag_name' "${TMP_DIR}/release.json")
[[ "${TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fatal "unexpected release tag '${TAG}'"

asset_url() {
    jq -r --arg name "$1" '.assets[] | select(.name == $name) | .browser_download_url' "${TMP_DIR}/release.json"
}

# Skill names are discovered from the release assets named <skill>-<tag>.tar.gz.
SKILLS=$(jq -r --arg tag "${TAG}" '.assets[].name | select(endswith("-" + $tag + ".tar.gz")) | rtrimstr("-" + $tag + ".tar.gz")' \
    "${TMP_DIR}/release.json" | sort)
[[ -n "${SKILLS}" ]] || fatal "no skill archives found in release ${TAG}"

checksums_url=$(asset_url "checksums.txt")
[[ -n "${checksums_url}" ]] || fatal "checksums.txt not found in release ${TAG}"
curl -fsSL --max-filesize "${MAX_SIZE}" "${checksums_url}" -o "${TMP_DIR}/checksums.txt" ||
    fatal "failed to download ${checksums_url}"

mkdir -p "${TMP_DIR}/extract"
for skill in ${SKILLS}; do
    [[ "${skill}" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]] || fatal "invalid skill name '${skill}'"
    archive="${skill}-${TAG}.tar.gz"
    url=$(asset_url "${archive}")
    info "Downloading ${archive}"
    curl -fsSL --max-filesize "${MAX_SIZE}" "${url}" -o "${TMP_DIR}/${archive}" ||
        fatal "failed to download ${url}"

    expected=$(awk -v name="${archive}" '$2 == name {print $1}' "${TMP_DIR}/checksums.txt")
    [[ -n "${expected}" ]] || fatal "no checksum found for ${archive}"
    actual=$(cd "${TMP_DIR}" && ${SHA256} "${archive}" | awk '{print $1}')
    [[ "${expected}" == "${actual}" ]] || fatal "checksum mismatch for ${archive}: expected ${expected}, got ${actual}"

    # Only regular files and directories under <skill>/ are allowed.
    while IFS= read -r entry; do
        [[ "${entry}" == "${skill}/"* && "${entry}" != *..* ]] || fatal "unexpected path '${entry}' in ${archive}"
    done < <(tar -tzf "${TMP_DIR}/${archive}")
    listing=$(tar -tvzf "${TMP_DIR}/${archive}")
    not_regular=$(printf '%s\n' "${listing}" | grep -v '^[-d]' || true)
    [[ -z "${not_regular}" ]] || fatal "${archive} contains entries that are not regular files or directories"

    tar -xzf "${TMP_DIR}/${archive}" -C "${TMP_DIR}/extract"
    [[ -f "${TMP_DIR}/extract/${skill}/SKILL.md" ]] || fatal "${archive} does not contain ${skill}/SKILL.md"
    # Evals are used for testing the skills and are not served to agents.
    find "${TMP_DIR}/extract/${skill}" -type d -name evals -prune -exec rm -rf {} +
done

rm -rf "${DEST_DIR}"
mv "${TMP_DIR}/extract" "${DEST_DIR}"
echo "${TAG}" >"${VERSION_FILE}"
info "Agent skills ${TAG}: ${SKILLS//$'\n'/ }"

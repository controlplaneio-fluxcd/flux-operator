#!/usr/bin/env bash

# Copyright 2025 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)
LATEST_VERSION=$(gh release view --json tagName -q '.tagName')
NEXT_VERSION="$1"

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

# crane is required to resolve the latest tags and digests for the CLI base images.
command -v crane >/dev/null 2>&1 || \
    fatal "crane not found, install it with: go install github.com/google/go-containerregistry/cmd/crane@latest"

# If the NEXT_VERSION is not provided, increment LATEST_VERSION to the next minor
if [ -z "${NEXT_VERSION:-}" ]; then
    info "No version provided, calculating next version from latest: ${LATEST_VERSION}"
    
    if [[ ${LATEST_VERSION} =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        MAJOR=${BASH_REMATCH[1]}
        MINOR=${BASH_REMATCH[2]}
        NEXT_MINOR=$((MINOR + 1))
        NEXT_VERSION="v${MAJOR}.${NEXT_MINOR}.0"
        info "Calculated next version: ${NEXT_VERSION}"
    else
        fatal "Unable to parse version format from ${LATEST_VERSION}"
    fi
fi

if [ -z "${NEXT_VERSION}" ]; then
    fatal "NEXT_VERSION is required as an argument or will be auto-calculated"
fi

info "Preparing release ${NEXT_VERSION}"

BRANCH_NAME="release-${NEXT_VERSION}"
info "Creating and switching to branch: ${BRANCH_NAME}"
git checkout -b "${BRANCH_NAME}"

info "Updating version in kustomization files"
yq -i '.images[0].newTag = "'"${NEXT_VERSION}"'"' "${REPOSITORY_ROOT}/config/manager/kustomization.yaml"
yq -i '.images[0].newTag = "'"${NEXT_VERSION}"'"' "${REPOSITORY_ROOT}/config/mcp/kustomization.yaml"

# bump_image pins a CLI Dockerfile base image to the latest semver tag and its digest.
# Args: <image repository> <build stage name>
bump_image() {
    local repo="$1"
    local stage="$2"
    local tag digest

    tag=$(crane ls "${repo}" 2>/dev/null | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -1) \
        || fatal "Failed to list tags for ${repo}"
    [ -n "${tag}" ] || fatal "No semver tag found for ${repo}"

    digest=$(crane digest "${repo}:${tag}") || fatal "Failed to get digest for ${repo}:${tag}"

    info "Pinning ${stage} to ${repo}:${tag}@${digest}"
    local newline="FROM ${repo}:${tag}@${digest} AS ${stage}"
    perl -i -pe "s|^FROM .* AS ${stage}\$|${newline//@/\\@}|" "${DOCKERFILE}"
}

DOCKERFILE="${REPOSITORY_ROOT}/cmd/cli/Dockerfile"
info "Bumping base images in cmd/cli/Dockerfile"
bump_image ghcr.io/fluxcd/flux-cli flux-cli
bump_image ghcr.io/fluxcd/flux-schema flux-schema
bump_image ghcr.io/fluxcd/flux-mirror flux-mirror
bump_image registry.k8s.io/kubectl kubectl

info "Committing all config changes"
git add --all "config/" "cmd/cli/Dockerfile"

if [[ -z $(git status --porcelain config/ cmd/cli/Dockerfile) ]]; then
    fatal "No changes to commit. Version may already be up to date."
fi

git commit -s -m "Release ${NEXT_VERSION}"

info "Pushing branch to remote"
git push origin "${BRANCH_NAME}"

info "Creating pull request"
gh pr create --title "Release ${NEXT_VERSION}" --body "Prepare release ${NEXT_VERSION}" --base main --head "${BRANCH_NAME}"

info "Release preparation complete for ${NEXT_VERSION}"

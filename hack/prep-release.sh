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

info "Committing all config changes"
git add --all "config/"

if [[ -z $(git status --porcelain config/) ]]; then
    fatal "No changes to commit. Version may already be up to date."
fi

git commit -s -m "Release ${NEXT_VERSION}"

info "Pushing branch to remote"
git push origin "${BRANCH_NAME}"

info "Creating pull request"
gh pr create --title "Release ${NEXT_VERSION}" --body "Prepare release ${NEXT_VERSION}" --base main --head "${BRANCH_NAME}"

info "Release preparation complete for ${NEXT_VERSION}"

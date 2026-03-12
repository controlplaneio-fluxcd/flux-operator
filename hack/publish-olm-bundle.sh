#!/usr/bin/env bash

# Copyright 2026 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

set -euo pipefail

VERSION="${1:-}"
UPSTREAM="${2:-}"
FORK_OWNER="${3:-stefanprodan}"
REPOSITORY_ROOT=$(git rev-parse --show-toplevel)

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

if [ -z "${VERSION}" ]; then
    fatal "VERSION is required as the first argument (e.g. 0.44.0)"
fi

if [ -z "${UPSTREAM}" ]; then
    fatal "UPSTREAM is required as the second argument (e.g. k8s-operatorhub/community-operators)"
fi

REPO_NAME="${UPSTREAM##*/}"
OLM_DIR="${REPOSITORY_ROOT}/bin/olm/${VERSION}"
BRANCH_NAME="flux-operator-${VERSION}"
TITLE="operator flux-operator (${VERSION})"

if [ ! -d "${OLM_DIR}" ]; then
    fatal "OLM manifests not found in ${OLM_DIR}, run make build-olm-manifests first"
fi

CLONE_DIR=$(mktemp -d)
trap "rm -rf ${CLONE_DIR}" EXIT

info "Syncing fork ${FORK_OWNER}/${REPO_NAME} with ${UPSTREAM}"
gh repo sync "${FORK_OWNER}/${REPO_NAME}"

info "Cloning ${FORK_OWNER}/${REPO_NAME} into ${CLONE_DIR}"
gh repo clone "${FORK_OWNER}/${REPO_NAME}" "${CLONE_DIR}" -- --depth=1

info "Creating branch ${BRANCH_NAME}"
git -C "${CLONE_DIR}" checkout -b "${BRANCH_NAME}"

DEST_DIR="${CLONE_DIR}/operators/flux-operator/${VERSION}"
info "Copying OLM manifests to ${DEST_DIR}"
mkdir -p "${DEST_DIR}"
cp -r "${OLM_DIR}/"* "${DEST_DIR}/"

git -C "${CLONE_DIR}" add operators/flux-operator/

if [[ -z $(git -C "${CLONE_DIR}" status --porcelain operators/flux-operator/) ]]; then
    fatal "No changes to commit. Bundle for ${VERSION} may already exist in ${UPSTREAM}."
fi

info "Committing and pushing"
git -C "${CLONE_DIR}" commit -s -m "${TITLE}"
git -C "${CLONE_DIR}" push origin "${BRANCH_NAME}"

BODY=$(cat <<'EOF'
### Updates to existing Operators

* [x] Did you create a `ci.yaml` file according to the [update instructions](https://github.com/operator-framework/community-operators/blob/master/docs/operator-ci-yaml.md)?
* [x] Is your new CSV pointing to the previous version with the `replaces` property if you chose `replaces-mode` via the `updateGraph` property in `ci.yaml`?
* [x] Is your new CSV referenced in the [appropriate channel](https://github.com/operator-framework/community-operators/blob/master/docs/packaging-operator.md#channels) defined in the `package.yaml` or `annotations.yaml` ?
* [x] Have you tested an update to your Operator when deployed via OLM?
* [x] Is your submission [signed](https://github.com/operator-framework/community-operators/blob/master/docs/contributing-prerequisites.md#sign-your-work)?
EOF
)

info "Creating pull request"
gh pr create \
    --repo "${UPSTREAM}" \
    --head "${FORK_OWNER}:${BRANCH_NAME}" \
    --base main \
    --title "${TITLE}" \
    --body "${BODY}"

info "Done"

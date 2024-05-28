#!/usr/bin/env bash

set -euo pipefail

REPOSITORY_ROOT=$(git rev-parse --show-toplevel)

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

build() {
  info "extracting manifests for Flux ${1}"
  curl -sLO https://github.com/fluxcd/flux2/releases/download/${1}/manifests.tar.gz
  mkdir -p "${REPOSITORY_ROOT}/data/flux/${1}"
  tar xzf manifests.tar.gz -C "${REPOSITORY_ROOT}/data/flux/${1}"
  rm -rf manifests.tar.gz
}

for var in "$@"
do
    build "$var"
done

info "all manifests extracted to ${REPOSITORY_ROOT}/data/flux/"

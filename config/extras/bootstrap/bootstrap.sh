#!/bin/sh
set -e

# Copyright 2026 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0
#
# Bootstrap script for deploying a FluxInstance resource as a one-time setup.
# This script is intended to be run from a Kubernetes Job after the Flux Operator have been installed.
# The FluxInstance should be managed by Flux itself after creation.
#
# Operations performed by this script:
# Creates the FluxInstance if it doesn't exist and waits for it to become ready.
# The waiting period is configurable via the READY_TIMEOUT variable, defaulting to 10 minutes.
# If the FluxInstance CRD is not yet available, the script will retry applying the manifest.
# Adjust MAX_RETRIES and RETRY_DELAY as needed, by default will wait up to 50 seconds.

MAX_RETRIES=5
RETRY_DELAY=10
READY_TIMEOUT="10m"

INSTANCE_NAME="flux"
NAMESPACE="flux-system"
MANIFEST_PATH="/manifests/flux-instance.yaml"

echo "checking if instance '${INSTANCE_NAME}' exists in namespace '${NAMESPACE}'..."

if kubectl get fluxinstance "${INSTANCE_NAME}" -n "${NAMESPACE}" > /dev/null 2>&1; then
  echo "skipping creation instance '${INSTANCE_NAME}' already exists"
  exit 0
fi

for i in $(seq 1 $MAX_RETRIES); do
  echo "applying manifest (attempt $i/$MAX_RETRIES)..."
  if kubectl apply --server-side -f "${MANIFEST_PATH}"; then
    break
  fi
  if [ "$i" -eq "$MAX_RETRIES" ]; then
    echo "failed to apply manifest after $MAX_RETRIES attempts"
    exit 1
  fi
  echo "retrying in ${RETRY_DELAY}s..."
  sleep $RETRY_DELAY
done

echo "waiting for instance '${INSTANCE_NAME}' to become ready..."
kubectl wait fluxinstance "${INSTANCE_NAME}" -n "${NAMESPACE}" \
  --for=condition=Ready \
  --timeout="${READY_TIMEOUT}"

echo "instance '${INSTANCE_NAME}' is ready"

#!/usr/bin/env bash
set -euo pipefail

VERSION=$(ls -d ./config/operatorhub/flux-operator/$1/ | xargs -I{} basename {})
OLM_VERSION=$2
IMG=ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-v"${VERSION}"
OPM=ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-index:latest

# create kind cluster
kind create cluster --name=flux-operator-olm --wait=5m

# load images
kind load docker-image --name=flux-operator-olm ${IMG}
kind load docker-image --name=flux-operator-olm ${OPM}

# install OLM
operator-sdk olm install --version ${OLM_VERSION}

# create olm namespace, catalogsource and subscription
ls -d ./config/operatorhub/flux-operator/test-yamls/* | xargs -I{} kubectl apply -f {}
sleep 60

# wait for installplan to be installed
INSTALL_PLAN=$(kubectl get installplan -n flux-system -oyaml | yq e .items[].metadata.name -)
kubectl wait --for=condition=Installed=true installplan/$INSTALL_PLAN -n flux-system 

kubectl get installplan -A
kubectl get csv -A
sleep 60

# wait for flux-operator to be ready
kubectl wait --for=condition=Ready=true pod -lapp.kubernetes.io/name=flux-operator -n flux-system

kubectl get pods -n flux-system

# scorecard test
operator-sdk scorecard ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-v"${VERSION}" \
-c ./config/operatorhub/flux-operator/${VERSION}/tests/scorecard/config.yaml -w 60s -o json

#!/usr/bin/env bash
set -euo pipefail

kind create cluster --name=flux-operator-olm

operator-sdk olm install --version 0.28.0

ls -d ./config/operatorhub/flux-operator/test-yamls/* | xargs -I{} kubectl apply -f {}


sleep 120


INSTALL_PLAN=$(kubectl get installplan -n flux-system -oyaml | yq e .items[].metadata.name -)

kubectl wait --for=condition=Installed=true installplan/$INSTALL_PLAN -n flux-system 

kubectl get installplan -A
kubectl get csv -A

sleep 60

kubectl wait --for=condition=Ready=true pod -lapp=flux-operator -n flux-system

kubectl get pods -n flux-system
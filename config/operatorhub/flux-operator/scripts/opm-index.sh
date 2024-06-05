#!/usr/bin/env bash
set -euo pipefail

#
# Prepare catalog for e2e testing
#

VERSION=$1
ARCH=""
case $(uname -m) in
    x86_64)   ARCH="x86_64" ;;
    aarch64)  ARCH="aarch64" ;;
    arm64)    ARCH="aarch64" ;;
    *)        echo "Unsupported architecture"
              exit 1
              ;;
esac

list=""
for i in $(ls -d ./config/operatorhub/flux-operator/${VERSION}/ | xargs -I{} basename {}); do
  # docker build and push individual bundles
  docker build -t ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-v"${i}" \
  -f ./config/operatorhub/flux-operator/bundle.Dockerfile config/operatorhub/flux-operator/"${i}"
  list="$list,ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-v$i"
done

docker build -t opm --build-arg ARCH=$ARCH -f ./config/operatorhub/flux-operator/Dockerfile.opm .

list=${list:1} # remove first comma
docker run --rm -it \
  --privileged \
  -v /var/lib/docker:/var/lib/docker \
  -v /var/run/docker.sock:/var/run/docker.sock \
  opm:latest index add \
  --container-tool docker \
  --bundles "$list" \
  --tag ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-index:latest

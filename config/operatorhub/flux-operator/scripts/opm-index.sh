#!/usr/bin/env bash
set -euo pipefail

#
# Prepare catalog for e2e testing
#

VERSION=$1
ARCH=""
DIR="config/operatorhub/flux-operator/"
case $(uname -m) in
    x86_64)   ARCH="x86_64" ;;
    aarch64)  ARCH="aarch64" ;;
    arm64)    ARCH="aarch64" ;;
    *)        echo "Unsupported architecture"
              exit 1
              ;;
esac

if [ ! -d "${DIR}/${VERSION}" ]; then
  echo "Version ${VERSION} does not exist"
  exit 1
fi

# docker build and push individual bundles
docker build -t ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-"${VERSION}" \
-f "${DIR}/bundle.Dockerfile" "${DIR}/${VERSION}"
docker push ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-"${VERSION}"

docker build -t opm --build-arg ARCH=$ARCH -f "${DIR}/Dockerfile.opm" .

docker run --rm -it \
  --privileged \
  -v /var/lib/docker:/var/lib/docker \
  -v /var/run/docker.sock:/var/run/docker.sock \
  opm:latest index add \
  --container-tool docker \
  --bundles ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-catalog:bundle-"${VERSION}" \
  --tag ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-index:${VERSION}

#push index
docker push ghcr.io/controlplaneio-fluxcd/openshift-flux-operator-index:${VERSION}
 
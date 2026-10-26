#!/usr/bin/env bash
# End-to-end test for inter-controller SPIFFE mTLS with flux-operator.
set -euo pipefail

FLUX_NAMESPACE="${FLUX_NAMESPACE:-flux-system}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
MANIFESTS_DIR="${ROOT_DIR}/test/e2e/spiffe-mtls/manifests"
IMG="${IMG:-test/flux-operator:e2e}"
ENVOY_GATEWAY_VERSION="${ENVOY_GATEWAY_VERSION:-v1.9.2}"
SPIRE_CRDS_CHART_VERSION="${SPIRE_CRDS_CHART_VERSION:-0.6.1}"
SPIRE_CHART_VERSION="${SPIRE_CHART_VERSION:-0.30.3}"
SPIFFE_DOMAIN="example.org"
ENVOY_ID="spiffe://${SPIFFE_DOMAIN}/ns/envoy-gateway-system/sa/envoy-gateway-proxy"
STANDALONE_ID="spiffe://${SPIFFE_DOMAIN}/ns/${FLUX_NAMESPACE}/sa/standalone-web"
PODINFO_ID="spiffe://${SPIFFE_DOMAIN}/ns/${FLUX_NAMESPACE}/sa/podinfo-spiffe"
SOURCE_WATCHER_ID="spiffe://${SPIFFE_DOMAIN}/ns/${FLUX_NAMESPACE}/sa/source-watcher"
OPERATOR_ID="spiffe://${SPIFFE_DOMAIN}/ns/${FLUX_NAMESPACE}/sa/flux-operator"

log()  { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
fail() { printf '\033[1;31mFAIL: %s\033[0m\n' "$*" >&2; exit 1; }

# events prints the Flux events received by the event sink, one JSON object per line.
events() {
    kubectl -n "${FLUX_NAMESPACE}" logs deploy/event-sink --tail=-1 2>/dev/null \
        | jq -cR 'fromjson? | select(.msg == "payload received") | .payload' 2>/dev/null || true
}

# wait_event <jq condition> <failure message>
wait_event() {
    for _ in $(seq 1 60); do
        events | jq -e "select($1)" >/dev/null && return 0
        sleep 2
    done
    fail "$2"
}

# podinfo_requests prints the requests logged by podinfo-spiffe, one JSON object per line.
podinfo_requests() {
    kubectl -n "${FLUX_NAMESPACE}" logs deploy/podinfo-spiffe --tail=-1 2>/dev/null \
        | jq -cR 'fromjson? | select(.msg == "request started")' 2>/dev/null || true
}

# wait_podinfo_artifact_request <client SPIFFE ID> <failure message>
wait_podinfo_artifact_request() {
    for _ in $(seq 1 30); do
        podinfo_requests | jq -e --arg id "$1" \
            'select(.uri == "/artifact" and .spiffe_id == $id)' >/dev/null && return 0
        sleep 2
    done
    fail "$2"
}

# wait_podinfo_handshake_error <message substring> <failure message>
wait_podinfo_handshake_error() {
    for _ in $(seq 1 30); do
        kubectl -n "${FLUX_NAMESPACE}" logs deploy/podinfo-spiffe --tail=-1 2>/dev/null \
            | jq -eR --arg s "$1" 'fromjson? | select(.msg | startswith("http: TLS handshake error") and contains($s))' \
            >/dev/null && return 0
        sleep 2
    done
    fail "$2"
}

# expect_tls_rejection <url> <server name> fails unless a TLS client without
# an SVID is rejected. Retries until the port-forward accepts connections.
expect_tls_rejection() {
    local rc
    for _ in $(seq 1 30); do
        rc=0
        curl -k -sS --max-time 5 "$1" -o /dev/null 2>/dev/null || rc=$?
        case "${rc}" in
            0) fail "$2 accepted a TLS client without an SVID" ;;
            7|28) sleep 1 ;;
            *) return 0 ;;
        esac
    done
    fail "$2 was not reachable for the TLS client check"
}

# wait_spiffeid <ClusterSPIFFEID name> waits for the entry to select at least one pod.
wait_spiffeid() {
    local selected
    for _ in $(seq 1 60); do
        selected=$(kubectl get clusterspiffeid "$1" -o jsonpath='{.status.stats.podsSelected}' 2>/dev/null || true)
        [[ "${selected:-0}" -gt 0 ]] && return 0
        sleep 2
    done
    fail "ClusterSPIFFEID $1 selected no pods"
}

# artifact_digest reads HTTP response headers from stdin and prints X-Artifact-Digest.
artifact_digest() {
    tr -d '\r' | awk -F': ' 'tolower($1)=="x-artifact-digest" {print $2}'
}

# static_external_artifact <name> <url> <digest> creates an ExternalArtifact with a Ready status.
static_external_artifact() {
    local last
    last="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    kubectl apply -f - >/dev/null <<EOF
apiVersion: source.toolkit.fluxcd.io/v1
kind: ExternalArtifact
metadata:
  name: $1
  namespace: ${FLUX_NAMESPACE}
spec: {}
EOF
    kubectl -n "${FLUX_NAMESPACE}" patch "externalartifact/$1" \
        --subresource=status --type=merge -p "$(jq -n \
            --arg url "$2" \
            --arg digest "$3" \
            --arg revision "spiffe-e2e" \
            --arg last "${last}" \
            '{status:{artifact:{url:$url,digest:$digest,revision:$revision,path:"artifact.tar.gz",lastUpdateTime:$last},conditions:[{type:"Ready",status:"True",reason:"Succeeded",message:"Static artifact for SPIFFE e2e",lastTransitionTime:$last}]}}')" >/dev/null
}

cleanup() {
    local code=$?
    if [[ $code -ne 0 ]]; then
        log "FAILURE (exit ${code}) — dumping diagnostics"
        kubectl -n "${FLUX_NAMESPACE}" get pods -o wide || true
        kubectl -n "${FLUX_NAMESPACE}" get fluxinstance/flux -o yaml || true
        kubectl -n "${FLUX_NAMESPACE}" get events --sort-by=.lastTimestamp | tail -50 || true
        for dep in flux-operator source-controller source-watcher kustomize-controller helm-controller notification-controller image-reflector-controller image-automation-controller; do
            echo "--- ${dep} ---"
            kubectl -n "${FLUX_NAMESPACE}" logs "deploy/${dep}" --tail=80 || true
        done
        kubectl -n "${FLUX_NAMESPACE}" logs deploy/event-sink --tail=80 || true
        kubectl -n "${FLUX_NAMESPACE}" logs deploy/artifact-server --tail=80 || true
        kubectl -n "${FLUX_NAMESPACE}" logs deploy/podinfo-spiffe --tail=80 || true
        kubectl get clusterspiffeids -o yaml || true
        kubectl -n "${FLUX_NAMESPACE}" logs deploy/standalone-web --tail=80 || true
        kubectl -n "${FLUX_NAMESPACE}" get gateway/receiver -o yaml || true
        kubectl -n "${FLUX_NAMESPACE}" get httproute,backendtlspolicy -o yaml || true
        kubectl -n envoy-gateway-system get pods -o wide || true
        kubectl -n envoy-gateway-system logs deploy/envoy-gateway --tail=80 || true
        kubectl -n envoy-gateway-system logs -l gateway.envoyproxy.io/owning-gateway-name=receiver --tail=80 || true
    fi
    if [[ -n "${PORT_FORWARD_PID:-}" ]]; then
        kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    fi
    if [[ -n "${WEB_PORT_FORWARD_PID:-}" ]]; then
        kill "${WEB_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    fi
    if [[ -n "${STANDALONE_PORT_FORWARD_PID:-}" ]]; then
        kill "${STANDALONE_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    fi
    if [[ -n "${PODINFO_PORT_FORWARD_PID:-}" ]]; then
        kill "${PODINFO_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
    fi
    exit "$code"
}
trap cleanup EXIT

for bin in kubectl helm kind docker jq curl tar; do
    command -v "${bin}" >/dev/null 2>&1 || fail "required tool '${bin}' not found"
done

log "Installing SPIRE"
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened >/dev/null 2>&1 || true
helm repo update spiffe >/dev/null
helm upgrade --install spire-crds spiffe/spire-crds \
    --version "${SPIRE_CRDS_CHART_VERSION}" \
    --namespace spire-crds \
    --create-namespace \
    --wait --timeout 5m
# The SPIRE chart creates its own namespaces. Pre-create them and keep the
# chart's namespace creation disabled to avoid a conflict with Helm 4's
# --create-namespace behavior.
kubectl create namespace spire-system --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl create namespace spire-server --dry-run=client -o yaml | kubectl apply -f - >/dev/null
helm upgrade --install spire spiffe/spire \
    --version "${SPIRE_CHART_VERSION}" \
    --namespace spire-system \
    --set global.spire.trustDomain="${SPIFFE_DOMAIN}" \
    --set global.spire.namespaces.create=false \
    --set spire-server.controllerManager.enabled=true \
    --set spire-server.controllerManager.identities.clusterSPIFFEIDs.default.enabled=false \
    --set spire-agent.sds.enabled=true \
    --set spiffe-csi-driver.enabled=true \
    --set spiffe-oidc-discovery-provider.enabled=false \
    --wait --timeout 10m
kubectl -n spire-system wait --for=condition=Ready pods --all --timeout=3m
kubectl apply -f "${MANIFESTS_DIR}/spire.yaml"

log "Building and deploying flux-operator"
make -C "${ROOT_DIR}" docker-build IMG="${IMG}" >/dev/null
kind load docker-image "${IMG}" >/dev/null
make -C "${ROOT_DIR}" deploy IMG="${IMG}" >/dev/null
kubectl -n "${FLUX_NAMESPACE}" rollout status deploy/flux-operator --timeout=3m

log "Configuring the embedded Web UI for SPIFFE mTLS"
kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: flux-operator-web-config
  namespace: ${FLUX_NAMESPACE}
stringData:
  config.yaml: |
    apiVersion: web.fluxcd.controlplane.io/v1
    kind: Config
    spec:
      authentication:
        type: Anonymous
        anonymous:
          username: flux-e2e
          groups:
            - flux-admin
      userActions:
        audit:
          - download
EOF
kubectl create clusterrolebinding flux-e2e-web-admin \
    --clusterrole=flux-web-admin \
    --group=flux-admin \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null

log "Configuring the operator pod for the Workload API"
kubectl -n "${FLUX_NAMESPACE}" patch deploy/flux-operator --type=strategic -p "$(cat <<EOF
spec:
  template:
    spec:
      containers:
        - name: manager
          args:
            - --authorized-web-ui-client-spiffe-id=${ENVOY_ID}
            - --web-config-secret-name=flux-operator-web-config
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: unix:///spiffe-workload-api/spire-agent.sock
          volumeMounts:
            - name: spiffe-workload-api
              mountPath: /spiffe-workload-api
              readOnly: true
      volumes:
        - name: spiffe-workload-api
          csi:
            driver: csi.spiffe.io
            readOnly: true
EOF
)" >/dev/null
kubectl -n "${FLUX_NAMESPACE}" rollout status deploy/flux-operator --timeout=3m

log "Deploying the standalone Web UI"
kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: standalone-web
  namespace: ${FLUX_NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: standalone-web-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: standalone-web
    namespace: ${FLUX_NAMESPACE}
---
apiVersion: v1
kind: Secret
metadata:
  name: standalone-web-config
  namespace: ${FLUX_NAMESPACE}
stringData:
  config.yaml: |
    apiVersion: web.fluxcd.controlplane.io/v1
    kind: Config
    spec:
      authentication:
        type: Anonymous
        anonymous:
          username: standalone-e2e
          groups:
            - flux-admin
      userActions:
        audit:
          - download
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: standalone-web
  namespace: ${FLUX_NAMESPACE}
  labels:
    app.kubernetes.io/name: standalone-web
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: standalone-web
  template:
    metadata:
      labels:
        app.kubernetes.io/name: standalone-web
    spec:
      serviceAccountName: standalone-web
      containers:
        - name: web
          image: ${IMG}
          imagePullPolicy: IfNotPresent
          args:
            - --web-server-only
            - --web-config-secret-name=standalone-web-config
            - --authorized-web-ui-client-spiffe-id=${ENVOY_ID}
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: unix:///spiffe-workload-api/spire-agent.sock
          volumeMounts:
            - name: spiffe-workload-api
              mountPath: /spiffe-workload-api
              readOnly: true
          ports:
            - containerPort: 9080
              name: http-web
      volumes:
        - name: spiffe-workload-api
          csi:
            driver: csi.spiffe.io
            readOnly: true
---
apiVersion: v1
kind: Service
metadata:
  name: standalone-web
  namespace: ${FLUX_NAMESPACE}
spec:
  selector:
    app.kubernetes.io/name: standalone-web
  ports:
    - name: http-web
      port: 9080
      targetPort: 9080
EOF
kubectl -n "${FLUX_NAMESPACE}" rollout status deploy/standalone-web --timeout=3m

log "Creating the FluxInstance with SPIFFE mTLS"
kubectl apply -f - <<EOF
apiVersion: fluxcd.controlplane.io/v1
kind: FluxInstance
metadata:
  name: flux
  namespace: ${FLUX_NAMESPACE}
spec:
  distribution:
    version: "2.10.x"
    registry: "ghcr.io/fluxcd"
  components:
    - source-controller
    - kustomize-controller
    - helm-controller
    - notification-controller
    - image-reflector-controller
    - image-automation-controller
    - source-watcher
  cluster:
    domain: cluster.local
    networkPolicy: true
  spiffe:
    workloadAPI:
      endpoint: unix:///spiffe-workload-api/spire-agent.sock
      csiDriver: csi.spiffe.io
    mtls:
      enabled: true
      componentIDTemplate: "spiffe://${SPIFFE_DOMAIN}/ns/${FLUX_NAMESPACE}/sa/{component}"
      receivers:
        - "${ENVOY_ID}"
      extraArtifactClients:
        - "${STANDALONE_ID}"
      extraArtifactServers:
        - "${ENVOY_ID}"
        - "${PODINFO_ID}"
      extraNotifiers:
        - "${STANDALONE_ID}"
EOF

kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready fluxinstance/flux --timeout=3m

log "Installing Envoy Gateway"
kubectl apply --server-side -f \
    "https://github.com/envoyproxy/gateway/releases/download/${ENVOY_GATEWAY_VERSION}/install.yaml" >/dev/null
kubectl -n envoy-gateway-system rollout status deployment/envoy-gateway --timeout=10m
kubectl apply -f "${MANIFESTS_DIR}/envoy-gateway-config.yaml" >/dev/null
kubectl -n envoy-gateway-system rollout restart deployment/envoy-gateway >/dev/null
kubectl -n envoy-gateway-system rollout status deployment/envoy-gateway --timeout=10m
kubectl apply -f "${MANIFESTS_DIR}/envoy-gateway.yaml" >/dev/null
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Accepted gateway/receiver --timeout=3m

log "Deploying the event sink and podinfo"
kubectl apply -f "${MANIFESTS_DIR}/event-sink.yaml"
kubectl apply -f "${MANIFESTS_DIR}/podinfo-spiffe.yaml"
kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/event-sink --timeout=2m
kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/artifact-server --timeout=2m
kubectl -n "${FLUX_NAMESPACE}" rollout status deployment/podinfo-spiffe --timeout=3m

log "Verifying the ClusterSPIFFEIDs select pods"
for id in flux-controllers flux-operator standalone-web podinfo-spiffe; do
    wait_spiffeid "${id}"
done
info "ClusterSPIFFEIDs selected pods"

log "Running inter-controller traffic tests"
kubectl apply -f "${MANIFESTS_DIR}/test-resources.yaml"

kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready gitrepository/podinfo --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready kustomization/podinfo --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready helmrepository/podinfo --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready helmrelease/podinfo --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready artifactgenerator/podinfo --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready externalartifact/podinfo-split --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready externalartifact/podinfo-chart-split --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready artifactgenerator/podinfo-chain --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready externalartifact/podinfo-chained --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready kustomization/podinfo-external --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready helmrelease/podinfo-external --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready imagerepository/podinfo --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready receiver/spiffe-e2e --timeout=3m

# Apply this after the Alert is established so its first reconciliation event
# is forwarded to the event sink over the operator's SPIFFE notifier.
kubectl apply -f - >/dev/null <<EOF
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: spiffe-e2e
  namespace: ${FLUX_NAMESPACE}
spec:
  type: Static
  defaultValues:
    environment: spiffe-e2e
EOF
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready resourcesetinputprovider/spiffe-e2e --timeout=3m

WEBHOOK_PATH=$(kubectl -n "${FLUX_NAMESPACE}" get receiver/spiffe-e2e -o jsonpath='{.status.webhookPath}')
[[ -n "${WEBHOOK_PATH}" ]] || fail "receiver webhook path is empty"

log "Calling the receiver through Envoy Gateway"
for _ in $(seq 1 60); do
    kubectl -n envoy-gateway-system get deployment \
        -l gateway.envoyproxy.io/owning-gateway-name=receiver -o name 2>/dev/null | grep -q . && break
    sleep 2
done
kubectl -n envoy-gateway-system rollout status deployment \
    -l gateway.envoyproxy.io/owning-gateway-name=receiver --timeout=10m
wait_spiffeid envoy-gateway-proxy

ENVOY_SERVICE=$(kubectl -n envoy-gateway-system get service \
    -l gateway.envoyproxy.io/owning-gateway-name=receiver -o jsonpath='{.items[0].metadata.name}')
[[ -n "${ENVOY_SERVICE}" ]] || fail "Envoy Gateway service not found"
for _ in $(seq 1 60); do
    if kubectl -n envoy-gateway-system get service "${ENVOY_SERVICE}" \
        -o jsonpath='{.spec.ports[*].port}' | grep -qw 443; then
        break
    fi
    sleep 2
done
kubectl -n envoy-gateway-system get service "${ENVOY_SERVICE}" \
    -o jsonpath='{.spec.ports[*].port}' | grep -qw 443 \
    || fail "Envoy HTTPS listener is not exposed"

kubectl -n envoy-gateway-system port-forward "service/${ENVOY_SERVICE}" 8080:80 >/dev/null 2>&1 &
PORT_FORWARD_PID=$!
for _ in $(seq 1 30); do
    curl -s -o /dev/null "http://127.0.0.1:8080/" && break
    sleep 1
done
curl -fsS -X POST "http://127.0.0.1:8080${WEBHOOK_PATH}" \
    -H 'Content-Type: application/json' -d '{}' >/dev/null

WEBHOOK_DIGEST="${WEBHOOK_PATH##*/}"
for _ in $(seq 1 30); do
    webhook_logs=$(kubectl -n "${FLUX_NAMESPACE}" logs deploy/notification-controller --tail=-1 2>/dev/null || true)
    grep -q "handling request: ${WEBHOOK_DIGEST}" <<<"${webhook_logs}" && break
    sleep 1
done
grep -q "handling request: ${WEBHOOK_DIGEST}" <<<"${webhook_logs}" \
    || fail "notification-controller did not handle the receiver request"
info "receiver call through Envoy Gateway succeeded"

log "Downloading an artifact through the embedded Web UI over SPIFFE mTLS"
WEB_ARTIFACT="$(mktemp)"
curl -fsS -o "${WEB_ARTIFACT}" \
    "http://127.0.0.1:8080/api/v1/artifact/download?kind=GitRepository&namespace=${FLUX_NAMESPACE}&name=podinfo"
tar -tzf "${WEB_ARTIFACT}" >/dev/null
rm -f "${WEB_ARTIFACT}"
info "artifact download through the Web UI succeeded"

log "Verifying the embedded Web UI rejects clients without an SVID"
kubectl -n "${FLUX_NAMESPACE}" port-forward svc/flux-operator 9443:9080 >/dev/null 2>&1 &
WEB_PORT_FORWARD_PID=$!
expect_tls_rejection "https://127.0.0.1:9443/" "embedded Web UI"
kill "${WEB_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
wait "${WEB_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
WEB_PORT_FORWARD_PID=
info "web UI rejected a TLS client without an SVID"

wait_event '.reportingController == "flux-operator" and .involvedObject.kind == "GitRepository" and .involvedObject.name == "podinfo" and .reason == "WebAction"' \
    "event sink did not receive the Web UI download audit event"

log "Downloading an artifact through the standalone Web UI over SPIFFE mTLS"
STANDALONE_ARTIFACT="$(mktemp)"
curl -fsS -o "${STANDALONE_ARTIFACT}" \
    "http://127.0.0.1:8080/standalone/api/v1/artifact/download?kind=ExternalArtifact&namespace=${FLUX_NAMESPACE}&name=podinfo-split"
tar -tzf "${STANDALONE_ARTIFACT}" >/dev/null
rm -f "${STANDALONE_ARTIFACT}"
info "artifact download through the standalone Web UI succeeded"

log "Verifying the standalone Web UI rejects clients without an SVID"
kubectl -n "${FLUX_NAMESPACE}" port-forward svc/standalone-web 9444:9080 >/dev/null 2>&1 &
STANDALONE_PORT_FORWARD_PID=$!
expect_tls_rejection "https://127.0.0.1:9444/" "standalone Web UI"
kill "${STANDALONE_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
wait "${STANDALONE_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
STANDALONE_PORT_FORWARD_PID=
info "standalone Web UI rejected a TLS client without an SVID"

wait_event '.reportingController == "flux-operator" and .involvedObject.kind == "ExternalArtifact" and .involvedObject.name == "podinfo-split" and .reason == "WebAction"' \
    "event sink did not receive the standalone Web UI audit event"

log "Testing extraArtifactServers through Envoy Gateway"
ENVOY_ARTIFACT_DIGEST=$(curl -fsSI "http://127.0.0.1:8080/artifact" | artifact_digest)
[[ "${ENVOY_ARTIFACT_DIGEST}" == sha256:* ]] || fail "artifact server returned no digest"
ENVOY_ARTIFACT_URL="https://${ENVOY_SERVICE}.envoy-gateway-system.svc.cluster.local./artifact"
static_external_artifact podinfo-via-envoy "${ENVOY_ARTIFACT_URL}" "${ENVOY_ARTIFACT_DIGEST}"

kubectl apply -f - >/dev/null <<EOF
apiVersion: source.extensions.fluxcd.io/v1beta1
kind: ArtifactGenerator
metadata:
  name: podinfo-via-envoy
  namespace: ${FLUX_NAMESPACE}
spec:
  sources:
    - alias: split
      kind: ExternalArtifact
      name: podinfo-via-envoy
  artifacts:
    - name: podinfo-via-envoy-split
      copy:
        - from: "@split/**"
          to: "@artifact/"
EOF
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready artifactgenerator/podinfo-via-envoy --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready externalartifact/podinfo-via-envoy-split --timeout=3m
info "artifact fetch through Envoy Gateway succeeded"

log "Testing extraArtifactServers with a direct SPIFFE mTLS server"
PODINFO_ARTIFACT_DIGEST=$(kubectl -n "${FLUX_NAMESPACE}" exec deploy/podinfo-spiffe -- \
    curl -fsSI http://localhost:9898/artifact | artifact_digest)
[[ "${PODINFO_ARTIFACT_DIGEST}" == sha256:* ]] || fail "podinfo returned no artifact digest"
static_external_artifact podinfo-direct \
    "https://podinfo-spiffe.${FLUX_NAMESPACE}.svc.cluster.local:9899/artifact" \
    "${PODINFO_ARTIFACT_DIGEST}"

kubectl apply -f - >/dev/null <<EOF
apiVersion: source.extensions.fluxcd.io/v1beta1
kind: ArtifactGenerator
metadata:
  name: podinfo-direct
  namespace: ${FLUX_NAMESPACE}
spec:
  sources:
    - alias: direct
      kind: ExternalArtifact
      name: podinfo-direct
  artifacts:
    - name: podinfo-direct-split
      copy:
        - from: "@direct/**"
          to: "@artifact/"
EOF
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready artifactgenerator/podinfo-direct --timeout=3m
kubectl -n "${FLUX_NAMESPACE}" wait --for=condition=Ready externalartifact/podinfo-direct-split --timeout=3m
wait_podinfo_artifact_request "${SOURCE_WATCHER_ID}" \
    "podinfo did not serve the artifact to source-watcher over mTLS"
info "artifact fetch from podinfo over SPIFFE mTLS succeeded"

log "Downloading the direct artifact through the embedded Web UI"
DIRECT_ARTIFACT="$(mktemp)"
curl -fsS -o "${DIRECT_ARTIFACT}" \
    "http://127.0.0.1:8080/api/v1/artifact/download?kind=ExternalArtifact&namespace=${FLUX_NAMESPACE}&name=podinfo-direct"
tar -tzf "${DIRECT_ARTIFACT}" >/dev/null
rm -f "${DIRECT_ARTIFACT}"
wait_podinfo_artifact_request "${OPERATOR_ID}" \
    "podinfo did not serve the artifact to flux-operator over mTLS"
info "direct artifact download through the embedded Web UI succeeded"

log "Verifying podinfo rejects SVIDs that are not authorized"
STANDALONE_CODE=$(curl -s -o /dev/null -w '%{http_code}' \
    "http://127.0.0.1:8080/standalone/api/v1/artifact/download?kind=ExternalArtifact&namespace=${FLUX_NAMESPACE}&name=podinfo-direct" || true)
if [[ "${STANDALONE_CODE}" == 2* ]]; then
    fail "standalone Web UI downloaded the artifact from podinfo with an unauthorized SVID"
fi
if podinfo_requests | jq -e --arg id "${STANDALONE_ID}" 'select(.spiffe_id == $id)' >/dev/null; then
    fail "podinfo served a request from the standalone Web UI"
fi
wait_podinfo_handshake_error "unexpected ID \"${STANDALONE_ID}\"" \
    "podinfo did not reject the standalone Web UI SVID"
info "podinfo rejected the standalone Web UI SVID (download returned ${STANDALONE_CODE})"

log "Verifying podinfo rejects clients without an SVID"
kubectl -n "${FLUX_NAMESPACE}" port-forward svc/podinfo-spiffe 9445:9899 >/dev/null 2>&1 &
PODINFO_PORT_FORWARD_PID=$!
expect_tls_rejection "https://127.0.0.1:9445/artifact" "podinfo"
wait_podinfo_handshake_error "client didn't provide a certificate" \
    "podinfo did not reject the TLS client without an SVID"
kill "${PODINFO_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
wait "${PODINFO_PORT_FORWARD_PID}" >/dev/null 2>&1 || true
PODINFO_PORT_FORWARD_PID=
info "podinfo rejected a TLS client without an SVID"

log "Waiting for an event from every controller"
expected_controllers=(
    source-controller
    source-watcher
    kustomize-controller
    helm-controller
    image-reflector-controller
    image-automation-controller
)
missing=()
for _ in $(seq 1 60); do
    reporters=$(events | jq -r '.reportingController' | sort -u)
    missing=()
    for controller in "${expected_controllers[@]}"; do
        grep -qx "${controller}" <<<"${reporters}" || missing+=("${controller}")
    done
    if [[ ${#missing[@]} -eq 0 ]]; then
        info "event sink received events from all controllers"
        break
    fi
    sleep 2
done
[[ ${#missing[@]} -eq 0 ]] || fail "event sink is missing events from: ${missing[*]}"

log "Waiting for the ResourceSetInputProvider notifier event"
wait_event '.reportingController == "flux-operator" and .involvedObject.kind == "ResourceSetInputProvider" and .involvedObject.name == "spiffe-e2e" and .reason == "ReconciliationSucceeded"' \
    "event sink did not receive the ResourceSetInputProvider notifier event"

log "SUCCESS: all inter-controller SPIFFE mTLS checks passed"

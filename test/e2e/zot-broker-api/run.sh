#!/usr/bin/env bash
# End-to-end test for OCI registry authentication with Kubernetes ServiceAccount
# tokens, using Zot's OIDC bearer authentication.
#
# It creates a dedicated kind cluster with a ServiceAccount issuer reachable
# from a Zot container on the kind network, deploys flux-operator with the
# SPIFFE Broker API, and verifies that a ResourceSetInputProvider of type
# OCIArtifactTag authenticates to Zot with a projected ServiceAccount token
# and with a JWT-SVID obtained through the SPIFFE Broker API.
set -euo pipefail

IMG="${IMG:-test/flux-operator:e2e}"
ZOT_IMAGE="${ZOT_IMAGE:-ghcr.io/project-zot/zot-linux-amd64:v2.1.22}"
CLUSTER="${CLUSTER:-zot-oidc}"
ZOT_CONTAINER="${ZOT_CONTAINER:-zot-oidc}"
ZOT_PORT="${ZOT_PORT:-5000}"
ZOT_HOST_PORT="${ZOT_HOST_PORT:-5050}"
AUDIENCE="zot-registry"
NS="flux-system"
APP_NS="zot-e2e"
SA_NAME="zot-puller"
SA_NAME_B="zot-puller-b"
WRONG_AUD_SA_NAME="zot-puller-wrong-aud"
REPO_A="zot-e2e-a"
REPO_B="zot-e2e-b"
RSIP_A="zot-oidc-a"
RSIP_B="zot-oidc-b"
REV="test"

# SPIFFE Broker API test setup.
SPIRE_NAMESPACE="spire"
SPIRE_TRUST_DOMAIN="example.org"
SPIRE_SERVER_IMAGE="${SPIRE_SERVER_IMAGE:-ghcr.io/spiffe/spire-server:1.15.5}"
SPIRE_AGENT_IMAGE="${SPIRE_AGENT_IMAGE:-ghcr.io/spiffe/spire-agent:1.15.5}"
SPIRE_OIDC_IMAGE="${SPIRE_OIDC_IMAGE:-ghcr.io/spiffe/oidc-discovery-provider:1.15.5}"
BROKER_SA="flux-operator"
SPIRE_BROKER_ENDPOINT="dns:///spire-broker.spire:8443"
SPIRE_BROKER_ID="spiffe://${SPIRE_TRUST_DOMAIN}/spire/agent/k8s_psat/broker-cluster/"
SPIRE_OIDC_ISSUER="http://${CLUSTER}-control-plane:8082"
JWT_SA_NAME="zot-puller-jwt"
JWT_SA_NAME_B="zot-puller-jwt-b"
JWT_OBJ_TYPE="resourcesetinputproviders.fluxcd.controlplane.io"
JWT_RSIP_A="zot-jwt-a"
JWT_RSIP_B="zot-jwt-b"
JWT_RSIP_A_CROSS="zot-jwt-a-cross"
JWT_SPIFFE_A="spiffe://${SPIRE_TRUST_DOMAIN}/zot/resourcesetinputproviders/${JWT_RSIP_A}"
JWT_SPIFFE_B="spiffe://${SPIRE_TRUST_DOMAIN}/zot/resourcesetinputproviders/${JWT_RSIP_B}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
WORKDIR="$(mktemp -d)"
CREATED_CLUSTER=false

log()  { echo -e "\n\033[1;34m==> $*\033[0m"; }
info() { echo "    $*"; }
fail() { echo -e "\033[1;31mFAIL: $*\033[0m" >&2; exit 1; }

cleanup() {
    local code=$?
    if [ "$code" -ne 0 ]; then
        log "FAILURE (exit ${code}) — dumping diagnostics"
        kubectl -n "${NS}" get pods -o wide 2>/dev/null || true
        kubectl -n "${NS}" get deploy -o wide 2>/dev/null || true
        kubectl -n "${NS}" logs deploy/flux-operator --tail=120 2>/dev/null || true
        kubectl -n "${SPIRE_NAMESPACE}" get pods -o wide 2>/dev/null || true
        kubectl -n "${APP_NS}" get resourcesetinputproviders -o yaml 2>/dev/null || true
        docker logs --tail=80 "${ZOT_CONTAINER}" 2>/dev/null || true
    fi
    if [ -n "${KEEP:-}" ]; then
        log "KEEP set — leaving cluster, container and ${WORKDIR} for inspection"
        exit "${code}"
    fi
    log "Cleaning up"
    docker rm -f "${ZOT_CONTAINER}" >/dev/null 2>&1 || true
    if [ "${CREATED_CLUSTER}" = "true" ]; then
        kind delete cluster --name "${CLUSTER}" >/dev/null 2>&1 || true
    fi
    rm -rf "${WORKDIR}" 2>/dev/null || sudo rm -rf "${WORKDIR}" 2>/dev/null || true
    exit "${code}"
}
trap cleanup EXIT

require() { command -v "$1" >/dev/null 2>&1 || fail "required tool '$1' not found in PATH"; }
for bin in docker kind kubectl flux helm; do require "$bin"; done

start_zot() {
    local config="$1"
    docker rm -f "${ZOT_CONTAINER}" >/dev/null 2>&1 || true
    docker run -d --name "${ZOT_CONTAINER}" --network kind \
        -p "127.0.0.1:${ZOT_HOST_PORT}:${ZOT_PORT}" \
        -v "${config}:/etc/zot/config.json:ro" \
        -v "${WORKDIR}/zot-data:/var/lib/zot" \
        "${ZOT_IMAGE}" serve /etc/zot/config.json >/dev/null

    local code
    for _ in $(seq 1 60); do
        code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${ZOT_HOST_PORT}/v2/" 2>/dev/null || echo 000)"
        case "${code}" in
            200|401) return 0 ;;
        esac
        sleep 1
    done
    docker logs --tail=80 "${ZOT_CONTAINER}" 2>/dev/null || true
    fail "Zot did not become ready (last HTTP status: ${code})"
}

##############################################################################
log "Creating kind cluster '${CLUSTER}' with an externally reachable issuer"
##############################################################################
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER}"; then
    info "Deleting pre-existing cluster"
    kind delete cluster --name "${CLUSTER}"
fi
cat <<EOF | kind create cluster --name "${CLUSTER}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      certSANs:
        - "${CLUSTER}-control-plane"
        - "127.0.0.1"
        - "localhost"
      extraArgs:
        service-account-issuer: "https://${CLUSTER}-control-plane:6443"
        anonymous-auth: "true"
EOF
CREATED_CLUSTER=true
kubectl config use-context "kind-${CLUSTER}" >/dev/null
kubectl wait --for=condition=Ready nodes --all --timeout=180s

OIDC_ISSUER="https://${CLUSTER}-control-plane:6443"
info "OIDC issuer: ${OIDC_ISSUER}"

log "Allowing unauthenticated OIDC discovery"
kubectl create clusterrolebinding oidc-reviewer \
    --clusterrole=system:service-account-issuer-discovery \
    --group=system:unauthenticated >/dev/null 2>&1 || true

log "Exporting the kind CA certificate"
docker cp "${CLUSTER}-control-plane:/etc/kubernetes/pki/ca.crt" "${WORKDIR}/kind-ca.pem"
mkdir -p "${WORKDIR}/zot-data"
chmod 0777 "${WORKDIR}/zot-data"

##############################################################################
log "Seeding Zot with an artifact while authentication is disabled"
##############################################################################
cat > "${WORKDIR}/zot-open.json" <<EOF
{
  "distSpecVersion": "1.1.1",
  "storage": {
    "rootDirectory": "/var/lib/zot"
  },
  "http": {
    "address": "0.0.0.0",
    "port": ${ZOT_PORT},
    "compat": ["docker2s2"]
  },
  "log": {
    "level": "debug"
  }
}
EOF
start_zot "${WORKDIR}/zot-open.json"

mkdir -p "${WORKDIR}/artifact-a" "${WORKDIR}/artifact-b"
echo "artifact for ${SA_NAME}" > "${WORKDIR}/artifact-a/index.txt"
echo "artifact for ${SA_NAME_B}" > "${WORKDIR}/artifact-b/index.txt"
flux push artifact "oci://localhost:${ZOT_HOST_PORT}/${REPO_A}:${REV}" \
    --path="${WORKDIR}/artifact-a" \
    --source=zot-oidc-e2e-a \
    --revision="${REV}"
flux push artifact "oci://localhost:${ZOT_HOST_PORT}/${REPO_B}:${REV}" \
    --path="${WORKDIR}/artifact-b" \
    --source=zot-oidc-e2e-b \
    --revision="${REV}"
docker rm -f "${ZOT_CONTAINER}" >/dev/null


##############################################################################
log "Installing SPIRE with the SPIFFE Broker API and OIDC discovery"
##############################################################################
kubectl create namespace "${SPIRE_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: spire-server
  namespace: ${SPIRE_NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: spire-server
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["authentication.k8s.io"]
    resources: ["tokenreviews"]
    verbs: ["get", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: spire-server
subjects:
  - kind: ServiceAccount
    name: spire-server
    namespace: ${SPIRE_NAMESPACE}
roleRef:
  kind: ClusterRole
  name: spire-server
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: spire-server
  namespace: ${SPIRE_NAMESPACE}
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["spire-bundle"]
    verbs: ["get", "patch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["create", "update", "get"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: spire-server
  namespace: ${SPIRE_NAMESPACE}
subjects:
  - kind: ServiceAccount
    name: spire-server
    namespace: ${SPIRE_NAMESPACE}
roleRef:
  kind: Role
  name: spire-server
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: spire-bundle
  namespace: ${SPIRE_NAMESPACE}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: spire-server
  namespace: ${SPIRE_NAMESPACE}
data:
  server.conf: |
    server {
      bind_address = "0.0.0.0"
      bind_port = "8081"
      trust_domain = "${SPIRE_TRUST_DOMAIN}"
      data_dir = "/run/spire/data"
      log_level = "INFO"
      default_x509_svid_ttl = "1h"
      ca_ttl = "12h"
      jwt_issuer = "${SPIRE_OIDC_ISSUER}"
    }

    plugins {
      DataStore "sql" {
        plugin_data {
          database_type = "sqlite3"
          connection_string = "/run/spire/data/datastore.sqlite3"
        }
      }

      NodeAttestor "k8s_psat" {
        plugin_data {
          clusters = {
            "broker-cluster" = {
              service_account_allow_list = ["${SPIRE_NAMESPACE}:spire-agent"]
            }
          }
        }
      }

      KeyManager "disk" {
        plugin_data {
          keys_path = "/run/spire/data/keys.json"
        }
      }

      BundlePublisher "k8s_configmap" {
        plugin_data {
          clusters = {
            "broker-cluster" = {
              configmap_name = "spire-bundle"
              configmap_key  = "bundle.crt"
              namespace      = "${SPIRE_NAMESPACE}"
              format         = "pem"
            }
          }
        }
      }
    }

    health_checks {
      listener_enabled = true
      bind_address = "0.0.0.0"
      bind_port = 8080
      live_path = "/live"
      ready_path = "/ready"
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: spire-server
  namespace: ${SPIRE_NAMESPACE}
  labels:
    app: spire-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: spire-server
  template:
    metadata:
      labels:
        app: spire-server
    spec:
      serviceAccountName: spire-server
      containers:
        - name: spire-server
          image: ${SPIRE_SERVER_IMAGE}
          args: ["-config", "/run/spire/config/server.conf"]
          ports:
            - containerPort: 8081
          volumeMounts:
            - name: spire-config
              mountPath: /run/spire/config
              readOnly: true
          livenessProbe:
            httpGet:
              path: /live
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
      volumes:
        - name: spire-config
          configMap:
            name: spire-server
---
apiVersion: v1
kind: Service
metadata:
  name: spire-server
  namespace: ${SPIRE_NAMESPACE}
spec:
  ports:
    - name: grpc
      port: 8081
      targetPort: 8081
  selector:
    app: spire-server
EOF
kubectl -n "${SPIRE_NAMESPACE}" rollout status deployment/spire-server --timeout=5m

NODE_UID="$(kubectl get nodes -o jsonpath='{.items[0].metadata.uid}')"
AGENT_ID="spiffe://${SPIRE_TRUST_DOMAIN}/spire/agent/k8s_psat/broker-cluster/${NODE_UID}"

create_entry() {
    local spiffe_id="$1"
    shift
    kubectl -n "${SPIRE_NAMESPACE}" exec deployment/spire-server -- \
        /opt/spire/bin/spire-server entry create \
            -spiffeID "${spiffe_id}" \
            -parentID "${AGENT_ID}" \
            "$@" >/dev/null
}

create_entry "spiffe://${SPIRE_TRUST_DOMAIN}/oidc-discovery-provider" \
    -selector "k8s:ns:${SPIRE_NAMESPACE}" \
    -selector "k8s:sa:spire-agent" \
    -selector "k8s:container-name:oidc-discovery-provider"
# The flux-operator uses the Workload API for its own Broker API client.
create_entry "spiffe://${SPIRE_TRUST_DOMAIN}/ns/${NS}/sa/${BROKER_SA}" \
    -selector "k8s:ns:${NS}" \
    -selector "k8s:sa:${BROKER_SA}"
create_entry "${JWT_SPIFFE_A}" \
    -selector "k8s:resource:${JWT_OBJ_TYPE}" \
    -selector "k8s:namespace:${APP_NS}" \
    -selector "k8s:name:${JWT_RSIP_A}"
create_entry "${JWT_SPIFFE_B}" \
    -selector "k8s:resource:${JWT_OBJ_TYPE}" \
    -selector "k8s:namespace:${APP_NS}" \
    -selector "k8s:name:${JWT_RSIP_B}"

kubectl apply -f - >/dev/null <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: spire-agent
  namespace: ${SPIRE_NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: spire-agent
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes", "nodes/proxy"]
    verbs: ["get", "list"]
  - apiGroups: ["fluxcd.controlplane.io"]
    resources: ["resourcesetinputproviders"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: spire-agent
subjects:
  - kind: ServiceAccount
    name: spire-agent
    namespace: ${SPIRE_NAMESPACE}
roleRef:
  kind: ClusterRole
  name: spire-agent
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: spire-agent
  namespace: ${SPIRE_NAMESPACE}
data:
  agent.conf: |
    agent {
      data_dir = "/run/spire"
      log_level = "INFO"
      server_address = "spire-server"
      server_port = "8081"
      socket_path = "/run/spire/agent-sockets/api.sock"
      trust_bundle_path = "/run/spire/bundle/bundle.crt"
      trust_domain = "${SPIRE_TRUST_DOMAIN}"

      experimental {
        broker {
          bind_address = "0.0.0.0:8443"
          brokers = [
            {
              id = "spiffe://${SPIRE_TRUST_DOMAIN}/ns/${NS}/sa/${BROKER_SA}"
              allowed_reference_types = [
                {
                  type_url = "type.googleapis.com/spiffe.broker.KubernetesObjectReference"
                  allow_over_tcp = true
                },
              ]
            },
          ]
        }
      }
    }

    plugins {
      NodeAttestor "k8s_psat" {
        plugin_data {
          cluster = "broker-cluster"
        }
      }

      KeyManager "memory" {
        plugin_data {}
      }

      WorkloadAttestor "k8s" {
        plugin_data {
          skip_kubelet_verification = true
          node_name_env = "MY_NODE_NAME"

          experimental {
            broker {
              access_policy = "permissive"
              brokers = [
                {
                  id = "spiffe://${SPIRE_TRUST_DOMAIN}/ns/${NS}/sa/${BROKER_SA}"
                },
              ]
            }
          }
        }
      }
    }

    health_checks {
      listener_enabled = true
      bind_address = "0.0.0.0"
      bind_port = 8080
      live_path = "/live"
      ready_path = "/ready"
    }
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: spire-oidc
  namespace: ${SPIRE_NAMESPACE}
data:
  oidc.conf: |
    log_level = "INFO"
    domains = ["${CLUSTER}-control-plane"]
    insecure_addr = "0.0.0.0:8082"
    allow_insecure_scheme = true
    jwt_issuer = "${SPIRE_OIDC_ISSUER}"
    jwks_uri = "${SPIRE_OIDC_ISSUER}/keys"
    workload_api {
      socket_path = "/run/spire/agent-sockets/api.sock"
      trust_domain = "${SPIRE_TRUST_DOMAIN}"
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: spire-agent
  namespace: ${SPIRE_NAMESPACE}
  labels:
    app: spire-agent
spec:
  selector:
    matchLabels:
      app: spire-agent
  template:
    metadata:
      labels:
        app: spire-agent
    spec:
      hostPID: true
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      serviceAccountName: spire-agent
      containers:
        - name: spire-agent
          image: ${SPIRE_AGENT_IMAGE}
          args: ["-config", "/run/spire/config/agent.conf"]
          env:
            - name: MY_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          ports:
            - name: broker
              containerPort: 8443
            - name: healthz
              containerPort: 8080
          volumeMounts:
            - name: spire-config
              mountPath: /run/spire/config
              readOnly: true
            - name: spire-bundle
              mountPath: /run/spire/bundle
              readOnly: true
            - name: spire-agent-socket
              mountPath: /run/spire/agent-sockets
            - name: spire-token
              mountPath: /var/run/secrets/tokens
          livenessProbe:
            httpGet:
              path: /live
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
        - name: oidc-discovery-provider
          image: ${SPIRE_OIDC_IMAGE}
          args: ["-config", "/run/spire/oidc/oidc.conf"]
          ports:
            - name: oidc
              containerPort: 8082
          readinessProbe:
            httpGet:
              path: /.well-known/openid-configuration
              port: 8082
              httpHeaders:
                - name: Host
                  value: ${CLUSTER}-control-plane
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: spire-oidc
              mountPath: /run/spire/oidc
              readOnly: true
            - name: spire-agent-socket
              mountPath: /run/spire/agent-sockets
      volumes:
        - name: spire-config
          configMap:
            name: spire-agent
        - name: spire-bundle
          configMap:
            name: spire-bundle
        - name: spire-oidc
          configMap:
            name: spire-oidc
        - name: spire-agent-socket
          hostPath:
            path: /run/spire/agent-sockets
            type: DirectoryOrCreate
        - name: spire-token
          projected:
            sources:
              - serviceAccountToken:
                  path: spire-agent
                  expirationSeconds: 7200
                  audience: spire-server
---
apiVersion: v1
kind: Service
metadata:
  name: spire-broker
  namespace: ${SPIRE_NAMESPACE}
spec:
  selector:
    app: spire-agent
  ports:
    - name: broker
      port: 8443
      targetPort: 8443
EOF
kubectl -n "${SPIRE_NAMESPACE}" rollout status daemonset/spire-agent --timeout=5m

##############################################################################
log "Installing the SPIFFE CSI driver"
##############################################################################
helm repo add spiffe https://spiffe.github.io/helm-charts-hardened >/dev/null 2>&1 || true
helm repo update spiffe >/dev/null
helm upgrade --install spire-csi spiffe/spire \
    --namespace "${SPIRE_NAMESPACE}" \
    --set global.spire.trustDomain="${SPIRE_TRUST_DOMAIN}" \
    --set global.spire.namespaces.create=false \
    --set spire-server.enabled=false \
    --set spire-agent.enabled=false \
    --set spiffe-csi-driver.enabled=true \
    --set spiffe-csi-driver.agentSocketPath=/run/spire/agent-sockets/api.sock \
    --set spiffe-oidc-discovery-provider.enabled=false \
    --wait --timeout 5m
kubectl -n "${SPIRE_NAMESPACE}" rollout status daemonset/spire-csi-spiffe-csi-driver --timeout=5m

##############################################################################
log "Starting Zot with OIDC bearer authentication"
##############################################################################
cat > "${WORKDIR}/zot-auth.json" <<EOF
{
  "distSpecVersion": "1.1.1",
  "storage": {
    "rootDirectory": "/var/lib/zot"
  },
  "http": {
    "address": "0.0.0.0",
    "port": ${ZOT_PORT},
    "compat": ["docker2s2"],
    "auth": {
      "bearer": {
        "realm": "http://${ZOT_CONTAINER}:${ZOT_PORT}/zot/auth/token",
        "service": "${ZOT_CONTAINER}:${ZOT_PORT}",
        "oidc": [
          {
            "issuer": "${OIDC_ISSUER}",
            "audiences": ["${AUDIENCE}"],
            "certificateAuthorityFile": "/etc/zot/kind-ca.pem"
          },
          {
            "issuer": "${SPIRE_OIDC_ISSUER}",
            "audiences": ["${AUDIENCE}"]
          }
        ]
      }
    },
    "accessControl": {
      "repositories": {
        "${REPO_A}": {
          "policies": [
            {
              "users": ["${OIDC_ISSUER}/system:serviceaccount:${APP_NS}:${SA_NAME}", "${SPIRE_OIDC_ISSUER}/${JWT_SPIFFE_A}"],
              "actions": ["read"]
            }
          ],
          "defaultPolicy": []
        },
        "${REPO_B}": {
          "policies": [
            {
              "users": ["${OIDC_ISSUER}/system:serviceaccount:${APP_NS}:${SA_NAME_B}", "${SPIRE_OIDC_ISSUER}/${JWT_SPIFFE_B}"],
              "actions": ["read"]
            }
          ],
          "defaultPolicy": []
        }
      }
    }
  },
  "log": {
    "level": "debug"
  }
}
EOF
docker run -d --name "${ZOT_CONTAINER}" --network kind \
    -p "127.0.0.1:${ZOT_HOST_PORT}:${ZOT_PORT}" \
    -v "${WORKDIR}/zot-auth.json:/etc/zot/config.json:ro" \
    -v "${WORKDIR}/kind-ca.pem:/etc/zot/kind-ca.pem:ro" \
    -v "${WORKDIR}/zot-data:/var/lib/zot" \
    "${ZOT_IMAGE}" serve /etc/zot/config.json >/dev/null

for _ in $(seq 1 60); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${ZOT_HOST_PORT}/v2/" 2>/dev/null || echo 000)"
    [ "${code}" = "401" ] && break
    sleep 1
done
[ "${code}" = "401" ] || fail "Zot did not start with bearer auth (last HTTP status: ${code})"

##############################################################################
log "Deploying flux-operator"
##############################################################################
make -C "${ROOT_DIR}" docker-build IMG="${IMG}" >/dev/null
kind load docker-image "${IMG}" --name "${CLUSTER}" >/dev/null
make -C "${ROOT_DIR}" deploy IMG="${IMG}" >/dev/null
kubectl -n "${NS}" rollout status deploy/flux-operator --timeout=5m

log "Configuring flux-operator with the SPIFFE Broker API"
kubectl -n "${NS}" patch deploy/flux-operator --type=strategic -p "$(cat <<EOF
spec:
  template:
    spec:
      containers:
        - name: manager
          args:
            - --spiffe-broker-endpoint=${SPIRE_BROKER_ENDPOINT}
            - --spiffe-broker-id=${SPIRE_BROKER_ID}
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: unix:///spiffe-workload-api/api.sock
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
kubectl -n "${NS}" rollout status deploy/flux-operator --timeout=3m

log "Creating ServiceAccounts and OCIArtifactTag providers"
##############################################################################
kubectl create namespace "${APP_NS}" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n "${APP_NS}" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME}
  annotations:
    credential.auth.fluxcd.io/provider: kubernetes
    credential.auth.fluxcd.io/type: jwt
    credential.auth.fluxcd.io/audiences: ${AUDIENCE}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SA_NAME_B}
  annotations:
    credential.auth.fluxcd.io/provider: kubernetes
    credential.auth.fluxcd.io/type: jwt
    credential.auth.fluxcd.io/audiences: ${AUDIENCE}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${WRONG_AUD_SA_NAME}
  annotations:
    credential.auth.fluxcd.io/provider: kubernetes
    credential.auth.fluxcd.io/type: jwt
    credential.auth.fluxcd.io/audiences: wrong-audience
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${JWT_SA_NAME}
  annotations:
    credential.auth.fluxcd.io/provider: spiffe
    credential.auth.fluxcd.io/type: jwt
    credential.auth.fluxcd.io/audiences: ${AUDIENCE}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${JWT_SA_NAME_B}
  annotations:
    credential.auth.fluxcd.io/provider: spiffe
    credential.auth.fluxcd.io/type: jwt
    credential.auth.fluxcd.io/audiences: ${AUDIENCE}
EOF

kubectl -n "${APP_NS}" apply -f - >/dev/null <<EOF
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${RSIP_A}
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${SA_NAME}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_A}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${RSIP_B}
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${SA_NAME_B}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_B}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${RSIP_A}-cross
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${SA_NAME}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_B}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${RSIP_B}-cross
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${SA_NAME_B}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_A}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${RSIP_A}-wrong-audience
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${WRONG_AUD_SA_NAME}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_A}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${JWT_RSIP_A}
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${JWT_SA_NAME}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_A}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${JWT_RSIP_B}
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${JWT_SA_NAME_B}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_B}
---
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: ${JWT_RSIP_A_CROSS}
  namespace: ${APP_NS}
spec:
  type: OCIArtifactTag
  insecure: true
  serviceAccountName: ${JWT_SA_NAME}
  url: oci://${ZOT_CONTAINER}:${ZOT_PORT}/${REPO_B}
EOF

assert_tag() {
    local name="$1" tags
    for _ in $(seq 1 30); do
        tags="$(kubectl -n "${APP_NS}" get "resourcesetinputprovider/${name}" \
            -o jsonpath='{.status.exportedInputs[*].tag}' 2>/dev/null || true)"
        if grep -qw "${REV}" <<<"${tags}"; then
            return 0
        fi
        sleep 2
    done
    fail "ResourceSetInputProvider ${name} did not export tag '${REV}' (tags=${tags:-none})"
}

log "Waiting for the owned-repository OCIArtifactTag providers"
kubectl -n "${APP_NS}" wait "resourcesetinputprovider/${RSIP_A}" \
    --for=condition=Ready --timeout=3m
kubectl -n "${APP_NS}" wait "resourcesetinputprovider/${RSIP_B}" \
    --for=condition=Ready --timeout=3m
assert_tag "${RSIP_A}"
assert_tag "${RSIP_B}"
info "Each ServiceAccount authenticated to its own repository"

log "Waiting for the JWT-SVID OCIArtifactTag providers"
kubectl -n "${APP_NS}" wait "resourcesetinputprovider/${JWT_RSIP_A}" \
    --for=condition=Ready --timeout=3m
kubectl -n "${APP_NS}" wait "resourcesetinputprovider/${JWT_RSIP_B}" \
    --for=condition=Ready --timeout=3m
assert_tag "${JWT_RSIP_A}"
assert_tag "${JWT_RSIP_B}"
info "Each JWT-SVID identity authenticated to its own repository"

expect_denied() {
    local name="$1" status msg
    for _ in $(seq 1 60); do
        status="$(kubectl -n "${APP_NS}" get "resourcesetinputprovider/${name}" \
            -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
        msg="$(kubectl -n "${APP_NS}" get "resourcesetinputprovider/${name}" \
            -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}' 2>/dev/null || true)"
        if [ "${status}" = "True" ]; then
            fail "ResourceSetInputProvider ${name} unexpectedly succeeded"
        fi
        if [ "${status}" = "False" ] && printf '%s' "${msg}" | grep -qiE '401|403|unauthorized|forbidden|denied|authentication|permission|svid|spiffe|no identity'; then
            info "ResourceSetInputProvider ${name} correctly denied"
            return 0
        fi
        sleep 2
    done
    fail "ResourceSetInputProvider ${name} did not fail in time (status=${status:-unknown}; message=${msg:-})"
}

log "Verifying cross-ServiceAccount and wrong-audience access is denied"
expect_denied "${RSIP_A}-cross"
expect_denied "${RSIP_B}-cross"
expect_denied "${RSIP_A}-wrong-audience"
expect_denied "${JWT_RSIP_A_CROSS}"

log "SUCCESS: OCIArtifactTag object identity accessed only the authorized repository"

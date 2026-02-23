#!/bin/bash
set -euo pipefail

# test-helm-install.sh
# validates helm chart installs correctly against a clean kind cluster
# and that deployed pods become healthy and respond to HTTP requests

CLUSTER_NAME="helm-test-$$"
NAMESPACE="mcp-system"
CHART_DIR="./charts/mcp-gateway"
TIMEOUT="300s"
ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

# tool paths (built by make)
KIND="${ROOT_DIR}/bin/kind"
HELM="${ROOT_DIR}/bin/helm"
KUSTOMIZE="${ROOT_DIR}/bin/kustomize"
YQ="${ROOT_DIR}/bin/yq"

cd "$ROOT_DIR"

cleanup() {
    echo "cleaning up kind cluster ${CLUSTER_NAME}..."
    # kill any background port-forward
    if [[ -n "${PF_PID:-}" ]] && kill -0 "$PF_PID" 2>/dev/null; then
        kill "$PF_PID" 2>/dev/null || true
        wait "$PF_PID" 2>/dev/null || true
    fi
    "$KIND" delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
}
trap cleanup EXIT

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

info() {
    echo "--- $1"
}

# check required tools exist
for tool in "$KIND" "$HELM" "$KUSTOMIZE" "$YQ"; do
    [[ -x "$tool" ]] || fail "required tool not found: $tool (run 'make tools' first)"
done
command -v docker >/dev/null 2>&1 || fail "docker is required"
command -v kubectl >/dev/null 2>&1 || fail "kubectl is required"

# 1. create minimal kind cluster
info "creating kind cluster ${CLUSTER_NAME}"
cat <<EOF | "$KIND" create cluster --name "$CLUSTER_NAME" --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 30080
    hostPort: 0
    protocol: TCP
EOF

# 2. install gateway API CRDs
info "installing gateway API CRDs"
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml
kubectl wait --for=condition=Established --timeout=60s crd/gateways.gateway.networking.k8s.io

# 3. install MCP CRDs
info "installing MCP CRDs"
kubectl apply -f config/crd/mcp.kagenti.com_mcpserverregistrations.yaml
kubectl apply -f config/crd/mcp.kagenti.com_mcpvirtualservers.yaml
kubectl apply -f config/crd/mcp.kagenti.com_mcpgatewayextensions.yaml
kubectl wait --for=condition=Established --timeout=60s crd/mcpserverregistrations.mcp.kagenti.com

# 4. install istio via sail operator
info "installing istio via sail operator"
SAIL_VERSION="1.27.0"
"$HELM" upgrade --install sail-operator \
    --create-namespace \
    --namespace istio-system \
    --wait \
    --timeout=300s \
    "https://github.com/istio-ecosystem/sail-operator/releases/download/${SAIL_VERSION}/sail-operator-${SAIL_VERSION}.tgz"
kubectl apply -f config/istio/istio.yaml
kubectl -n istio-system wait --for=condition=Ready istio/default --timeout="${TIMEOUT}"

# 5. install metallb for LoadBalancer support
info "installing metallb"
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.15.2/config/manifests/metallb-native.yaml
kubectl -n metallb-system wait --for=condition=Available deployments controller --timeout="${TIMEOUT}"
kubectl -n metallb-system wait --for=condition=ready pod --selector=app=metallb --timeout=120s
./utils/docker-network-ipaddresspool.sh kind "$YQ" | kubectl apply -n metallb-system -f -

# 6. build and load images into kind cluster
info "building images"
docker build --build-arg LDFLAGS="" -t ghcr.io/kuadrant/mcp-gateway:latest .
docker build --file Dockerfile.controller -t ghcr.io/kuadrant/mcp-controller:latest .

info "loading images into kind"
TMP_DIR=$(mktemp -d)
docker save -o "${TMP_DIR}/gateway.tar" ghcr.io/kuadrant/mcp-gateway:latest
"$KIND" load image-archive "${TMP_DIR}/gateway.tar" --name "$CLUSTER_NAME"
docker save -o "${TMP_DIR}/controller.tar" ghcr.io/kuadrant/mcp-controller:latest
"$KIND" load image-archive "${TMP_DIR}/controller.tar" --name "$CLUSTER_NAME"
rm -rf "$TMP_DIR"

# 7. create namespaces
info "creating namespaces"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace gateway-system --dry-run=client -o yaml | kubectl apply -f -

# 8. helm install
info "running helm install"
"$HELM" install mcp-gateway "$CHART_DIR" \
    --namespace "$NAMESPACE" \
    --set gateway.create=true \
    --set gateway.name=mcp-gateway \
    --set gateway.namespace=gateway-system \
    --set broker.create=true \
    --set controller.enabled=true \
    --set envoyFilter.create=true \
    --set envoyFilter.namespace=istio-system \
    --set envoyFilter.name=mcp-gateway \
    --set httpRoute.create=true \
    --set gateway.nodePort.create=true \
    --set gateway.publicHost=mcp.127-0-0-1.sslip.io \
    --set mcpGatewayExtension.gatewayRef.name=mcp-gateway \
    --set mcpGatewayExtension.gatewayRef.namespace=gateway-system \
    --wait \
    --timeout="${TIMEOUT}"

# 9. wait for deployments
info "waiting for broker deployment"
kubectl wait --for=condition=Available deployment -l app.kubernetes.io/instance=mcp-gateway -n "$NAMESPACE" --timeout="${TIMEOUT}"

info "waiting for gateway pod in gateway-system"
kubectl wait --for=condition=Programmed gateway/mcp-gateway -n gateway-system --timeout="${TIMEOUT}" || true

# 10. port-forward and test endpoints
info "port-forwarding to broker"
LOCAL_PORT=18080
kubectl port-forward -n "$NAMESPACE" deployment/mcp-gateway-mcp-gateway "${LOCAL_PORT}:8080" &
PF_PID=$!
sleep 3

# verify port-forward is running
kill -0 "$PF_PID" 2>/dev/null || fail "port-forward died"

info "testing /healthz endpoint"
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${LOCAL_PORT}/healthz" --max-time 10)
[[ "$HTTP_CODE" == "200" ]] || fail "/healthz returned HTTP ${HTTP_CODE}, expected 200"

info "testing /status endpoint"
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${LOCAL_PORT}/status" --max-time 10)
[[ "$HTTP_CODE" == "200" ]] || fail "/status returned HTTP ${HTTP_CODE}, expected 200"

echo ""
echo "PASS: helm install test completed successfully"

# Isolated MCP Gateway Deployment

This guide demonstrates how to deploy MCP Gateway instances for your environment. Each deployment is given its own configuration for which MCP Servers to manage based on the MCPGatewayExtension resource that defines which Gateway it expects request from.

This guide assumes some knowledge about configuring an MCPServerRegistration. You can find more information in the following guide [register-mcp-servers](./register-mcp-servers.md).

## Overview

The MCP Gateway requires an `MCPGatewayExtension` resource to operate. This resource:

- Defines which Gateway the MCP Gateway instance is responsible for
- Determines where configuration secrets are created (same namespace as the MCPGatewayExtension)
- Enables isolation by allowing multiple MCP Gateway instances (in different namespaces) to target different Gateways

MCPServerRegistration resources are only processed when a valid MCPGatewayExtension exists for the Gateway their HTTPRoute is attached to. Without a matching MCPGatewayExtension, registrations will show a NotReady status.

## Prerequisites

- Kubernetes cluster with Gateway API support
- Istio installed as Gateway API provider
- MCP Gateway CRDs installed
- Helm 3.x

## Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                        MCP System Namespace                           │
├───────────────────────────────────────────────────────────────────────┤
│                    MCP Controller (cluster-wide)                      │
└───────────────────────────────────────────────────────────────────────┘
                                    │
              ┌─────────────────────┴─────────────────────┐
              ▼                                           ▼
┌───────────────────────────────┐     ┌───────────────────────────────┐
│          Team A NS            │     │          Team B NS            │
├───────────────────────────────┤     ├───────────────────────────────┤
│ MCPGatewayExtension           │     │ MCPGatewayExtension           │
│   → Gateway A                 │     │   → Gateway B                 │
│ MCP Broker/Router             │     │ MCP Broker/Router             │
│ Config Secret                 │     │ Config Secret                 │
│ MCPServerRegistrations        │     │ MCPServerRegistrations        │
└───────────────────────────────┘     └───────────────────────────────┘
```

Each MCPGatewayExtension targets a different Gateway. The controller creates configuration secrets in the same namespace(s) as valid MCPGatewayExtension(s), which are mounted into the broker/router deployments.

For cross-namespace Gateway references, a ReferenceGrant must exist in the Gateway's namespace.

## Step 1: Deploy the MCP Controller

The MCP Controller runs cluster-wide and reconciles MCPGatewayExtension and MCPServerRegistration resources. Deploy it once in a central namespace:

```bash
helm install mcp-controller ./charts/mcp-gateway \
  --namespace mcp-system \
  --create-namespace \
  --set envoyFilter.create=false \
  --set mcpGatewayExtension.create=false
```

## Step 2: Create the Team Namespace

Create a namespace for the isolated MCP Gateway deployment:

```bash
kubectl create namespace team-alpha
```

## Step 3: Deploy MCP Gateway Instance

Deploy the broker and router into the team's namespace. The Helm chart automatically creates the MCPGatewayExtension and ReferenceGrant (for cross-namespace references):

```bash
helm install mcp-gateway ./charts/mcp-gateway \
  --namespace team-alpha \
  --set envoyFilter.create=true \
  --set envoyFilter.namespace=istio-system \
  --set gateway.publicHost=team-alpha.mcp.example.com \
  --set mcpGatewayExtension.gatewayRef.name=mcp-gateway \
  --set mcpGatewayExtension.gatewayRef.namespace=gateway-system
```

The Helm chart creates:
- MCPGatewayExtension targeting the specified Gateway
- ReferenceGrant in the Gateway namespace (for cross-namespace references)
- Broker/Router deployment
- Service for the broker
- EnvoyFilter to route traffic to this instance
- ServiceAccount and RBAC
- Config Secret for MCP server configuration

Wait for the MCPGatewayExtension to become ready:

```bash
kubectl wait --for=condition=Ready mcpgatewayextension/mcp-gateway -n team-alpha --timeout=60s
```

> **Note**: To skip automatic MCPGatewayExtension creation (e.g., for manual control), set `--set mcpGatewayExtension.create=false` and create the resources manually as shown in the [Manual Resource Creation](#manual-resource-creation) section.



## Step 4: Register MCP Servers

> Note: this assumes you have created a HTTPRoute name `alpha-server-route`

Create MCPServerRegistration resources in the team's namespace. The controller will automatically add their configuration to the team's config secret:

```bash
kubectl apply -f - <<EOF
apiVersion: mcp.kagenti.com/v1alpha1
kind: MCPServerRegistration
metadata:
  name: team-alpha-server
  namespace: team-alpha
spec:
  toolPrefix: alpha_
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: alpha-server-route
EOF
```

The MCPServerRegistration must target an HTTPRoute that is attached to a Gateway with a valid MCPGatewayExtension in the same namespace. Otherwise, the registration will show a NotReady status.

## Verification

Wait for the MCPServerRegistration to become ready:

```bash
kubectl wait --for=condition=Ready mcpserverregistration/team-alpha-server -n team-alpha --timeout=60s
```

Check that the configuration secret was created:

```bash
kubectl get secret mcp-gateway-config -n team-alpha
```

## Multiple Teams Example

To deploy a second isolated gateway for another team targeting a different Gateway:

```bash
# Create namespace
kubectl create namespace team-beta

# Create a second Gateway for team-beta (or use an existing one)
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: team-beta-gateway
  namespace: gateway-system
spec:
  gatewayClassName: istio
  listeners:
    - name: http
      port: 8080
      protocol: HTTP
      hostname: "*.team-beta.example.com"
EOF

# Deploy broker/router (Helm automatically creates MCPGatewayExtension and ReferenceGrant)
helm install mcp-gateway ./charts/mcp-gateway \
  --namespace team-beta \
  --set envoyFilter.create=true \
  --set envoyFilter.namespace=istio-system \
  --set gateway.publicHost=mcp.team-beta.example.com \
  --set mcpGatewayExtension.gatewayRef.name=team-beta-gateway \
  --set mcpGatewayExtension.gatewayRef.namespace=gateway-system
```

Each team now has their own isolated MCP Gateway instance targeting separate Gateways.

## Limitations

**Broker/Router must be co-located with MCPGatewayExtension**: The MCP Gateway instance (broker and router) must be deployed in the same namespace as the MCPGatewayExtension. The controller writes the configuration secret to the MCPGatewayExtension's namespace, and the broker/router mount this secret.

**One MCPGatewayExtension per namespace**: Each namespace can only have one MCPGatewayExtension. The controller writes configuration to a well-known secret name, so multiple extensions would overwrite each other.

**One MCPGatewayExtension per Gateway**: Only one MCPGatewayExtension can target a given Gateway. If multiple extensions target the same Gateway, the controller marks newer ones as conflicted. The oldest extension (by creation timestamp) wins.

## Troubleshooting

### MCPGatewayExtension shows RefGrantRequired

The MCPGatewayExtension is targeting a Gateway in a different namespace, but no ReferenceGrant exists:

```bash
kubectl get mcpgatewayextension -n team-alpha -o yaml
```

Look for the condition:
```yaml
conditions:
  - type: Ready
    status: "False"
    reason: ReferenceGrantRequired
    message: "ReferenceGrant required in namespace gateway-system to allow cross-namespace reference"
```

Create the ReferenceGrant in the Gateway's namespace as shown in Step 3.

### MCPGatewayExtension shows InvalidMCPGatewayExtension

The target Gateway doesn't exist or there's a conflict.

check if there is another MCPGatewayExtension that is older that is also targeting the Gateway.

### MCPServerRegistration shows NotReady

The registration can't find a valid MCPGatewayExtension for the Gateway its HTTPRoute is attached to:

```bash
kubectl get mcpserverregistration -n team-alpha -o yaml
```

Check that:
1. The HTTPRoute exists and references the correct Gateway and its status is accepted
2. An MCPGatewayExtension exists targeting that Gateway
3. The MCPGatewayExtension is in Ready state

### No configuration in secret

The config secret exists but has no servers:

```bash
kubectl get secret mcp-gateway-config -n team-alpha -o jsonpath='{.data.config\.yaml}' | base64 -d
```

Check that MCPServerRegistration resources exist and are Ready:

```bash
kubectl get mcpserverregistration -n team-alpha
```

## Cleanup

To remove an isolated deployment:

```bash
# Delete MCP server registrations
kubectl delete mcpserverregistration --all -n team-alpha

# Uninstall Helm release (removes MCPGatewayExtension and ReferenceGrant)
helm uninstall mcp-gateway -n team-alpha

# Delete namespace
kubectl delete namespace team-alpha
```

## Manual Resource Creation

If you prefer to create the MCPGatewayExtension and ReferenceGrant manually instead of having Helm manage them, disable automatic creation and apply the resources yourself:

### ReferenceGrant (Cross-Namespace Only)

If the MCPGatewayExtension is in a different namespace than the Gateway, create a ReferenceGrant in the Gateway's namespace:

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-team-alpha
  namespace: gateway-system
spec:
  from:
    - group: mcp.kagenti.com
      kind: MCPGatewayExtension
      namespace: team-alpha
  to:
    - group: gateway.networking.k8s.io
      kind: Gateway
EOF
```

### MCPGatewayExtension

Create the MCPGatewayExtension to associate the team's namespace with the target Gateway:

```bash
kubectl apply -f - <<EOF
apiVersion: mcp.kagenti.com/v1alpha1
kind: MCPGatewayExtension
metadata:
  name: team-alpha-gateway
  namespace: team-alpha
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: mcp-gateway
    namespace: gateway-system
EOF
```

### Deploy without automatic resource creation

```bash
helm install mcp-gateway ./charts/mcp-gateway \
  --namespace team-alpha \
  --set envoyFilter.create=true \
  --set envoyFilter.namespace=istio-system \
  --set gateway.publicHost=team-alpha.mcp.example.com \
  --set mcpGatewayExtension.create=false
```

# Vault Integration Example

<br/>
<br/>

## ❶ Setup a local environment

This step will create a local Kubernetes cluster and deploy the following components:
- MCP Gateway with Authorino enabled
- Sample MCP servers, including Test MCP Server 2 (used in this example to test the integration with Vault)
- Keycloak as OpenID Connect SSO provider
- Vault server

You can skip this step if you already have a Kubernetes cluster with a similar setup. In this case, make sure to adjust the commands and configuration accordingly.

#### 1.1 Create a local cluster with MCP Gateway and sample MCP servers

```sh
make local-env-setup
```

#### 1.2 Deploy Keycloak, Authorino, and Vault

```sh
make oauth-token-exchange-example-setup
```

#### 1.3 Enable required Keycloak client settings

Login to the Keycloak admin console and enable the "Direct Access grants" and "Service accounts roles" options for the "mcp-gateway" client.

- Keycloak Admin Console: http://keycloak.127-0-0-1.sslip.io:8002/auth/admin/
- Username: admin
- Password: admin

#### 1.4 Expose the Vault service locally

```sh
kubectl port-forward -n vault svc/vault 8200:8200 2>&1 >/dev/null &
export VAULT_TOKEN=root
```

<br/>
<br/>

## ❷ Integrate Vault with the MCP Gateway via Authorino

### OPTION A - Using JWTs for Authorino to authenticate to Vault

This option is more secure as it uses JWTs issued by Keycloak for Authorino to authenticate to Vault. Authorino can be granted limited access to Vault based on configured policies.

#### 2.A1 Configure Vault to trust Keycloak as OIDC provider

Patch the Vault deployment to resolve Keycloak's host name to MCP gateway IP:

```sh
export GATEWAY_IP=$(kubectl get gateway/mcp-gateway -n gateway-system -o jsonpath='{.status.addresses[0].value}' 2>/dev/null || true)
kubectl patch deployment/vault -n vault --type='json' -p="$(cat config/keycloak/patch-hostaliases.json | envsubst)"
```

Enable JWT authentication method in Vault:

```sh
curl -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST \
  --data '{"type":"jwt"}' \
  http://localhost:8200/v1/sys/auth/jwt
```

Trust Keycloak as OIDC provider in Vault:

```sh
kubectl get configmap/mcp-gateway-keycloak-cert -n kuadrant-system -o jsonpath='{.data.keycloak\.crt}' | \
  jq -Rs '{oidc_discovery_url: "https://keycloak.127-0-0-1.sslip.io:8002/realms/mcp", oidc_discovery_ca_pem: ., default_role: "authorino"}' | \
  curl -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST --data @- http://localhost:8200/v1/auth/jwt/config
```

Create a Vault policy and Vault role for Authorino:

```sh
curl -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST \
  --data '{
    "policy": "path \"secret/data/mcp-gateway/*\" {\n  capabilities = [\"read\", \"list\"]\n}"
  }' \
  http://localhost:8200/v1/sys/policies/acl/authorino
```

```sh
curl -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST \
  --data '{
    "role_type": "jwt",
    "bound_audiences": ["account"],
    "user_claim": "azp",
    "policies": ["authorino"],
    "ttl": "1h"
  }' \
  http://localhost:8200/v1/auth/jwt/role/authorino
```

#### 2.A2 Create the AuthPolicy

```sh
kubectl apply -f -<<EOF
apiVersion: kuadrant.io/v1
kind: AuthPolicy
metadata:
  name: vault-integration-policy
  namespace: mcp-test
spec:
  # Change it to target your MCP server route that requires fetching credentials from Vault or
  # the entire MCP Gateway listener used to route internal MCP traffic if all routes require Vault integration
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: mcp-server2-route
  rules:
    authentication:
      "mcp-clients":
        jwt:
          # Change it to the issuer URL of your OpenId Connect SSO provider
          # You can also use jwksUrl instead of issuerUrl for authentication servers that do not implement OIDC Discovery
          issuerUrl: https://keycloak.127-0-0-1.sslip.io:8002/realms/mcp
    metadata:
      "keycloak-token":
        priority: 0
        http:
          url: https://keycloak.127-0-0-1.sslip.io:8002/realms/mcp/protocol/openid-connect/token
          method: POST
          credentials:
            authorizationHeader:
              prefix: Basic
          sharedSecretRef:
            name: authorino-oauth-client
            key: client_secret
          bodyParameters:
            grant_type:
              value: client_credentials
            scope:
              value: openid
        cache:
          key:
            value: 'singleton'
          ttl: 1800 # 30 minutes
      "vault-login":
        priority: 1
        when:
        - predicate: auth.metadata.exists(p, p == "keycloak-token") && has(auth.metadata["keycloak-token"].access_token)
        http:
          url: http://vault.vault.svc.cluster.local:8200/v1/auth/jwt/login
          method: POST
          body:
            expression: |
              "{\"role\": \"authorino\", \"jwt\": \"" + auth.metadata["keycloak-token"].access_token + "\"}"
        cache:
          key:
            value: 'singleton'
          ttl: 3600 # 1 hour
      "vault":
        priority: 2
        when:
        - predicate: auth.metadata.exists(p, p == "vault-login") && has(auth.metadata["vault-login"].auth ) && has(auth.metadata["vault-login"].auth.client_token)
        http:
          # Change it to your Vault server URL and secret path
          urlExpression: |
            "http://vault.vault.svc.cluster.local:8200/v1/secret/data/mcp-gateway/" + auth.identity.preferred_username
          method: GET
          headers:
            "X-Vault-Token":
              expression: auth.metadata["vault-login"].auth.client_token
    authorization:
      "found-vault-secret":
        patternMatching:
          patterns:
          - predicate: |
              has(auth.metadata.vault.data) && has(auth.metadata.vault.data.data) && has(auth.metadata.vault.data.data.test_server2_pat) && type(auth.metadata.vault.data.data.test_server2_pat) == string
    response:
      success:
        headers:
          "Authorization":
            plain:
              expression: |
                "Bearer " + auth.metadata.vault.data.data.test_server2_pat
      unauthenticated:
        code: 401
        headers:
          'WWW-Authenticate':
            value: Bearer resource_metadata=http://mcp.127-0-0-1.sslip.io:8001/.well-known/oauth-protected-resource/mcp
        body:
          value: |
            {
              "error": "Forbidden",
              "message": "MCP Tool Access denied. Unauthenticated."
            }
      unauthorized:
        code: 403
        body:
          value: |
            {
              "error": "Forbidden",
              "message": "MCP Tool Access denied. Insufficient permissions for this tool."
            }
---
apiVersion: v1
kind: Secret
metadata:
  name: authorino-oauth-client
  namespace: kuadrant-system
stringData:
  client_secret: <secret-value>
type: Opaque
EOF
```

> **Note:** Due to the typical size of the token responses returned by Keycloak, Authorino might not be able to cache them if the default cache size limit (1KB) is not increased. In such cases, you can increase the cache size limit by setting `spec.evaluatorCacheSize` option of the Authorino custom resource (e.g., to 2KB).

<br/>
<br/>

### OPTION B - Using Vault root token for Authorino to authenticate to Vault

This option is simpler, but much less secure. It uses the Vault root token for Authorino to authenticate to Vault, which grants Authorino with full access to read and write any secret stored in Vault.

#### 2.B1 Create the AuthPolicy

```sh
kubectl apply -f -<<EOF
apiVersion: kuadrant.io/v1
kind: AuthPolicy
metadata:
  name: vault-integration-policy
  namespace: mcp-test
spec:
  # Change it to target your MCP server route that requires fetching credentials from Vault or
  # the entire MCP Gateway listener used to route internal MCP traffic if all routes require Vault integration
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: mcp-server2-route
  rules:
    authentication:
      "mcp-clients":
        jwt:
          # Change it to the issuer URL of your OpenId Connect SSO provider
          # You can also use jwksUrl instead of issuerUrl for authentication servers that do not implement OIDC Discovery
          issuerUrl: https://keycloak.127-0-0-1.sslip.io:8002/realms/mcp
    metadata:
      "vault":
        http:
          # Change it to your Vault server URL and secret path
          urlExpression: |
            "http://vault.vault.svc.cluster.local:8200/v1/secret/data/mcp-gateway/" + auth.identity.preferred_username
          method: GET
          credentials:
            customHeader:
              name: X-Vault-Token
          sharedSecretRef:
            name: vault-secret
            key: root-token
    authorization:
      "found-vault-secret":
        patternMatching:
          patterns:
          - predicate: |
              has(auth.metadata.vault.data) && has(auth.metadata.vault.data.data) && has(auth.metadata.vault.data.data.test_server2_pat) && type(auth.metadata.vault.data.data.test_server2_pat) == string
    response:
      success:
        headers:
          "Authorization":
            plain:
              expression: |
                "Bearer " + auth.metadata.vault.data.data.test_server2_pat
      unauthenticated:
        code: 401
        headers:
          'WWW-Authenticate':
            value: Bearer resource_metadata=http://mcp.127-0-0-1.sslip.io:8001/.well-known/oauth-protected-resource/mcp
        body:
          value: |
            {
              "error": "Forbidden",
              "message": "MCP Tool Access denied. Unauthenticated."
            }
      unauthorized:
        code: 403
        body:
          value: |
            {
              "error": "Forbidden",
              "message": "MCP Tool Access denied. Insufficient permissions for this tool."
            }
---
apiVersion: v1
kind: Secret
metadata:
  name: vault-secret
  namespace: kuadrant-system
stringData:
  root-token: root
type: Opaque
EOF
```

### ❸ Test the integration with Vault

#### 3.1 Store a secret in Vault

```sh
curl -s -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST \
  --data '{"data":{"test_server2_pat":"s3cr3t"}}' \
  http://localhost:8200/v1/secret/data/mcp-gateway/mcp
```

#### 3.2 Start a session with the MCP Gateway

Obtain an access token from Keycloak:

```sh
ACCESS_TOKEN=$(curl -k https://keycloak.127-0-0-1.sslip.io:8002/realms/mcp/protocol/openid-connect/token -s -d 'grant_type=password' -d 'client_id=mcp-gateway' -d 'client_secret=secret' -d 'username=mcp' -d 'password=mcp' -d 'scope=openid profile groups roles' | jq -r .access_token)
```

Initialize the session with the MCP Gateway:

```sh
MCP_SESSION_ID=$(curl -s -o /dev/null -w '%header{mcp-session-id}\n' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'mcp-protocol-version: 2025-06-18' \
  --data-raw '{"method":"initialize","params":{"_meta":{"progressToken":1}},"jsonrpc":"2.0","id":1}' \
  http://mcp.127-0-0-1.sslip.io:8001/mcp)
```

#### 3.3 Send a request to the MCP server route that requires fetching credentials from Vault

Call the `headers` tool of Test MCP Server 2:

```sh
curl -s \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'mcp-protocol-version: 2025-06-18' \
  -H "mcp-session-id: $MCP_SESSION_ID" \
  --data-raw '{"method":"tools/call","params":{"name":"test2_headers","_meta":{"progressToken":1}},"jsonrpc":"2.0","id":1}' \
  http://server1.127-0-0-1.sslip.io:8001/mcp
```

The output should show that the request was successful and the `Authorization:` header was set using the secret fetched from Vault:

```jsonc
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Authorization: [Bearer <ACCESS_TOKEN>]"
      },
      …
    ],
    …
  }
}
```

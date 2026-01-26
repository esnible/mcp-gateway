# Integrating Vault with MCP Gateway

<br/>
<br/>

## Overview

The Kuadrant MCP Gateway provides a centralized way to connect AI agents to tools with the Model Context Protocol (MCP). Many backend MCP servers require sensitive credentials such as API keys or Personal Access Tokens (PATs) to access external APIs (for example, GitHub and Slack).

### Using Vault

You can use HashiCorp Vault to securely store these credentials and a Kuadrant AuthPolicy to retrieve and inject those credentials into the request flow. Authorino is used by Kuadrant to add authorization and authentication to APIs that do not have credential checks built-in. The essentials of the workflow include the following elements:

- MCP Gateway: Acts as the entry point for AI clients (e.g., Claude Code, VS Code).
- Authorino: The external authorization service used by Kuadrant to validate identities and fetch external metadata.
- HashiCorp Vault: The source of truth for secrets.
- AuthPolicy: The Kuadrant resource that defines how to authenticate the user and fetch their specific secret from Vault.

### Using an existing setup

If you already have a Kubernetes cluster, a central authorization tool that uses standard protocols like OpenID Connect or OAuth 2.0, and an MCP server ready to connect, use the AuthPolicy examples that follow to create your own object and apply it.

Adjust the commands and configuration in the following examples according to your use case. The goal of this documentation is to guide you on testing an AuthPolicy that speaks with an OIDC server on one side and with a Vault instance on the other.

The sample procedure presented below enforces an AuthPolicy that integrates MCP Gateway with Vault for requests targeting a specific MCP server. For enforcing the Vault integration policy on multiple servers, you can consider targeting a Gateway or individual gateway Listener instead. However, those details are beyond the scope of this documentation.
<!--
### Creating a setup

If you do not have a cluster and related resources ready to try Vault with, then you can continue with the local environment setup. The following procedure outlines a workflow and example commands. Deploying Keycloak, Kuadrant, and Vault with this procedure is resource-heavy and disruptive. Use only in development environments.

Creating a local environment requires several steps:
- clone the repo and use developer tools which build and deploy a custom image of the MCP Gateway
- deploy resources that are not customized to your use case
- configure self-signed TLS certs and test-only DNS services
- reconfigure the Kubernetes API server's authentication

These steps are not required if you are running the procedure in your own preexisting environment.

<details>
<summary>Set up a local environment</summary>

This procedure creates a local Kind cluster and deploys the following components:
- MCP Gateway with Kuadrant and Authorino enabled
- Sample MCP servers, including Test MCP Server 2 (used in this example to test the integration with Vault)
- Keycloak as OpenID Connect SSO provider
- Vault server

#### 1. Clone the repo

```sh
git clone git@github.com:kuadrant/mcp-gateway.git && cd mcp-gateway
```

#### 2. Create a local Kind cluster with MCP Gateway and sample MCP servers

```sh
make local-env-setup
```

#### 3. Deploy Keycloak, Kuadrant, and Vault

```sh
make oauth-token-exchange-example-setup
```

#### 4. Enable required Keycloak client settings

Login to the Keycloak admin console at:

- Keycloak Admin Console: https://keycloak.127-0-0-1.sslip.io:8002/
- Username: admin
- Password: admin

Enable the "Direct Access grants" and "Service accounts roles" options for the "mcp-gateway" client.

Map the `mcp` user to the `mcp-test/mcp-server2-route`'s `headers` client role.

#### 5. Expose the Vault service locally

```sh
kubectl port-forward -n vault svc/vault 8200:8200 2>&1 >/dev/null &
export VAULT_TOKEN=root
```
</details>
<br/>

## Integrate Vault with the MCP Gateway Using JWTs

The following provides a workflow for using JSON Web Tokens (JWTs) for Authorino to authenticate to Vault. This option is much more secure than using a root token. Authorino can be granted limited access to Vault based on configured policies, such as role-based policies.
-->
## Enable JWT authentication in your Vault server

For instructions on how to configure JWT authentication in Vault, see [Vault's documentation](https://developer.hashicorp.com/vault/api-docs/auth/jwt#configure).
<!--
- You must have the JWT `auth` method enabled in Vault (`vault auth enable jwt`) and a role configured that trusts the OIDC issuer.
- Make sure the connection to `vault.vault.svc.cluster.local` is secure. In a production environment, using `https` and providing a CA certificate trusted by the Authorino instance are best practices.
-->
Make sure to create a Vault policy and Vault role that grants access for Authorino to read secrets at the `secret/data/mcp-gateway/*` path, or whatever path you decided to namespace the MCP server secrets.

<details>
<summary>Example Vault policy and role for Authorino</summary>

In general, a Vault policy for Authorino has the following fields:

```sh
curl -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST \
  --data '{
    "bound_audiences": ["authorino"],
    "user_claim": "sub",
  }' \
```
<!--
The following Vault policy examples are specific to Keycloak:

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
</details>
-->
### Create the AuthPolicy

Create an AuthPolicy to connect an external OIDC Identity Provider (IdP) with Vault to get a Vault token on behalf of the user or service that needs access to the MCP server data.

```sh
kubectl apply -f -<<EOF
apiVersion: kuadrant.io/v1
kind: AuthPolicy
metadata:
  name: vault-integration-policy
  namespace: <target_namespace>
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: <your-httproute-name>
  rules:
    authentication:
      "oidc-token":
        priority: 0
        http:
          url: <replace-with-token-url>
      # If you are using a shared secret, you can add the following:
      sharedSecret:
            name: oidc-client-credentials # This points to a K8s secret
            namespace: <kuadrant-system>
      "vault-login":
        priority: 1
        when:
        - predicate: auth.metadata.exists(p, p == "oidc-token") && has(auth.metadata["oidc-token"].access_token)
        http:
          url: http://vault.vault.svc.cluster.local:8200/v1/auth/jwt/login
          method: POST
          body:
            expression: |
              "{\"role\": \"<replace-with-vault-role>\", \"jwt\": \"" + auth.metadata["oidc-token"].access_token + "\"}"
    authorization:
      "validate-vault-response":
        priority: 0
        when:
        - predicate: auth.metadata.exists(p, p == "vault-login")
        json:
          rules:
          - selector: auth.metadata["vault-login"].auth.client_token
            operator: ne
            value: ""
    # The Injector: Passes the successful token to your app
    response:
      success:
        filters:
          "inject-vault-token":
            header:
              name: X-Vault-Token
              value:
                expression: auth.metadata["vault-login"].auth.client_token
EOF
```
- `oidc-token`: the policy makes a call to the OIDC provider.
- `vault-login`: the series of steps that perform the JWT authentication against the secrets stored in Vault.

<br/>
<br/>

## Using a Vault root token

This option is easier to use, but much less secure than using tokens configured with policies. Using the Vault root token for Authorino to authenticate to Vault gives Authorino full access to read and write any secret stored in Vault. Use a root token only for initial setup or in development environments.

The following `AuthPolicy` is an example. Update it with the specifications that apply to your use case.

### Create the AuthPolicy

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
    name: <mcp-server2-route>
  rules:
    authentication:
      "mcp-clients":
        jwt:
          issuerUrl: <your-issuer-url>
          # Use the issuer URL of your OpenId Connect SSO provider
          # Or an jwksUrl instead for authentication servers that do not implement OIDC Discovery
    metadata:
      "vault":
        http:
          # Use your Vault server URL and secret path
          urlExpression: |
            "https://vault.vault.svc.cluster.local:8200/v1/secret/data/mcp-gateway/" + auth.identity.preferred_username
          # Using preferred_username skips Configs if you are using Keycloak
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

## Testing MCP Gateway integration with Vault

You can test your MCP Gateway integration by using the general steps that follow. Example commands are available in the details lists for reference.

### 1. Store a secret in Vault

<details>
<summary>Example curl command to store a vault token</summary>

```sh
curl -s -H "X-Vault-Token: $VAULT_TOKEN" -H 'Content-Type: application/json' -X POST \
  --data '{"data":{"test_server2_pat":"s3cr3t"}}' \
  http://localhost:8200/v1/secret/data/mcp-gateway/mcp
```
</details>

### 2. Get an access token

<details>
<summary>Example access token request</summary>

```sh
ACCESS_TOKEN=$(curl <replace-with-your-issuer-url> -s -d 'grant_type=client_credentials' -d 'client_id=<mcp-client-id>' -d 'client_secret=<mcp-client-secret>' -d 'scope=openid profile groups roles' | jq -r .access_token)
```
</details>

### 3. Start a session with the MCP Gateway

You can initialize a session according to your own development environment's set up.

<!--
<details>
<summary>Example session initialization</summary>

If you are using the local environment set up given in this guide, use the following command:

```sh
MCP_SESSION_ID=$(curl -s -o /dev/null -w '%header{mcp-session-id}\n' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'mcp-protocol-version: 2025-06-18' \
  --data-raw '{"method":"initialize","params":{"_meta":{"progressToken":1}},"jsonrpc":"2.0","id":1}' \
  http://mcp.127-0-0-1.sslip.io:8001/mcp)
```
</details>
-->

### Send a request to the MCP server route that requires fetching credentials from Vault

You can send a request to the MCP server according to your own development environment's set up.
<!--
If you are using the local environment set up given in this guide, use the following command:

<details>
<summary>Example request</summary>

Call the `headers` tool of Test MCP Server 2:

```sh
curl -s \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'mcp-protocol-version: 2025-06-18' \
  -H "mcp-session-id: $MCP_SESSION_ID" \
  --data-raw '{"method":"tools/call","params":{"name":"test2_headers","_meta":{"progressToken":1}},"jsonrpc":"2.0","id":1}' \
  http://mcp.127-0-0-1.sslip.io:8001/mcp
```
</details>
-->
### Example output

The expected output shows that the request was successful and the `Authorization:` header was set using the secret fetched from Vault:

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

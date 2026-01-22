# Releasing MCP Gateway

Creating a GitHub release triggers automated workflows that:
1. Build and push container images (`mcp-gateway`, `mcp-controller`) to `ghcr.io/kuadrant/`
2. Package and push the Helm chart to `oci://ghcr.io/kuadrant/charts/mcp-gateway`

## Release Steps

### 1. Update Helm Version

```bash
git checkout main
git pull
git checkout -b release-X.Y.Z
```

Edit `config/openshift/deploy_openshift.sh` and update `MCP_GATEWAY_HELM_VERSION`:
```bash
MCP_GATEWAY_HELM_VERSION="${MCP_GATEWAY_HELM_VERSION:-X.Y.Z}"
```

Commit and push:
```bash
git add config/openshift/deploy_openshift.sh
git commit -s -m "Update MCP_GATEWAY_HELM_VERSION to X.Y.Z"
git push -u origin release-X.Y.Z
```

### 2. Create GitHub Release

1. Go to [Releases](https://github.com/Kuadrant/mcp-gateway/releases)
2. Click **Draft a new release**
3. Click **Choose a tag** and create a new tag `vX.Y.Z`
4. Set **Target** to your `release-X.Y.Z` branch
5. Set the release title to `vX.Y.Z`
6. Click **Generate release notes**
7. Click **Publish release**

### 3. Verify Workflows Complete

1. [Build Images](https://github.com/Kuadrant/mcp-gateway/actions/workflows/images.yaml) - builds container images with version tag
2. [Helm Chart Release](https://github.com/Kuadrant/mcp-gateway/actions/workflows/helm-release.yaml) - pushes chart to OCI registry

### 4. Verify Published Artifacts

```bash
docker pull ghcr.io/kuadrant/mcp-gateway:vX.Y.Z
docker pull ghcr.io/kuadrant/mcp-controller:vX.Y.Z
helm show chart oci://ghcr.io/kuadrant/charts/mcp-gateway --version X.Y.Z
```

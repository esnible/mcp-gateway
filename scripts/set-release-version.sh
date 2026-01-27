#!/bin/bash

# Updates version references in release-related files
# Usage: ./scripts/set-release-version.sh <version>
# Example: ./scripts/set-release-version.sh 0.5.0
# Example: ./scripts/set-release-version.sh 0.5.0-rc1

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 0.5.0"
    echo "Example: $0 0.5.0-rc1"
    exit 1
fi

VERSION="$1"

# Validate version format (semver with optional pre-release)
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$ ]]; then
    echo "Error: Version must be in semver format X.Y.Z or X.Y.Z-prerelease"
    echo "Examples: 0.5.0, 0.5.0-rc1, 0.5.0-alpha.1"
    exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

echo "Setting release version to: $VERSION"

# Update config/openshift/deploy_openshift.sh
OPENSHIFT_SCRIPT="$REPO_ROOT/config/openshift/deploy_openshift.sh"
if [ -f "$OPENSHIFT_SCRIPT" ]; then
    sed -i.bak -E "s/MCP_GATEWAY_HELM_VERSION=\"\\\$\{MCP_GATEWAY_HELM_VERSION:-[^}]+\}\"/MCP_GATEWAY_HELM_VERSION=\"\${MCP_GATEWAY_HELM_VERSION:-$VERSION}\"/" "$OPENSHIFT_SCRIPT"
    rm -f "$OPENSHIFT_SCRIPT.bak"
    echo "Updated: $OPENSHIFT_SCRIPT"
else
    echo "Warning: $OPENSHIFT_SCRIPT not found"
fi

# Update charts/sample_local_helm_setup.sh
SAMPLE_SCRIPT="$REPO_ROOT/charts/sample_local_helm_setup.sh"
if [ -f "$SAMPLE_SCRIPT" ]; then
    sed -i.bak -E "s/--version [0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?/--version $VERSION/" "$SAMPLE_SCRIPT"
    rm -f "$SAMPLE_SCRIPT.bak"
    echo "Updated: $SAMPLE_SCRIPT"
else
    echo "Warning: $SAMPLE_SCRIPT not found"
fi

echo "Done. Version set to $VERSION"
echo ""
echo "Files updated:"
echo "  - config/openshift/deploy_openshift.sh"
echo "  - charts/sample_local_helm_setup.sh"
echo ""
echo "Review changes with: git diff"

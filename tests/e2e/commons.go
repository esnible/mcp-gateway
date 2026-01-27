//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test timeouts and intervals
const (
	TestTimeoutMedium     = time.Second * 60
	TestTimeoutLong       = time.Minute * 3
	TestTimeoutConfigSync = time.Minute * 6
	TestRetryInterval     = time.Second * 5
)

// Namespace constants
const (
	TestNamespace   = "mcp-test"
	SystemNamespace = "mcp-system"
	ConfigMapName   = "mcp-gateway-config"
)

// UniqueName generates a unique name with the given prefix
func UniqueName(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

// CleanupResource deletes a resource, ignoring not found errors
func CleanupResource(ctx context.Context, k8sClient client.Client, obj client.Object) {
	err := k8sClient.Delete(ctx, obj)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			Expect(err).ToNot(HaveOccurred())
		}
	}
}

// Legacy aliases for backwards compatibility during migration
// These delegate to the new unified builder

// NewMCPServerResourcesWithDefaults creates a new builder with defaults (legacy alias)
func NewMCPServerResourcesWithDefaults(testName string, k8sClient client.Client) *TestResourcesBuilder {
	return NewTestResourcesWithDefaults(testName, k8sClient)
}

// NewMCPServerResources creates a new builder for a specific service (legacy alias)
func NewMCPServerResources(testName, hostName, serviceName string, port int32, k8sClient client.Client) *TestResourcesBuilder {
	return NewTestResources(testName, k8sClient).
		ForInternalService(serviceName, port).
		WithHostname(hostName)
}

// NewExternalMCPServerResources creates a new builder for external services (legacy alias)
func NewExternalMCPServerResources(testName string, k8sClient client.Client, externalHost string, port int32) *TestResourcesBuilder {
	return NewTestResources(testName, k8sClient).
		ForExternalService(externalHost, port)
}

// BuildTestMCPVirtualServer creates a virtual server builder (legacy alias)
func BuildTestMCPVirtualServer(name, namespace string, tools []string) *MCPVirtualServerBuilder {
	return NewMCPVirtualServerBuilder(name, namespace).WithTools(tools)
}

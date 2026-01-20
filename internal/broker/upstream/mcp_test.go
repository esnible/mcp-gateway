package upstream

import (
	"testing"

	"github.com/Kuadrant/mcp-gateway/internal/config"
	"github.com/stretchr/testify/require"
)

// NewUpstreamMCP creates a new MCPServer instance from the provided configuration.
// It sets up default headers including user-agent and gateway-server-id, and adds
// an Authorization header if credentials are configured.
func TestNewUpstreamMCP(t *testing.T) {
	testServer := config.MCPServer{
		Name:       "test-server",
		URL:        "http://localhost:8088/mcp",
		ToolPrefix: "",
		Enabled:    true,
		Hostname:   "dummy",
	}
	up := NewUpstreamMCP(&testServer)
	require.NotNil(t, up)
	require.Equal(t, testServer, up.GetConfig())
}

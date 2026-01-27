package upstream

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/Kuadrant/mcp-gateway/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

// MockMCP implements the MCP interface for testing
type MockMCP struct {
	name            string
	prefix          string
	id              config.UpstreamMCPID
	cfg             *config.MCPServer
	connectErr      error
	pingErr         error
	tools           []mcp.Tool
	listToolsErr    error
	protocolVersion string
	hasToolsCap     bool
	connected       bool
}

func (m *MockMCP) GetName() string {
	return m.name
}

func (m *MockMCP) GetConfig() config.MCPServer {
	return *m.cfg
}

func (m *MockMCP) ID() config.UpstreamMCPID {
	return m.id
}

func (m *MockMCP) GetPrefix() string {
	return m.prefix
}

func (m *MockMCP) Connect(_ context.Context, onConnected func()) error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	if onConnected != nil {
		onConnected()
	}
	return nil
}

func (m *MockMCP) SupportsToolsListChanged() bool {
	return m.hasToolsCap
}

func (m *MockMCP) Disconnect() error {
	m.connected = false
	return nil
}

func (m *MockMCP) ListTools(_ context.Context, _ mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	if m.listToolsErr != nil {
		return nil, m.listToolsErr
	}
	return &mcp.ListToolsResult{Tools: m.tools}, nil
}

func (m *MockMCP) OnNotification(_ func(notification mcp.JSONRPCNotification)) {}

func (m *MockMCP) OnConnectionLost(_ func(err error)) {}

func (m *MockMCP) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *MockMCP) ProtocolInfo() *mcp.InitializeResult {
	result := &mcp.InitializeResult{
		ProtocolVersion: m.protocolVersion,
		Capabilities:    mcp.ServerCapabilities{},
	}
	if m.hasToolsCap {
		result.Capabilities.Tools = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{}
	}
	return result
}

// newMockMCP creates a MockMCP with sensible defaults for testing
func newMockMCP(name, prefix string) *MockMCP {
	id := config.UpstreamMCPID(fmt.Sprintf("%s:%s:http://mock/mcp", name, prefix))
	return &MockMCP{
		name:            name,
		prefix:          prefix,
		id:              id,
		cfg:             &config.MCPServer{Name: name, ToolPrefix: prefix, URL: "http://mock/mcp"},
		protocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		hasToolsCap:     true,
		tools:           []mcp.Tool{{Name: "mock_tool"}},
	}
}

func TestDiffTools(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("test-server", "test_")
	manager := NewUpstreamMCPManager(mock, nil, logger, 0)

	tests := []struct {
		name            string
		oldTools        []mcp.Tool
		newTools        []mcp.Tool
		expectedAdded   int
		expectedRemoved int
		addedNames      []string
		removedNames    []string
	}{
		{
			name:            "no changes",
			oldTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			newTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			expectedAdded:   0,
			expectedRemoved: 0,
		},
		{
			name:            "add new tool",
			oldTools:        []mcp.Tool{{Name: "tool1"}},
			newTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			expectedAdded:   1,
			expectedRemoved: 0,
			addedNames:      []string{"test_tool2"},
		},
		{
			name:            "remove tool",
			oldTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			newTools:        []mcp.Tool{{Name: "tool1"}},
			expectedAdded:   0,
			expectedRemoved: 1,
			removedNames:    []string{"test_tool2"},
		},
		{
			name:            "add and remove tools",
			oldTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			newTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool3"}},
			expectedAdded:   1,
			expectedRemoved: 1,
			addedNames:      []string{"test_tool3"},
			removedNames:    []string{"test_tool2"},
		},
		{
			name:            "empty old tools",
			oldTools:        []mcp.Tool{},
			newTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			expectedAdded:   2,
			expectedRemoved: 0,
		},
		{
			name:            "empty new tools",
			oldTools:        []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			newTools:        []mcp.Tool{},
			expectedAdded:   0,
			expectedRemoved: 2,
		},
		{
			name:            "both empty",
			oldTools:        []mcp.Tool{},
			newTools:        []mcp.Tool{},
			expectedAdded:   0,
			expectedRemoved: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := manager.diffTools(tt.oldTools, tt.newTools)
			assert.Len(t, added, tt.expectedAdded, "unexpected number of added tools")
			assert.Len(t, removed, tt.expectedRemoved, "unexpected number of removed tools")

			if len(tt.addedNames) > 0 {
				addedToolNames := make([]string, len(added))
				for i, tool := range added {
					addedToolNames[i] = tool.Tool.Name
				}
				for _, expectedName := range tt.addedNames {
					assert.Contains(t, addedToolNames, expectedName)
				}
			}

			if len(tt.removedNames) > 0 {
				for _, expectedName := range tt.removedNames {
					assert.Contains(t, removed, expectedName)
				}
			}
		})
	}
}

// MockGatewayServer implements ToolsAdderDeleter for testing
type MockGatewayServer struct {
	tools map[string]*server.ServerTool
	mu    sync.Mutex
}

func NewMockGatewayServer() *MockGatewayServer {
	return &MockGatewayServer{
		tools: make(map[string]*server.ServerTool),
	}
}

func (m *MockGatewayServer) AddTools(tools ...server.ServerTool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range tools {
		m.tools[tools[i].Tool.Name] = &tools[i]
	}
}

func (m *MockGatewayServer) DeleteTools(names ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, name := range names {
		delete(m.tools, name)
	}
}

func (m *MockGatewayServer) ListTools() map[string]*server.ServerTool {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]*server.ServerTool, len(m.tools))
	for k, v := range m.tools {
		result[k] = v
	}
	return result
}

func TestServerToolsManagement(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name                 string
		prefix               string
		initialTools         []mcp.Tool // tools returned by first ListTools call to backend MCP
		updatedTools         []mcp.Tool // tools returned by second ListTools call to backend MCP
		expectedServerTools  []string   // expected tool names in serverTools after update from backend MCP
		expectedGatewayTools []string   // expected tool names in gateway after update from backend MCP
	}{
		{
			name:                 "add tools to empty",
			prefix:               "test_",
			initialTools:         []mcp.Tool{},
			updatedTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			expectedServerTools:  []string{"test_tool1", "test_tool2"},
			expectedGatewayTools: []string{"test_tool1", "test_tool2"},
		},
		{
			name:                 "remove single tool",
			prefix:               "test_",
			initialTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}, {Name: "tool3"}},
			updatedTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool3"}},
			expectedServerTools:  []string{"test_tool1", "test_tool3"},
			expectedGatewayTools: []string{"test_tool1", "test_tool3"},
		},
		{
			name:                 "remove multiple tools",
			prefix:               "test_",
			initialTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}, {Name: "tool3"}},
			updatedTools:         []mcp.Tool{{Name: "tool1"}},
			expectedServerTools:  []string{"test_tool1"},
			expectedGatewayTools: []string{"test_tool1"},
		},
		{
			name:                 "add and remove tools simultaneously",
			prefix:               "test_",
			initialTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			updatedTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool3"}, {Name: "tool4"}},
			expectedServerTools:  []string{"test_tool1", "test_tool3", "test_tool4"},
			expectedGatewayTools: []string{"test_tool1", "test_tool3", "test_tool4"},
		},
		{
			name:                 "no changes keeps existing tools",
			prefix:               "test_",
			initialTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			updatedTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			expectedServerTools:  []string{"test_tool1", "test_tool2"},
			expectedGatewayTools: []string{"test_tool1", "test_tool2"},
		},
		{
			name:                 "remove all tools",
			prefix:               "test_",
			initialTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			updatedTools:         []mcp.Tool{},
			expectedServerTools:  []string{},
			expectedGatewayTools: []string{},
		},
		{
			name:                 "works without prefix",
			prefix:               "",
			initialTools:         []mcp.Tool{{Name: "tool1"}},
			updatedTools:         []mcp.Tool{{Name: "tool1"}, {Name: "tool2"}},
			expectedServerTools:  []string{"tool1", "tool2"},
			expectedGatewayTools: []string{"tool1", "tool2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockMCP := newMockMCP("test-server", tt.prefix)
			mockGateway := NewMockGatewayServer()
			manager := NewUpstreamMCPManager(mockMCP, mockGateway, logger, 0)

			// First manage call - establish initial tools
			mockMCP.tools = tt.initialTools
			manager.manage(ctx)

			// Second manage call - apply updates
			mockMCP.tools = tt.updatedTools
			manager.manage(ctx)

			// Verify serverTools
			manager.toolsLock.RLock()
			serverToolNames := make([]string, len(manager.serverTools))
			for i, st := range manager.serverTools {
				serverToolNames[i] = st.Tool.Name
			}
			manager.toolsLock.RUnlock()

			assert.ElementsMatch(t, tt.expectedServerTools, serverToolNames,
				"serverTools mismatch")

			// Verify gateway tools
			gatewayTools := mockGateway.ListTools()
			gatewayToolNames := make([]string, 0, len(gatewayTools))
			for name := range gatewayTools {
				gatewayToolNames = append(gatewayToolNames, name)
			}

			assert.ElementsMatch(t, tt.expectedGatewayTools, gatewayToolNames,
				"gateway tools mismatch")

			// Verify no duplicates in serverTools
			seen := make(map[string]bool)
			for _, name := range serverToolNames {
				assert.False(t, seen[name], "duplicate tool found: %s", name)
				seen[name] = true
			}
		})
	}
}

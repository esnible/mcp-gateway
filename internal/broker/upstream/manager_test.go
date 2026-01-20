package upstream

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

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

// MockToolsAdderDeleter implements ToolsAdderDeleter for testing
type MockToolsAdderDeleter struct {
	tools    map[string]*server.ServerTool
	addCalls int
	delCalls int
}

func newMockToolsAdderDeleter() *MockToolsAdderDeleter {
	return &MockToolsAdderDeleter{
		tools: make(map[string]*server.ServerTool),
	}
}

func (m *MockToolsAdderDeleter) AddTools(tools ...server.ServerTool) {
	m.addCalls++
	for i := range tools {
		m.tools[tools[i].Tool.Name] = &tools[i]
	}
}

func (m *MockToolsAdderDeleter) DeleteTools(names ...string) {
	m.delCalls++
	for _, name := range names {
		delete(m.tools, name)
	}
}

func (m *MockToolsAdderDeleter) ListTools() map[string]*server.ServerTool {
	return m.tools
}

func TestNewUpstreamMCPManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("uses default ticker interval when zero", func(t *testing.T) {
		mock := newMockMCP("test", "")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

		assert.Equal(t, DefaultTickerInterval, manager.tickerInterval)
		assert.NotNil(t, manager.done)
		assert.NotNil(t, manager.toolsMap)
		assert.NotNil(t, manager.servedToolsMap)
	})

	t.Run("uses custom ticker interval when provided", func(t *testing.T) {
		mock := newMockMCP("test", "")
		gateway := newMockToolsAdderDeleter()
		customInterval := time.Second * 30
		manager := NewUpstreamMCPManager(mock, gateway, logger, customInterval)

		assert.Equal(t, customInterval, manager.tickerInterval)
	})

	t.Run("uses default ticker interval when negative", func(t *testing.T) {
		mock := newMockMCP("test", "")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, -1)

		assert.Equal(t, DefaultTickerInterval, manager.tickerInterval)
	})
}

func TestMCPManager_MCPName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("my-test-server", "prefix_")
	manager := NewUpstreamMCPManager(mock, nil, logger, 0)

	assert.Equal(t, "my-test-server", manager.MCPName())
}

func TestMCPManager_GetStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("test-server", "test_")
	manager := NewUpstreamMCPManager(mock, nil, logger, 0)

	expectedStatus := ServerValidationStatus{
		ID:            "test-id",
		Name:          "test-server",
		LastValidated: time.Now(),
		Message:       "test message",
		Ready:         true,
		TotalTools:    5,
	}
	manager.SetStatusForTesting(expectedStatus)

	status := manager.GetStatus()
	assert.Equal(t, expectedStatus.ID, status.ID)
	assert.Equal(t, expectedStatus.Name, status.Name)
	assert.Equal(t, expectedStatus.Ready, status.Ready)
	assert.Equal(t, expectedStatus.TotalTools, status.TotalTools)
}

func TestMCPManager_GetManagedTools(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("test-server", "test_")
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

	tools := []mcp.Tool{
		{Name: "tool1", Description: "Tool 1"},
		{Name: "tool2", Description: "Tool 2"},
	}
	manager.SetToolsForTesting(tools)

	managedTools := manager.GetManagedTools()

	assert.Len(t, managedTools, 2)
	assert.Equal(t, "tool1", managedTools[0].Name)
	assert.Equal(t, "tool2", managedTools[1].Name)
}

func TestMCPManager_GetManagedTools_ReturnsCopy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("test-server", "test_")
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

	tools := []mcp.Tool{
		{Name: "tool1"},
	}
	manager.SetToolsForTesting(tools)

	// get tools and modify the returned slice
	managedTools := manager.GetManagedTools()
	managedTools[0].Name = "modified"

	// original should be unchanged
	original := manager.GetManagedTools()
	assert.Equal(t, "tool1", original[0].Name)
}

func TestMCPManager_GetServedManagedTool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("returns tool with prefix", func(t *testing.T) {
		mock := newMockMCP("test-server", "prefix_")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

		tools := []mcp.Tool{
			{Name: "mytool", Description: "My Tool"},
		}
		manager.SetToolsForTesting(tools)

		// should find with prefixed name
		tool := manager.GetServedManagedTool("prefix_mytool")
		assert.NotNil(t, tool)
		assert.Equal(t, "mytool", tool.Name)
	})

	t.Run("returns tool without prefix", func(t *testing.T) {
		mock := newMockMCP("test-server", "")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

		tools := []mcp.Tool{
			{Name: "mytool", Description: "My Tool"},
		}
		manager.SetToolsForTesting(tools)

		// should find without prefix
		tool := manager.GetServedManagedTool("mytool")
		assert.NotNil(t, tool)
		assert.Equal(t, "mytool", tool.Name)
	})

	t.Run("returns nil for non-existent tool", func(t *testing.T) {
		mock := newMockMCP("test-server", "prefix_")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

		tools := []mcp.Tool{
			{Name: "mytool"},
		}
		manager.SetToolsForTesting(tools)

		tool := manager.GetServedManagedTool("nonexistent")
		assert.Nil(t, tool)
	})
}

func TestMCPManager_setStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("sets success status", func(t *testing.T) {
		mock := newMockMCP("test-server", "test_")
		manager := NewUpstreamMCPManager(mock, nil, logger, 0)
		manager.serverTools = make([]server.ServerTool, 3)

		manager.setStatus(nil, 3)

		assert.Equal(t, string(mock.id), manager.status.ID)
		assert.Equal(t, "test-server", manager.status.Name)
		assert.True(t, manager.status.Ready)
		assert.Equal(t, 3, manager.status.TotalTools)
		assert.Contains(t, manager.status.Message, "server added successfully")
	})

	t.Run("sets error status", func(t *testing.T) {
		mock := newMockMCP("test-server", "test_")
		manager := NewUpstreamMCPManager(mock, nil, logger, 0)

		testErr := fmt.Errorf("connection failed")
		manager.setStatus(testErr, 0)

		assert.Equal(t, string(mock.id), manager.status.ID)
		assert.Equal(t, "test-server", manager.status.Name)
		assert.False(t, manager.status.Ready)
		assert.Equal(t, "connection failed", manager.status.Message)
	})
}

func TestMCPManager_hasTools(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("returns false when no tools", func(t *testing.T) {
		mock := newMockMCP("test", "")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

		assert.False(t, manager.hasTools())
	})

	t.Run("returns true when tools exist", func(t *testing.T) {
		mock := newMockMCP("test", "")
		gateway := newMockToolsAdderDeleter()
		manager := NewUpstreamMCPManager(mock, gateway, logger, 0)
		manager.serverTools = []server.ServerTool{
			{Tool: mcp.Tool{Name: "tool1"}},
		}

		assert.True(t, manager.hasTools())
	})
}

func TestPrefixedName(t *testing.T) {
	testCases := []struct {
		name     string
		prefix   string
		toolName string
		expected string
	}{
		{
			name:     "with prefix",
			prefix:   "server_",
			toolName: "tool",
			expected: "server_tool",
		},
		{
			name:     "without prefix",
			prefix:   "",
			toolName: "tool",
			expected: "tool",
		},
		{
			name:     "prefix with underscore",
			prefix:   "my_prefix_",
			toolName: "mytool",
			expected: "my_prefix_mytool",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := prefixedName(tc.prefix, tc.toolName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMCPManager_toolToServerTool(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("test-server", "prefix_")
	manager := NewUpstreamMCPManager(mock, nil, logger, 0)

	tool := mcp.Tool{
		Name:        "mytool",
		Description: "A test tool",
	}

	serverTool := manager.toolToServerTool(tool)

	assert.Equal(t, "prefix_mytool", serverTool.Tool.Name)
	assert.Equal(t, "A test tool", serverTool.Tool.Description)

	// check that meta has id field
	id, ok := serverTool.Tool.Meta.AdditionalFields["id"]
	assert.True(t, ok)
	assert.Equal(t, string(mock.id), id)

	// handler should return error result
	result, err := serverTool.Handler(context.Background(), mcp.CallToolRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestMCPManager_Stop_Idempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mock := newMockMCP("test", "")
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, time.Hour)

	// calling Stop multiple times should not panic
	manager.Stop()
	manager.Stop()
	manager.Stop()

	// verify manager state after stop
	assert.False(t, mock.connected, "mock should be disconnected after stop")
}

func TestMCPManager_manage_ConnectError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := newMockMCP("test-server", "test_")
	mock.connectErr = fmt.Errorf("connection refused")
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

	manager.manage(context.Background())

	status := manager.GetStatus()
	assert.False(t, status.Ready)
	assert.Contains(t, status.Message, "connection refused")
}

func TestMCPManager_manage_PingError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := newMockMCP("test-server", "test_")
	mock.pingErr = fmt.Errorf("ping timeout")
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

	manager.manage(context.Background())

	status := manager.GetStatus()
	assert.False(t, status.Ready)
	assert.Contains(t, status.Message, "ping")
}

func TestMCPManager_manage_ListToolsError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := newMockMCP("test-server", "test_")
	mock.listToolsErr = fmt.Errorf("list tools failed")
	mock.hasToolsCap = false // ensure we try to list tools
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

	manager.manage(context.Background())

	status := manager.GetStatus()
	assert.False(t, status.Ready)
	assert.Contains(t, status.Message, "list tools")
}

func TestMCPManager_manage_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mock := newMockMCP("test-server", "test_")
	mock.tools = []mcp.Tool{
		{Name: "tool1"},
		{Name: "tool2"},
	}
	mock.hasToolsCap = false // ensure we list tools every time
	gateway := newMockToolsAdderDeleter()
	manager := NewUpstreamMCPManager(mock, gateway, logger, 0)

	manager.manage(context.Background())

	status := manager.GetStatus()
	assert.True(t, status.Ready)
	assert.Equal(t, 2, status.TotalTools)

	// tools should be added to gateway
	assert.Len(t, gateway.tools, 2)
	assert.Contains(t, gateway.tools, "test_tool1")
	assert.Contains(t, gateway.tools, "test_tool2")
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

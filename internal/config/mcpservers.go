// Package config provides configuration types
package config

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"strings"
)

// MCPServersConfig holds server configuration
type MCPServersConfig struct {
	Servers        []*MCPServer
	VirtualServers []*VirtualServer
	observers      []Observer
}

// RegisterObserver registers an observer to be notified of changes to the config
func (config *MCPServersConfig) RegisterObserver(obs Observer) {
	config.observers = append(config.observers, obs)
}

// Notify notifies registered observers of config changes
func (config *MCPServersConfig) Notify(ctx context.Context) {
	for _, observer := range config.observers {
		observer.OnConfigChange(ctx, config)
	}
}

// StripServerPrefix returns the stripped tool name and whether stripping was needed
func (config *MCPServersConfig) StripServerPrefix(toolName string) string {
	if config == nil {
		return toolName
	}

	// strip matching prefix
	for _, server := range config.Servers {
		if strippedToolName, ok := strings.CutPrefix(toolName, server.ToolPrefix); ok {
			slog.Info("Stripped tool name", "tool", strippedToolName, "originalPrefix", server.ToolPrefix)
			return strippedToolName
		}
	}
	return toolName
}

// GetServerInfo retrieve the server info based on a prefix toolname. The prefix here is really used
// as a server id as it is unique to each registered MCPServer
func (config *MCPServersConfig) GetServerInfo(toolName string) *MCPServer {

	// find server by prefix
	for _, server := range config.Servers {
		if server.Enabled && strings.HasPrefix(toolName, server.ToolPrefix) {
			slog.Info("[EXT-PROC] Found matching server",
				"toolName", toolName,
				"serverPrefix", server.ToolPrefix,
				"serverName", server.Name)
			return server
		}
	}

	slog.Info("Tool name doesn't match any configured server prefix", "tool", toolName)
	return nil
}

// MCPServer represents a server
type MCPServer struct {
	Name             string
	URL              string
	ToolPrefix       string
	Enabled          bool
	Hostname         string
	CredentialEnvVar string // env var name for auth
}

// Path returns the path part of the mcp url
func (mcpServer *MCPServer) Path() (string, error) {
	parsedURL, err := url.Parse(mcpServer.URL)
	if err != nil {
		return "", err
	}
	return parsedURL.Path, nil
}

// Credential returns the configured credential for a backend MCP server
func (mcpServer *MCPServer) Credential() string {
	if mcpServer.CredentialEnvVar != "" {
		return os.Getenv(mcpServer.CredentialEnvVar)
	}
	return ""
}

// VirtualServer represents a virtual server configuration
type VirtualServer struct {
	Name  string
	Tools []string
}

// Observer provides an interface to implement in order to register as an Observer of config changes
type Observer interface {
	OnConfigChange(ctx context.Context, config *MCPServersConfig)
}

package broker

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetOAuthConfig(t *testing.T) {
	testCases := []struct {
		name     string
		envVars  map[string]string
		expected *OAuthProtectedResource
	}{
		{
			name:    "default values when no env vars set",
			envVars: map[string]string{},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{},
				BearerMethodsSupported: []string{"header"},
				ScopesSupported:        []string{"basic"},
			},
		},
		{
			name: "resource name override",
			envVars: map[string]string{
				envOAuthResourceName: "Custom MCP Server",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "Custom MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{},
				BearerMethodsSupported: []string{"header"},
				ScopesSupported:        []string{"basic"},
			},
		},
		{
			name: "resource override",
			envVars: map[string]string{
				envOAuthResource: "/custom/endpoint",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/custom/endpoint",
				AuthorizationServers:   []string{},
				BearerMethodsSupported: []string{"header"},
				ScopesSupported:        []string{"basic"},
			},
		},
		{
			name: "single authorization server",
			envVars: map[string]string{
				envOAuthAuthorizationServers: "https://auth.example.com",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{"https://auth.example.com"},
				BearerMethodsSupported: []string{"header"},
				ScopesSupported:        []string{"basic"},
			},
		},
		{
			name: "multiple authorization servers with whitespace",
			envVars: map[string]string{
				envOAuthAuthorizationServers: "https://auth1.example.com, https://auth2.example.com , https://auth3.example.com",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{"https://auth1.example.com", "https://auth2.example.com", "https://auth3.example.com"},
				BearerMethodsSupported: []string{"header"},
				ScopesSupported:        []string{"basic"},
			},
		},
		{
			name: "bearer methods override",
			envVars: map[string]string{
				envOAuthBearerMethodsSupported: "header, body",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{},
				BearerMethodsSupported: []string{"header", "body"},
				ScopesSupported:        []string{"basic"},
			},
		},
		{
			name: "scopes override",
			envVars: map[string]string{
				envOAuthScopesSupported: "read, write, admin",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{},
				BearerMethodsSupported: []string{"header"},
				ScopesSupported:        []string{"read", "write", "admin"},
			},
		},
		{
			name: "all env vars set",
			envVars: map[string]string{
				envOAuthResourceName:           "Full Config Server",
				envOAuthResource:               "/api/mcp",
				envOAuthAuthorizationServers:   "https://auth.example.com",
				envOAuthBearerMethodsSupported: "header,body",
				envOAuthScopesSupported:        "mcp:read,mcp:write",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "Full Config Server",
				Resource:               "/api/mcp",
				AuthorizationServers:   []string{"https://auth.example.com"},
				BearerMethodsSupported: []string{"header", "body"},
				ScopesSupported:        []string{"mcp:read", "mcp:write"},
			},
		},
		{
			name: "whitespace trimming on all comma-separated values",
			envVars: map[string]string{
				envOAuthAuthorizationServers:   "  https://auth1.com  ,  https://auth2.com  ",
				envOAuthBearerMethodsSupported: "  header  ,  body  ",
				envOAuthScopesSupported:        "  read  ,  write  ",
			},
			expected: &OAuthProtectedResource{
				ResourceName:           "MCP Server",
				Resource:               "/mcp",
				AuthorizationServers:   []string{"https://auth1.com", "https://auth2.com"},
				BearerMethodsSupported: []string{"header", "body"},
				ScopesSupported:        []string{"read", "write"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// use t.Setenv which automatically cleans up after each test
			// first clear all env vars by setting them to empty (t.Setenv handles cleanup)
			for _, envVar := range []string{
				envOAuthResourceName,
				envOAuthResource,
				envOAuthAuthorizationServers,
				envOAuthBearerMethodsSupported,
				envOAuthScopesSupported,
			} {
				t.Setenv(envVar, "")
			}

			// set env vars for this test case (overrides the empty values above)
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			result := getOAuthConfig()

			require.Equal(t, tc.expected.ResourceName, result.ResourceName)
			require.Equal(t, tc.expected.Resource, result.Resource)
			require.Equal(t, tc.expected.AuthorizationServers, result.AuthorizationServers)
			require.Equal(t, tc.expected.BearerMethodsSupported, result.BearerMethodsSupported)
			require.Equal(t, tc.expected.ScopesSupported, result.ScopesSupported)
		})
	}
}

func TestProtectedResourceHandler_Handle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		checkBody      bool
	}{
		{
			name:           "GET request returns JSON response",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			checkBody:      true,
		},
		{
			name:           "POST request returns JSON response",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			checkBody:      true,
		},
		{
			name:           "OPTIONS preflight request returns 200",
			method:         http.MethodOptions,
			expectedStatus: http.StatusOK,
			checkBody:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// clear env vars to use defaults using t.Setenv (auto cleanup)
			for _, envVar := range []string{
				envOAuthResourceName,
				envOAuthResource,
				envOAuthAuthorizationServers,
				envOAuthBearerMethodsSupported,
				envOAuthScopesSupported,
			} {
				t.Setenv(envVar, "")
			}

			handler := &ProtectedResourceHandler{Logger: logger}

			req := httptest.NewRequest(tc.method, "/.well-known/oauth-protected-resource", nil)
			rec := httptest.NewRecorder()

			handler.Handle(rec, req)

			require.Equal(t, tc.expectedStatus, rec.Code)

			// check CORS headers
			require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			require.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
			require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")

			if tc.checkBody {
				require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				var response OAuthProtectedResource
				err := json.NewDecoder(rec.Body).Decode(&response)
				require.NoError(t, err)

				// verify default values
				require.Equal(t, "MCP Server", response.ResourceName)
				require.Equal(t, "/mcp", response.Resource)
			}
		})
	}
}

func TestProtectedResourceHandler_Handle_WithCustomConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// set custom env vars using t.Setenv (auto cleanup)
	t.Setenv(envOAuthResourceName, "Test Server")
	t.Setenv(envOAuthResource, "/test/mcp")
	t.Setenv(envOAuthAuthorizationServers, "https://auth.test.com")

	handler := &ProtectedResourceHandler{Logger: logger}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response OAuthProtectedResource
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	require.Equal(t, "Test Server", response.ResourceName)
	require.Equal(t, "/test/mcp", response.Resource)
	require.Equal(t, []string{"https://auth.test.com"}, response.AuthorizationServers)
}

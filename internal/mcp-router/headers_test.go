package mcprouter

import (
	"testing"
)

func Test_Headers(t *testing.T) {
	headersBuilder := NewHeaders()

	expected := map[string]string{
		toolHeader:          "test_tool",
		authorityHeader:     "mcp1.mcp.local",
		"authorization":     "auth",
		methodHeader:        "tools/call",
		mcpServerNameHeader: "mcp1",
		sessionHeader:       "xxxx",
		":path":             "/mcp1",
	}

	headers := headersBuilder.
		WithAuthority(expected[authorityHeader]).
		WithAuth(expected["authorization"]).
		WithMCPMethod(expected[methodHeader]).
		WithMCPServerName(expected[mcpServerNameHeader]).
		WithMCPSession(expected[sessionHeader]).
		WithMCPToolName(expected[toolHeader]).
		WithPath("/mcp1").
		Build()

	for key, value := range expected {
		found := false
		for _, h := range headers {
			if h.Header.Key == key && string(h.Header.RawValue) == value {
				found = true
				continue
			}
		}
		if !found {
			t.Fatalf("did not find %s in %v", key, headers)
		}
	}
}

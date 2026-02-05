//go:build e2e

package e2e

import (
	"context"
	"crypto/tls"
	"maps"
	"net/http"
	"strings"

	goenv "github.com/caitlinelfring/go-env-default"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/ginkgo/v2"
)

var useInsecureClient = goenv.GetDefault("INSECURE_CLIENT", "false")

// NotifyingMCPClient wraps an MCP client with notification handling
type NotifyingMCPClient struct {
	*mcpclient.Client
	sessionID string
}

// NewMCPGatewayClient creates a new MCP client connected to the gateway
func NewMCPGatewayClient(ctx context.Context, gatewayHost string) (*mcpclient.Client, error) {
	return NewMCPGatewayClientWithHeaders(ctx, gatewayHost, nil)
}

// NewMCPGatewayClientWithHeaders creates a new MCP client with custom headers
func NewMCPGatewayClientWithHeaders(ctx context.Context, gatewayHost string, headers map[string]string) (*mcpclient.Client, error) {
	allHeaders := map[string]string{"e2e": "client"}
	maps.Copy(allHeaders, headers)
	options := []transport.StreamableHTTPCOption{transport.
		WithHTTPHeaders(allHeaders), transport.WithContinuousListening()}
	if strings.ToLower(useInsecureClient) == "true" {
		GinkgoWriter.Println("using insecure client for tests")
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		options = append(options, transport.WithHTTPBasicClient(client))
	}

	gatewayClient, err := mcpclient.NewStreamableHttpClient(gatewayHost, options...)
	if err != nil {
		return nil, err
	}
	err = gatewayClient.Start(ctx)
	if err != nil {
		return nil, err
	}
	_, err = gatewayClient.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "e2e",
				Version: "0.0.1",
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return gatewayClient, nil
}

// NewMCPGatewayClientWithNotifications creates an MCP client that captures notifications
func NewMCPGatewayClientWithNotifications(ctx context.Context, gatewayHost string, notificationFunc func(mcp.JSONRPCNotification)) (*NotifyingMCPClient, error) {
	client, err := NewMCPGatewayClientWithHeaders(ctx, gatewayHost, nil)
	if err != nil {
		return nil, err
	}

	client.OnNotification(func(notification mcp.JSONRPCNotification) {
		if notificationFunc != nil {
			notificationFunc(notification)
			return
		}
		GinkgoWriter.Println("default notification handler", notification)
	})

	client.OnConnectionLost(func(err error) {
		GinkgoWriter.Println("connection lost", err)
	})

	return &NotifyingMCPClient{
		Client:    client,
		sessionID: client.GetSessionId(),
	}, nil
}

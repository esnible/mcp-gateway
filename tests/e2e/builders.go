//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istiov1beta1 "istio.io/api/networking/v1beta1"
	istionetv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	mcpv1alpha1 "github.com/Kuadrant/mcp-gateway/api/v1alpha1"
)

// TestResourcesBuilder is a unified builder for creating test resources
type TestResourcesBuilder struct {
	k8sClient       client.Client
	testName        string
	namespace       string
	hostname        string
	serviceName     string
	port            int32
	toolPrefix      string
	path            string
	credential      *corev1.Secret
	credentialKey   string
	httpRoute       *gatewayapiv1.HTTPRoute
	mcpServer       *mcpv1alpha1.MCPServerRegistration
	serviceEntry    *istionetv1beta1.ServiceEntry
	destinationRule *istionetv1beta1.DestinationRule
	isExternal      bool
}

// NewTestResources creates a new TestResourcesBuilder with defaults for internal services
func NewTestResources(testName string, k8sClient client.Client) *TestResourcesBuilder {
	return &TestResourcesBuilder{
		k8sClient:     k8sClient,
		testName:      testName,
		namespace:     TestNamespace,
		hostname:      "e2e-server2.mcp.local",
		serviceName:   "mcp-test-server2",
		port:          9090,
		credentialKey: "token",
	}
}

// NewTestResourcesWithDefaults creates a builder with default backend (mcp-test-server2)
func NewTestResourcesWithDefaults(testName string, k8sClient client.Client) *TestResourcesBuilder {
	return NewTestResources(testName, k8sClient)
}

// ForInternalService configures the builder for an internal Kubernetes service
func (b *TestResourcesBuilder) ForInternalService(serviceName string, port int32) *TestResourcesBuilder {
	b.serviceName = serviceName
	b.port = port
	b.hostname = fmt.Sprintf("%s.mcp.local", serviceName)
	b.isExternal = false
	return b
}

// ForExternalService configures the builder for an external service
func (b *TestResourcesBuilder) ForExternalService(externalHost string, port int32) *TestResourcesBuilder {
	b.serviceName = externalHost
	b.port = port
	b.hostname = fmt.Sprintf("e2e-external-%s.mcp.local", b.testName)
	b.isExternal = true
	return b
}

// WithHostname sets a custom hostname
func (b *TestResourcesBuilder) WithHostname(hostname string) *TestResourcesBuilder {
	b.hostname = hostname
	return b
}

// WithToolPrefix sets the tool prefix for the MCPServerRegistration
func (b *TestResourcesBuilder) WithToolPrefix(prefix string) *TestResourcesBuilder {
	b.toolPrefix = prefix
	return b
}

// WithPath sets a custom MCP path
func (b *TestResourcesBuilder) WithPath(path string) *TestResourcesBuilder {
	b.path = path
	return b
}

// WithBackendTarget sets the backend service name and port for internal services
func (b *TestResourcesBuilder) WithBackendTarget(serviceName string, port int32) *TestResourcesBuilder {
	b.serviceName = serviceName
	b.port = port
	b.hostname = fmt.Sprintf("%s.mcp.local", serviceName)
	b.isExternal = false
	return b
}

// WithCredential sets the credential secret
func (b *TestResourcesBuilder) WithCredential(secret *corev1.Secret, key string) *TestResourcesBuilder {
	b.credential = secret
	b.credentialKey = key
	return b
}

// Build constructs all the resources based on configuration. Must be called before GetObjects() or Register().
func (b *TestResourcesBuilder) Build() *TestResourcesBuilder {
	routeName := UniqueName("e2e-route-" + b.testName)

	if b.isExternal {
		b.buildExternalResources(routeName)
	} else {
		b.buildInternalResources(routeName)
	}

	// build MCPServerRegistration
	b.mcpServer = &mcpv1alpha1.MCPServerRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UniqueName("e2e-mcp-" + b.testName),
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test", "test": b.testName},
		},
		Spec: mcpv1alpha1.MCPServerRegistrationSpec{
			ToolPrefix: b.toolPrefix,
			Path:       b.path,
			TargetRef: mcpv1alpha1.TargetReference{
				Group: "gateway.networking.k8s.io",
				Kind:  "HTTPRoute",
				Name:  routeName,
			},
		},
	}

	if b.credential != nil {
		b.mcpServer.Spec.CredentialRef = &mcpv1alpha1.SecretReference{
			Name: b.credential.Name,
			Key:  b.credentialKey,
		}
	}
	return b
}

func (b *TestResourcesBuilder) buildInternalResources(routeName string) {
	gatewayNamespace := "gateway-system"
	b.httpRoute = &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: gatewayapiv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{
				ParentRefs: []gatewayapiv1.ParentReference{
					{
						Name:      "mcp-gateway",
						Namespace: (*gatewayapiv1.Namespace)(&gatewayNamespace),
					},
				},
			},
			Hostnames: []gatewayapiv1.Hostname{
				gatewayapiv1.Hostname(b.hostname),
				gatewayapiv1.Hostname(strings.Replace(b.hostname, ".mcp.local", ".127-0-0-1.sslip.io", 1)),
			},
			Rules: []gatewayapiv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayapiv1.HTTPBackendRef{
						{
							BackendRef: gatewayapiv1.BackendRef{
								BackendObjectReference: gatewayapiv1.BackendObjectReference{
									Name: gatewayapiv1.ObjectName(b.serviceName),
									Port: (*gatewayapiv1.PortNumber)(&b.port),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (b *TestResourcesBuilder) buildExternalResources(routeName string) {
	externalHost := b.serviceName
	gatewayNamespace := "gateway-system"
	istioGroup := gatewayapiv1.Group("networking.istio.io")
	hostnameKind := gatewayapiv1.Kind("Hostname")
	portNum := gatewayapiv1.PortNumber(b.port)

	b.serviceEntry = &istionetv1beta1.ServiceEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UniqueName("e2e-se-" + b.testName),
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: istiov1beta1.ServiceEntry{
			Hosts: []string{externalHost},
			Ports: []*istiov1beta1.ServicePort{
				{
					Number:   uint32(b.port),
					Name:     "http",
					Protocol: "HTTP",
				},
			},
			Location:   istiov1beta1.ServiceEntry_MESH_EXTERNAL,
			Resolution: istiov1beta1.ServiceEntry_DNS,
		},
	}

	b.destinationRule = &istionetv1beta1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UniqueName("e2e-dr-" + b.testName),
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: istiov1beta1.DestinationRule{
			Host: externalHost,
			TrafficPolicy: &istiov1beta1.TrafficPolicy{
				Tls: &istiov1beta1.ClientTLSSettings{
					Mode: istiov1beta1.ClientTLSSettings_DISABLE,
				},
			},
		},
	}

	b.httpRoute = &gatewayapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: gatewayapiv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayapiv1.CommonRouteSpec{
				ParentRefs: []gatewayapiv1.ParentReference{
					{
						Name:      "mcp-gateway",
						Namespace: (*gatewayapiv1.Namespace)(&gatewayNamespace),
					},
				},
			},
			Hostnames: []gatewayapiv1.Hostname{
				gatewayapiv1.Hostname(b.hostname),
			},
			Rules: []gatewayapiv1.HTTPRouteRule{
				{
					Matches: []gatewayapiv1.HTTPRouteMatch{
						{
							Path: &gatewayapiv1.HTTPPathMatch{
								Type:  ptrTo(gatewayapiv1.PathMatchPathPrefix),
								Value: ptrTo("/mcp"),
							},
						},
					},
					Filters: []gatewayapiv1.HTTPRouteFilter{
						{
							Type: gatewayapiv1.HTTPRouteFilterURLRewrite,
							URLRewrite: &gatewayapiv1.HTTPURLRewriteFilter{
								Hostname: (*gatewayapiv1.PreciseHostname)(&externalHost),
							},
						},
					},
					BackendRefs: []gatewayapiv1.HTTPBackendRef{
						{
							BackendRef: gatewayapiv1.BackendRef{
								BackendObjectReference: gatewayapiv1.BackendObjectReference{
									Group: &istioGroup,
									Kind:  &hostnameKind,
									Name:  gatewayapiv1.ObjectName(externalHost),
									Port:  &portNum,
								},
							},
						},
					},
				},
			},
		},
	}
}

// Register creates all resources in the cluster and returns the MCPServerRegistration.
// Build() must be called before Register().
func (b *TestResourcesBuilder) Register(ctx context.Context) *mcpv1alpha1.MCPServerRegistration {
	if b.credential != nil {
		GinkgoWriter.Println("creating credential", b.credential.Name)
		Expect(b.k8sClient.Create(ctx, b.credential)).To(Succeed())
	}

	if b.serviceEntry != nil {
		GinkgoWriter.Println("creating ServiceEntry", b.serviceEntry.Name)
		Expect(b.k8sClient.Create(ctx, b.serviceEntry)).To(Succeed())
	}

	if b.destinationRule != nil {
		GinkgoWriter.Println("creating DestinationRule", b.destinationRule.Name)
		Expect(b.k8sClient.Create(ctx, b.destinationRule)).To(Succeed())
	}

	GinkgoWriter.Println("creating HTTPRoute", b.httpRoute.Name)
	Expect(b.k8sClient.Create(ctx, b.httpRoute)).To(Succeed())

	GinkgoWriter.Println("creating MCPServerRegistration", b.mcpServer.Name)
	Expect(b.k8sClient.Create(ctx, b.mcpServer)).To(Succeed())

	return b.mcpServer
}

// GetObjects returns all objects that will be created.
// Build() must be called before GetObjects().
func (b *TestResourcesBuilder) GetObjects() []client.Object {
	objects := []client.Object{}
	if b.mcpServer != nil {
		objects = append(objects, b.mcpServer)
	}
	if b.credential != nil {
		objects = append(objects, b.credential)
	}
	if b.httpRoute != nil {
		objects = append(objects, b.httpRoute)
	}
	if b.serviceEntry != nil {
		objects = append(objects, b.serviceEntry)
	}
	if b.destinationRule != nil {
		objects = append(objects, b.destinationRule)
	}
	return objects
}

// GetHTTPRouteName returns the name of the HTTPRoute
func (b *TestResourcesBuilder) GetHTTPRouteName() string {
	if b.httpRoute != nil {
		return b.httpRoute.Name
	}
	return ""
}

// GetMCPServer returns the MCPServerRegistration (after build)
func (b *TestResourcesBuilder) GetMCPServer() *mcpv1alpha1.MCPServerRegistration {
	return b.mcpServer
}

// MCPVirtualServerBuilder builds MCPVirtualServer resources
type MCPVirtualServerBuilder struct {
	name        string
	namespace   string
	description string
	tools       []string
}

// NewMCPVirtualServerBuilder creates a new MCPVirtualServerBuilder
func NewMCPVirtualServerBuilder(name, namespace string) *MCPVirtualServerBuilder {
	return &MCPVirtualServerBuilder{
		name:      name,
		namespace: namespace,
	}
}

// WithDescription sets the description
func (b *MCPVirtualServerBuilder) WithDescription(desc string) *MCPVirtualServerBuilder {
	b.description = desc
	return b
}

// WithTools sets the tools list
func (b *MCPVirtualServerBuilder) WithTools(tools []string) *MCPVirtualServerBuilder {
	b.tools = tools
	return b
}

// Build creates the MCPVirtualServer resource
func (b *MCPVirtualServerBuilder) Build() *mcpv1alpha1.MCPVirtualServer {
	return &mcpv1alpha1.MCPVirtualServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      UniqueName(b.name),
			Namespace: b.namespace,
		},
		Spec: mcpv1alpha1.MCPVirtualServerSpec{
			Description: b.description,
			Tools:       b.tools,
		},
	}
}

// BuildCredentialSecret creates a credential secret for testing
func BuildCredentialSecret(name, token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: TestNamespace,
			Labels: map[string]string{
				"mcp.kagenti.com/credential": "true",
				"e2e":                        "test",
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"token": fmt.Sprintf("Bearer %s", token),
		},
	}
}

func ptrTo[T any](v T) *T {
	return &v
}

// MCPGatewayExtensionBuilder builds MCPGatewayExtension resources
type MCPGatewayExtensionBuilder struct {
	name            string
	namespace       string
	targetGateway   string
	targetNamespace string
}

// NewMCPGatewayExtensionBuilder creates a new MCPGatewayExtensionBuilder
func NewMCPGatewayExtensionBuilder(name, namespace string) *MCPGatewayExtensionBuilder {
	return &MCPGatewayExtensionBuilder{
		name:      name,
		namespace: namespace,
	}
}

// WithTarget sets the target Gateway reference
func (b *MCPGatewayExtensionBuilder) WithTarget(gatewayName, gatewayNamespace string) *MCPGatewayExtensionBuilder {
	b.targetGateway = gatewayName
	b.targetNamespace = gatewayNamespace
	return b
}

// Build creates the MCPGatewayExtension resource
func (b *MCPGatewayExtensionBuilder) Build() *mcpv1alpha1.MCPGatewayExtension {
	return &mcpv1alpha1.MCPGatewayExtension{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: mcpv1alpha1.MCPGatewayExtensionSpec{
			TargetRef: mcpv1alpha1.MCPGatewayExtensionTargetReference{
				Group:     "gateway.networking.k8s.io",
				Kind:      "Gateway",
				Name:      b.targetGateway,
				Namespace: b.targetNamespace,
			},
		},
	}
}

// ReferenceGrantBuilder builds ReferenceGrant resources for cross-namespace references
type ReferenceGrantBuilder struct {
	name          string
	namespace     string
	fromNamespace string
	fromKind      string
	fromGroup     string
	toKind        string
	toGroup       string
}

// NewReferenceGrantBuilder creates a new ReferenceGrantBuilder
func NewReferenceGrantBuilder(name, namespace string) *ReferenceGrantBuilder {
	return &ReferenceGrantBuilder{
		name:      name,
		namespace: namespace,
		fromGroup: "mcp.kagenti.com",
		fromKind:  "MCPGatewayExtension",
		toGroup:   "gateway.networking.k8s.io",
		toKind:    "Gateway",
	}
}

// FromNamespace sets the namespace allowed to reference resources in this namespace
func (b *ReferenceGrantBuilder) FromNamespace(ns string) *ReferenceGrantBuilder {
	b.fromNamespace = ns
	return b
}

// Build creates the ReferenceGrant resource
func (b *ReferenceGrantBuilder) Build() *gatewayv1beta1.ReferenceGrant {
	return &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
			Labels:    map[string]string{"e2e": "test"},
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.Group(b.fromGroup),
					Kind:      gatewayv1beta1.Kind(b.fromKind),
					Namespace: gatewayv1beta1.Namespace(b.fromNamespace),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayv1beta1.Group(b.toGroup),
					Kind:  gatewayv1beta1.Kind(b.toKind),
				},
			},
		},
	}
}

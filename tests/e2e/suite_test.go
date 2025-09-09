//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	mcpv1alpha1 "github.com/kagenti/mcp-gateway/pkg/apis/mcp/v1alpha1"
)

var (
	k8sClient  client.Client
	testScheme *runtime.Scheme
	cfg        *rest.Config
	ctx        context.Context
	cancel     context.CancelFunc
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Gateway E2E Suite")
}

var _ = BeforeSuite(func() {
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("Setting up test scheme")
	testScheme = runtime.NewScheme()
	Expect(scheme.AddToScheme(testScheme)).To(Succeed())
	Expect(mcpv1alpha1.AddToScheme(testScheme)).To(Succeed())
	Expect(gatewayapiv1.Install(testScheme)).To(Succeed())

	By("Getting kubeconfig")
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).ToNot(HaveOccurred())

	By("Creating Kubernetes client")
	k8sClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).ToNot(HaveOccurred())

	By("Verifying cluster connection")
	namespaceList := &corev1.NamespaceList{}
	Expect(k8sClient.List(ctx, namespaceList)).To(Succeed())

	By("Checking test namespace exists")
	testNs := &corev1.Namespace{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: TestNamespace}, testNs)
	if err != nil {
		GinkgoWriter.Printf("Warning: test namespace %s does not exist, tests may fail\n", TestNamespace)
	}

	By("Checking system namespace exists")
	systemNs := &corev1.Namespace{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: SystemNamespace}, systemNs)
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("System namespace %s must exist", SystemNamespace))
})

var _ = AfterSuite(func() {
	By("Tearing down the test environment")
	if cancel != nil {
		cancel()
	}
})

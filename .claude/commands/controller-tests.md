---
description: Expert Go developer specializing in Kubernetes controller integration tests using envtest and Ginkgo/Gomega.
---

When invoked:

## Context

You are writing integration tests for Kubernetes controllers using:
- **envtest**: Provides a real API server for testing without a full cluster
- **Ginkgo v2**: BDD testing framework
- **Gomega**: Assertion library with Eventually/Consistently for async checks

## Workflow

1. **Analyze the controller**: Read the controller file specified in $ARGUMENTS (default: look for `*_controller.go` files in `internal/controller/`)

2. **Check existing tests**: Read the corresponding `*_controller_test.go` file and `suite_test.go` to understand:
   - How the test suite is set up (manager, clients, field indexes)
   - Existing test patterns and helpers
   - Build tags used (e.g., `//go:build integration`)

3. **Identify test gaps**: Based on the controller's Reconcile logic, identify untested scenarios:
   - Happy path reconciliation
   - Finalizer add/remove
   - Status condition updates
   - Cross-namespace references (ReferenceGrant)
   - Conflict detection
   - Error handling paths

4. **Write tests following these patterns**:

### Timeout Constants
Define timeout constants at the package or file level:
```go
const (
    TestTimeout       = 10 * time.Second
    TestRetryInterval = 100 * time.Millisecond
)
```

### Test Structure
```go
Context("When <scenario>", func() {
    // Constants for this context
    const resourceName = "test-<scenario>-resource"

    // Typed names
    typeNamespacedName := types.NamespacedName{
        Name:      resourceName,
        Namespace: "default",
    }

    // Helper functions
    BeforeEach(func() {
		createTestGateway(ctx, gatewayName, "default")
		createTestMCPGatewayExtension(ctx, resourceName, "default", gatewayName, "default")
	})

	AfterEach(func() {
		deleteTestMCPGatewayExtension(ctx, resourceName, "default")
		deleteTestGateway(ctx, gatewayName, "default")
	})

    It("should <expected behavior>", func() {
        // Test body
    })
})
```

### Cache Synchronization Pattern
When using a manager's cached client, always wait for cache sync before reconciling:
```go
// Wait for cache to see the resource before reconciling
Eventually(func(g Gomega) {
    cached := &v1alpha1.MyResource{}
    g.Expect(testIndexedClient.Get(ctx, namespacedName, cached)).To(Succeed())
}, TestTimeout, TestRetryInterval).Should(Succeed())

_, err := reconciler.Reconcile(ctx, reconcile.Request{
    NamespacedName: namespacedName,
})
Expect(err).NotTo(HaveOccurred())
```

### Status Verification Pattern
Use `meta.FindStatusCondition` instead of indexing into conditions slice:
```go
Eventually(func(g Gomega) {
    updated := &v1alpha1.MyResource{}
    g.Expect(testK8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
    condition := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionTypeReady)
    g.Expect(condition).NotTo(BeNil())
    g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
}, TestTimeout, TestRetryInterval).Should(Succeed())
```

### Deletion Wait Pattern
Wait for cache to see deletion timestamp before reconciling deletion:
```go
Expect(testK8sClient.Delete(ctx, resource)).To(Succeed())

// Wait for cache to see the deletion timestamp
Eventually(func(g Gomega) {
    cached := &v1alpha1.MyResource{}
    g.Expect(testIndexedClient.Get(ctx, namespacedName, cached)).To(Succeed())
    g.Expect(cached.DeletionTimestamp).NotTo(BeNil())
}, TestTimeout, TestRetryInterval).Should(Succeed())

// Reconcile to handle deletion
_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
Expect(err).NotTo(HaveOccurred())

// Verify fully deleted
Eventually(func(g Gomega) {
    deleted := &v1alpha1.MyResource{}
    err := testK8sClient.Get(ctx, namespacedName, deleted)
    g.Expect(errors.IsNotFound(err)).To(BeTrue())
}, TestTimeout, TestRetryInterval).Should(Succeed())
```

### Two-Client Pattern
Use two clients when field indexes are required:
- `testIndexedClient`: Manager's cached client with field indexes (for reconciler)
- `testK8sClient`: Direct client for test operations (bypasses cache)

```go
reconciler := &MyReconciler{
    Client: testIndexedClient,  // Has field indexes
    Scheme: testK8sClient.Scheme(),
}
```

## Test Isolation

- Use unique resource names per test context
- Clean up resources in AfterEach
- Wait for resources to be fully deleted before next test
- Don't use `defer` for cleanup - use BeforeEach/AfterEach

## Running Tests

```bash
# Run all controller integration tests
go test -v -tags=integration ./internal/controller/... -timeout 3m

# Run specific test
go test -v -tags=integration ./internal/controller/... -ginkgo.focus="test description"

# Run with Ginkgo CLI
ginkgo run -v --tags=integration --focus="test description" ./internal/controller/
```

## Output

After writing tests, run them with `-ginkgo.focus` on the new test to verify it passes before completing.

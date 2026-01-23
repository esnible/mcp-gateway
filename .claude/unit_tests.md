# Unit Testing Guidelines

## Philosophy

Tests should cover main functionality but don't need to be exhaustive. Focus on:
- Happy path scenarios
- Key error conditions
- Edge cases that are likely to cause bugs
- Behavior that documents how the code works

Avoid testing trivial getters/setters or implementation details that may change.

## Table-Driven Tests

Where possible, use table-driven tests. This approach:
- Makes it easy to add new test cases
- Reduces code duplication
- Makes test intent clear
- Provides consistent structure

### Example Structure

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "descriptive case name",
            input:   someInput,
            want:    expectedOutput,
            wantErr: false,
        },
        {
            name:    "error case",
            input:   badInput,
            want:    zero,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionUnderTest(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionUnderTest() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("FunctionUnderTest() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Naming Conventions

- Test functions: `TestTypeName_MethodName` or `TestFunctionName`
- Test cases: Use lowercase descriptive names that explain the scenario
- Examples: `"valid input"`, `"returns error when empty"`, `"handles nil gracefully"`

## Test Organization

- Place tests in `*_test.go` files alongside the code they test
- Group related test cases in the same table
- Use separate test functions for different methods or behaviors

## Mocking

- Use interfaces to enable mocking
- Prefer simple mock implementations over complex mocking frameworks
- Mock only external dependencies (HTTP clients, databases, Kubernetes API)

## Kubernetes Controller Tests

For controller tests:
- Use `envtest` for integration tests that need a real API server
- Use fake client (`fake.NewClientBuilder()`) for unit tests
- Test reconciliation logic with various resource states

## Running Tests

```bash
# Run all unit tests
make test-unit

# Run specific test
go test -v ./path/to/package -run TestName

# Run with race detection
go test -race ./...
```

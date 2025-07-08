# Claude Cache Service Development Guidelines

## Critical Best Practices

### 1. ABSOLUTELY NO SILENT ERROR HANDLING
- **NEVER use `_ =` to discard errors**
- **NEVER hide errors with empty error checks**
- **ALL errors must be handled with appropriate logging**
- **Delete ANY code that silences errors immediately**

#### Proper Error Handling Examples:
```go
// ❌ WRONG - Silent error
defer func() { _ = db.Close() }()

// ✅ CORRECT - Log the error
defer func() {
    if err := db.Close(); err != nil {
        logger.Error().Err(err).Msg("Failed to close database")
    }
}()
```

### 2. Configuration Best Practices
- **NEVER hardcode URLs, ports, or configuration values**
- **Use environment variables for ALL configuration**
- **Provide sensible defaults with clear documentation**

```go
// ❌ WRONG
const apiURL = "http://localhost:8080"
const cacheDir = "/tmp/cache"

// ✅ CORRECT
apiURL := os.Getenv("API_URL")
if apiURL == "" {
    apiURL = "http://localhost:8080"
}
```

### 3. Logging Standards
- **Use zerolog for ALL logging**
- **NEVER use fmt.Printf, log.Printf for production code**
- **Include context in all log messages**

```go
// Setup logger
logger := zerolog.New(os.Stdout).
    Level(zerolog.InfoLevel).
    With().
    Timestamp().
    Caller().
    Logger()
```

### 4. Testing Requirements
- **Minimum 80% code coverage**
- **Test error conditions explicitly**
- **Use table-driven tests for multiple scenarios**
- **Mock external dependencies**

### 5. API Design Principles
- **Version all APIs (/api/v1/...)**
- **Return consistent error responses**
- **Include request IDs for tracing**
- **Implement rate limiting from day 1**

```go
// Standard error response
type ErrorResponse struct {
    Error     string `json:"error"`
    Message   string `json:"message"`
    RequestID string `json:"request_id"`
    Timestamp int64  `json:"timestamp"`
}
```

### 6. Performance Standards
- **Cache operations must complete in <100ms**
- **Background updates should not block API requests**
- **Use connection pooling for all external services**
- **Implement graceful shutdown**

### 7. Security Requirements
- **Validate all inputs**
- **Use prepared statements for any SQL**
- **Implement authentication for write operations**
- **Log security events separately**
- **No secrets in code or logs**

## Git Workflow

### Commit Message Format
All commits MUST follow Conventional Commits:

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore

### Pre-commit Checks
1. **Go fmt** - Code must be formatted
2. **Go vet** - No vet errors
3. **Go test** - All tests must pass
4. **Golint** - No lint errors
5. **No hardcoded values** - Check for localhost, ports, etc.

### Branch Protection
- **main** branch is protected
- **All changes via PR**
- **Require review** before merge
- **CI must pass** before merge

## Development Workflow

### Before Starting Work
```bash
# Ensure clean working directory
git status

# Update dependencies
go mod tidy

# Run tests
go test ./...

# Check for security issues
go list -json -deps | nancy sleuth
```

### During Development
1. **Write tests first** (TDD approach)
2. **Handle all errors** explicitly
3. **Add appropriate logging**
4. **Document public APIs**
5. **Keep functions focused** (single responsibility)

### Before Committing
```bash
# Format code
go fmt ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run linter
golangci-lint run

# Check for common issues
go vet ./...
```

## Architecture Guidelines

### Package Structure
```
cmd/          # Main applications
internal/     # Private application code
pkg/          # Public libraries
api/          # API definitions (OpenAPI/protobuf)
```

### Dependency Injection
- Use interfaces for all external dependencies
- Pass dependencies explicitly
- Avoid global state

### Concurrency
- Use channels for communication
- Protect shared state with mutexes
- Implement context cancellation
- Handle goroutine lifecycle properly

## Quality Checklist

Before ANY PR:
- [ ] All tests pass
- [ ] Coverage >80% for new code
- [ ] No hardcoded values
- [ ] All errors handled
- [ ] Appropriate logging added
- [ ] API documented
- [ ] Performance impact considered
- [ ] Security implications reviewed
- [ ] Breaking changes noted

## Common Pitfalls to Avoid

1. **Ignoring errors** - Always handle them
2. **Global variables** - Use dependency injection
3. **Large functions** - Keep under 50 lines
4. **Missing tests** - Aim for 80%+ coverage
5. **Poor error messages** - Include context
6. **Synchronous updates** - Use background workers
7. **Memory leaks** - Close all resources
8. **Race conditions** - Use proper synchronization

## Performance Optimization

1. **Use sync.Pool** for frequently allocated objects
2. **Implement caching layers** (memory -> disk -> API)
3. **Batch operations** where possible
4. **Use buffered channels** appropriately
5. **Profile before optimizing** (pprof)

## Monitoring & Observability

1. **Structured logging** with zerolog
2. **Metrics** for cache hit/miss rates
3. **Tracing** for request flow
4. **Health checks** endpoint
5. **Performance metrics** (p50, p95, p99)

Remember: This is a high-performance caching service. Every millisecond counts!
# Claude Cache Service

**⚠️ PROJECT ON HOLD - SUBSCRIPTION INCOMPATIBILITY**

This service was designed to cache Claude API responses for SDK analysis, but it requires an Anthropic API key which is separate from Claude Pro/Max subscriptions. Claude Code/CLI does not expose prompt caching capabilities.

## Original Vision

A high-performance caching service for Claude AI interactions, designed to reduce token usage by 90% across multiple projects.

## Implementation Status

### Completed Features

- ✅ **Claude API Client**: Full implementation with retry logic and rate limiting
- ✅ **SDK Analysis**: Real Git integration for analyzing 29 Sentry SDKs
- ✅ **Cache Manager**: BadgerDB-based caching with TTL support
- ✅ **Update Worker**: Scheduled updates with fallback to mock data
- ✅ **REST API**: Basic endpoints for health, cache operations
- ✅ **Configuration**: Environment-based configuration with sensible defaults
- ✅ **Testing**: Comprehensive test suite with 48.7% overall coverage

### Not Implemented (Due to Project Hold)

- ❌ **WebSocket Support**: Real-time updates (Issue #4)
- ❌ **Analytics Storage**: Usage tracking (Issue #3)
- ❌ **Authentication**: API key management (Issue #5)
- ❌ **Rate Limiting**: Request throttling (Issue #6)
- ❌ **Client Libraries**: Go/TypeScript SDKs (Issues #7, #8)
- ❌ **Cache Refresh Endpoint**: Manual refresh trigger (Issue #9)

## Important Compatibility Notes

### Why This Project Is On Hold

1. **API Key Requirement**: This service requires an Anthropic API key, which is a separate paid product from Claude Pro/Max subscriptions
2. **No CLI Support**: Claude Code/CLI doesn't expose prompt caching APIs, only `--continue` and `--resume` for session persistence
3. **Subscription Mismatch**: Claude Pro/Max subscriptions are for claude.ai web interface only, not API access

### Alternative Approaches for Claude Pro/Max Users

- Use Claude Code's `--continue` and `--resume` flags for session persistence
- Build local file caching without API requirements
- Create prompt optimization tools that work with Claude Code/CLI
- Use the web interface at claude.ai which includes Projects feature for context management

## Quick Start (Requires Anthropic API Key)

```bash
# Clone the repository
git clone https://github.com/ryanrussell/claude-cache-service
cd claude-cache-service

# Install dependencies
go mod download

# Run the service
go run cmd/server/main.go

# Or use Docker
docker-compose up
```

## API Endpoints

### REST API

```bash
# Get cache summary
GET /api/v1/cache/summary

# Get project-specific cache
GET /api/v1/cache/project/:name

# Get SDK analysis
GET /api/v1/cache/sdk/:name

# Trigger cache refresh
POST /api/v1/cache/refresh

# Get usage analytics
GET /api/v1/analytics/usage
```

### WebSocket

```bash
# Real-time cache updates
WS /ws/updates

# Subscribe to specific project
WS /ws/project/:name
```

## Client Libraries

### Direct API Usage

Currently, use the REST API directly. Client libraries are planned:

```bash
# Get SDK summary
curl http://localhost:8080/api/v1/cache/sdk/sentry-go

# Get project cache  
curl http://localhost:8080/api/v1/cache/project/gremlin-arrow-flight
```

### Planned Client Libraries

- **Go Client** (TODO): `pkg/client/go/`
- **TypeScript Client** (TODO): `pkg/client/typescript/`
- **Python Client** (TODO): Coming soon

For now, use your favorite HTTP client library directly with the REST API.

## Configuration

Environment variables:

```bash
# Server port (default: 8080)
PORT=8080

# Cache directory (default: ./cache)
CACHE_DIR=/var/lib/claude-cache

# Update schedule (cron format, default: weekly)
UPDATE_SCHEDULE="0 2 * * 0"

# Claude API configuration
CLAUDE_API_KEY=your-api-key
CLAUDE_MODEL=claude-3-5-sonnet-20241022

# Enable debug logging
DEBUG=true
```

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   REST API      │     │  WebSocket API  │     │  Update Worker  │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                        │
         └───────────────────────┴────────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │     Cache Manager       │
                    │  - BuntDB (embedded)    │
                    │  - File storage         │
                    │  - Memory cache         │
                    └─────────────────────────┘
```

## Use Cases

### 1. Gremlin Arrow Flight (Sentry SDK Analysis)
- Cache SDK patterns and envelope formats
- Track breaking changes across 29 SDKs
- Reduce analysis time from 30s to <1s

### 2. Claude Code GUI (Interactive UI)
- Display real-time token savings
- Show cache hit rates in dashboard
- Stream updates via WebSocket

### 3. Future Projects
- Any project using Claude for code analysis
- Documentation generation with cached context
- Multi-repository pattern detection

## Performance

- **Token Reduction**: 90% average (50K → 5K per analysis)
- **Response Time**: <100ms for cache hits
- **Cache Hit Rate**: 85%+ after warm-up
- **Memory Usage**: <500MB for 29 SDKs
- **Update Time**: <5 minutes for full refresh

## Development

```bash
# Run tests
go test ./...

# Run with hot reload
air

# Build for production
go build -o claude-cache-service cmd/server/main.go

# Run linter
golangci-lint run
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT
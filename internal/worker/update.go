package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/ryanrussell/claude-cache-service/internal/analyzer"
	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/config"
)

// UpdateWorker handles scheduled cache updates.
type UpdateWorker struct {
	cache    *cache.Manager
	logger   zerolog.Logger
	config   *config.Config
	cron     *cron.Cron
	analyzer analyzer.Analyzer
}

// NewUpdateWorker creates a new update worker.
func NewUpdateWorker(cache *cache.Manager, logger zerolog.Logger, config *config.Config) *UpdateWorker {
	// Create analyzer if Claude API key is configured
	var sdkAnalyzer analyzer.Analyzer
	if config.ClaudeAPIKey != "" {
		sdkAnalyzer = analyzer.NewClaudeAnalyzer(config.ClaudeAPIKey, config.ClaudeModel, logger)
		logger.Info().Msg("Claude analyzer initialized")
	} else {
		logger.Warn().Msg("Claude API key not configured, using mock analyzer")
		sdkAnalyzer = &mockAnalyzer{logger: logger}
	}

	return &UpdateWorker{
		cache:    cache,
		logger:   logger,
		config:   config,
		cron:     cron.New(cron.WithLogger(cron.VerbosePrintfLogger(&cronLogger{logger: logger}))),
		analyzer: sdkAnalyzer,
	}
}

// Start starts the update worker.
func (w *UpdateWorker) Start(ctx context.Context) {
	w.logger.Info().Str("schedule", w.config.UpdateSchedule).Msg("Starting update worker")

	// Add scheduled job
	_, err := w.cron.AddFunc(w.config.UpdateSchedule, func() {
		if err := w.updateCache(ctx); err != nil {
			w.logger.Error().Err(err).Msg("Failed to update cache")
		}
	})
	if err != nil {
		w.logger.Error().Err(err).Msg("Failed to add cron job")
		return
	}

	// Run initial update
	go func() {
		w.logger.Info().Msg("Running initial cache update")
		if err := w.updateCache(ctx); err != nil {
			w.logger.Error().Err(err).Msg("Failed to run initial cache update")
		}
	}()

	// Start cron scheduler
	w.cron.Start()

	// Wait for context cancellation
	<-ctx.Done()
	w.logger.Info().Msg("Stopping update worker")

	// Stop cron scheduler
	cronCtx := w.cron.Stop()
	<-cronCtx.Done()
}

// updateCache performs the cache update.
func (w *UpdateWorker) updateCache(ctx context.Context) error {
	start := time.Now()
	w.logger.Info().Msg("Starting cache update")

	// SDK list to analyze
	sampleSDKs := []struct {
		name    string
		version string
		// In real implementation, this would contain actual SDK code files
		codeFiles map[string]string
	}{
		{
			name:    "sentry-go",
			version: "0.25.0",
			codeFiles: map[string]string{
				"transport.go": "package sentry\n\n// Transport interface",
				"client.go":    "package sentry\n\n// Client struct",
			},
		},
		{
			name:    "sentry-python",
			version: "1.40.0",
			codeFiles: map[string]string{
				"transport.py": "class HTTPTransport:\n    pass",
				"client.py":    "class Client:\n    pass",
			},
		},
		{
			name:    "sentry-javascript",
			version: "7.92.0",
			codeFiles: map[string]string{
				"transport.js": "export class Transport {}",
				"client.js":    "export class Client {}",
			},
		},
	}

	// Analyze SDKs
	for _, sdk := range sampleSDKs {
		select {
		case <-ctx.Done():
			return fmt.Errorf("update cancelled")
		default:
			// Prepare analysis request
			request := analyzer.AnalysisRequest{
				SDKName:    sdk.name,
				Version:    sdk.version,
				Code:       sdk.codeFiles,
				CommitHash: "latest", // In real implementation, get actual commit hash
			}

			// Analyze SDK
			w.logger.Info().
				Str("sdk", sdk.name).
				Str("version", sdk.version).
				Msg("Analyzing SDK")

			analysis, err := w.analyzer.AnalyzeCode(ctx, request)
			if err != nil {
				w.logger.Error().Err(err).Str("sdk", sdk.name).Msg("Failed to analyze SDK")
				continue
			}

			// Convert analysis to JSON for caching
			analysisJSON, err := json.Marshal(analysis)
			if err != nil {
				w.logger.Error().Err(err).Str("sdk", sdk.name).Msg("Failed to marshal analysis")
				continue
			}

			// Cache the analysis
			key := fmt.Sprintf("sdk:%s", sdk.name)
			if err := w.cache.Set(key, string(analysisJSON), w.config.CacheTTL); err != nil {
				w.logger.Error().Err(err).Str("sdk", sdk.name).Msg("Failed to cache SDK analysis")
			} else {
				w.logger.Info().
					Str("sdk", sdk.name).
					Int("tokens_used", analysis.TokensUsed).
					Msg("SDK analysis cached")
			}

			// Also cache version-specific analysis
			versionKey := fmt.Sprintf("sdk:%s:%s", sdk.name, sdk.version)
			if err := w.cache.Set(versionKey, string(analysisJSON), w.config.CacheTTL); err != nil {
				w.logger.Error().Err(err).Str("key", versionKey).Msg("Failed to cache version-specific analysis")
			}
		}
	}

	// Cache project summaries (these would be aggregated from actual usage data)
	projects := []string{
		"gremlin-arrow-flight",
		"claude-code-gui",
	}

	for _, project := range projects {
		summary := map[string]interface{}{
			"project":       project,
			"cache_hits":    1000,
			"token_savings": 45000,
			"last_updated":  time.Now().Format(time.RFC3339),
		}

		summaryJSON, err := json.Marshal(summary)
		if err != nil {
			w.logger.Error().Err(err).Str("project", project).Msg("Failed to marshal project summary")
			continue
		}

		key := fmt.Sprintf("project:%s", project)
		if err := w.cache.Set(key, string(summaryJSON), w.config.CacheTTL); err != nil {
			w.logger.Error().Err(err).Str("project", project).Msg("Failed to cache project summary")
		}
	}

	duration := time.Since(start)
	w.logger.Info().Dur("duration", duration).Msg("Cache update completed")

	return nil
}

// cronLogger adapts zerolog for cron logging.
type cronLogger struct {
	logger zerolog.Logger
}

func (l *cronLogger) Printf(format string, v ...interface{}) {
	l.logger.Debug().Msgf(format, v...)
}

// mockAnalyzer provides mock analysis when Claude API is not configured
type mockAnalyzer struct {
	logger zerolog.Logger
}

func (m *mockAnalyzer) AnalyzeCode(ctx context.Context, request analyzer.AnalysisRequest) (*analyzer.SDKAnalysis, error) {
	m.logger.Info().
		Str("sdk", request.SDKName).
		Str("version", request.Version).
		Msg("Using mock analyzer")

	// Return mock analysis data
	return &analyzer.SDKAnalysis{
		Language:       detectLanguage(request.SDKName),
		EnvelopeFormat: "JSON envelope with headers and items array",
		Transport: analyzer.TransportDetails{
			Type:                "http",
			Protocols:           []string{"https"},
			RetryMechanism:      "exponential backoff with jitter",
			QueueImplementation: "in-memory queue with disk overflow",
		},
		EventTypes: []string{"error", "transaction", "profile", "metric"},
		ErrorPatterns: []analyzer.ErrorPattern{
			{
				Name:        "structured_errors",
				Pattern:     "Error{type, message, stacktrace}",
				Description: "Structured error handling with full context",
			},
		},
		Integrations:    []string{"logging", "http", "database"},
		Features:        []string{"breadcrumbs", "attachments", "sessions", "release_tracking"},
		ProtocolVersion: "7",
		CachingPatterns: []analyzer.CachingPattern{
			{
				Type:        "envelope_buffer",
				Location:    "transport",
				Description: "Buffers envelopes during network failures",
			},
		},
		TokensUsed:      0, // Mock analyzer doesn't use tokens
		AnalyzedAt:      time.Now(),
		AnalysisVersion: "mock-1.0.0",
	}, nil
}

func (m *mockAnalyzer) BatchAnalyze(ctx context.Context, requests []analyzer.AnalysisRequest) (*analyzer.BatchAnalysisResult, error) {
	result := &analyzer.BatchAnalysisResult{
		JobID:   fmt.Sprintf("mock-job-%d", time.Now().Unix()),
		Status:  "completed",
		Results: make(map[string]*analyzer.SDKAnalysis),
		Errors:  make(map[string]string),
	}

	for _, req := range requests {
		analysis, err := m.AnalyzeCode(ctx, req)
		if err != nil {
			result.Errors[req.SDKName] = err.Error()
		} else {
			result.Results[req.SDKName] = analysis
		}
	}

	now := time.Now()
	result.CompletedAt = &now
	return result, nil
}

func (m *mockAnalyzer) GetBatchStatus(ctx context.Context, jobID string) (*analyzer.BatchAnalysisResult, error) {
	return nil, fmt.Errorf("batch status not supported in mock analyzer")
}

func (m *mockAnalyzer) CountTokens(ctx context.Context, request analyzer.AnalysisRequest) (int, error) {
	// Mock token count based on code size
	totalChars := 0
	for _, code := range request.Code {
		totalChars += len(code)
	}
	return totalChars / 4, nil // Approximate 4 chars per token
}

func detectLanguage(sdkName string) string {
	switch {
	case contains(sdkName, "python"):
		return "python"
	case contains(sdkName, "javascript"), contains(sdkName, "js"):
		return "javascript"
	case contains(sdkName, "go"):
		return "go"
	case contains(sdkName, "java"):
		return "java"
	case contains(sdkName, "ruby"):
		return "ruby"
	case contains(sdkName, "php"):
		return "php"
	case contains(sdkName, "dotnet"), contains(sdkName, "csharp"):
		return "csharp"
	default:
		return "unknown"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

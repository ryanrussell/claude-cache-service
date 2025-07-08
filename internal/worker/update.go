package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/ryanrussell/claude-cache-service/internal/analyzer"
	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/config"
	"github.com/ryanrussell/claude-cache-service/internal/git"
	"github.com/ryanrussell/claude-cache-service/internal/sdk"
)

// UpdateWorker handles scheduled cache updates.
type UpdateWorker struct {
	cache       *cache.Manager
	logger      zerolog.Logger
	config      *config.Config
	cron        *cron.Cron
	sdkAnalyzer *sdk.Analyzer
}

// NewUpdateWorker creates a new update worker.
func NewUpdateWorker(cache *cache.Manager, logger zerolog.Logger, config *config.Config) *UpdateWorker {
	// Create git client
	gitWorkDir := filepath.Join(config.CacheDir, "repos")
	gitClient := git.NewClient(gitWorkDir, logger)

	// Create analyzer based on configuration
	var claudeAnalyzer analyzer.Analyzer
	if config.ClaudeAPIKey != "" {
		claudeAnalyzer = analyzer.NewClaudeAnalyzer(config.ClaudeAPIKey, config.ClaudeModel, logger)
		logger.Info().Msg("Claude analyzer initialized")
	} else {
		logger.Warn().Msg("Claude API key not configured, using mock analyzer")
		claudeAnalyzer = &mockAnalyzer{logger: logger}
	}

	// Create SDK analyzer
	sdkAnalyzer, err := sdk.NewAnalyzer(gitClient, claudeAnalyzer, cache, logger)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create SDK analyzer")
		// Return worker without SDK analyzer, will use fallback
		return &UpdateWorker{
			cache:       cache,
			logger:      logger,
			config:      config,
			cron:        cron.New(cron.WithLogger(cron.VerbosePrintfLogger(&cronLogger{logger: logger}))),
			sdkAnalyzer: nil,
		}
	}

	return &UpdateWorker{
		cache:       cache,
		logger:      logger,
		config:      config,
		cron:        cron.New(cron.WithLogger(cron.VerbosePrintfLogger(&cronLogger{logger: logger}))),
		sdkAnalyzer: sdkAnalyzer,
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

	// Check if SDK analyzer is available
	if w.sdkAnalyzer == nil {
		w.logger.Warn().Msg("SDK analyzer not available, using fallback")
		return w.updateCacheFallback(ctx)
	}

	// Analyze all active SDKs
	results := w.sdkAnalyzer.AnalyzeAllSDKs(ctx)

	successCount := 0
	errorCount := 0

	// Process results
	for _, result := range results {
		if result.Error != nil {
			w.logger.Error().
				Err(result.Error).
				Str("sdk", result.SDK.Name).
				Msg("Failed to analyze SDK")
			errorCount++
			continue
		}

		// Convert analysis to JSON for caching
		analysisJSON, err := json.Marshal(result.Analysis)
		if err != nil {
			w.logger.Error().
				Err(err).
				Str("sdk", result.SDK.Name).
				Msg("Failed to marshal analysis")
			errorCount++
			continue
		}

		// Cache the analysis
		key := fmt.Sprintf("sdk:%s", result.SDK.Name)
		if err := w.cache.Set(key, string(analysisJSON), w.config.CacheTTL); err != nil {
			w.logger.Error().
				Err(err).
				Str("sdk", result.SDK.Name).
				Msg("Failed to cache SDK analysis")
			errorCount++
		} else {
			w.logger.Info().
				Str("sdk", result.SDK.Name).
				Int("tokens_used", result.Analysis.TokensUsed).
				Msg("SDK analysis cached")
			successCount++
		}

		// Cache version-specific analysis
		versionKey := fmt.Sprintf("sdk:%s:%s", result.SDK.Name, result.Analysis.AnalysisVersion)
		if err := w.cache.Set(versionKey, string(analysisJSON), w.config.CacheTTL); err != nil {
			w.logger.Error().
				Err(err).
				Str("key", versionKey).
				Msg("Failed to cache version-specific analysis")
		}

		// Update last analyzed timestamp
		timestampKey := fmt.Sprintf("sdk:%s:last_analyzed", result.SDK.Name)
		if err := w.cache.Set(timestampKey, time.Now().Format(time.RFC3339), 0); err != nil {
			w.logger.Error().
				Err(err).
				Str("sdk", result.SDK.Name).
				Msg("Failed to update last analyzed timestamp")
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
	w.logger.Info().
		Dur("duration", duration).
		Int("success", successCount).
		Int("errors", errorCount).
		Msg("Cache update completed")

	return nil
}

// updateCacheFallback performs cache update using mock data when SDK analyzer is not available
func (w *UpdateWorker) updateCacheFallback(ctx context.Context) error {
	// Use the original mock implementation
	sampleSDKs := []string{"sentry-go", "sentry-python", "sentry-javascript"}

	for _, sdkName := range sampleSDKs {
		select {
		case <-ctx.Done():
			return fmt.Errorf("update cancelled")
		default:
			// Create mock analysis
			mockAnalyzer := &mockAnalyzer{logger: w.logger}
			request := analyzer.AnalysisRequest{
				SDKName:    sdkName,
				Version:    "1.0.0",
				Code:       map[string]string{"main.file": "// mock code"},
				CommitHash: "mock",
			}

			analysis, err := mockAnalyzer.AnalyzeCode(ctx, request)
			if err != nil {
				w.logger.Error().Err(err).Str("sdk", sdkName).Msg("Failed to analyze SDK")
				continue
			}

			// Convert analysis to JSON for caching
			analysisJSON, err := json.Marshal(analysis)
			if err != nil {
				w.logger.Error().Err(err).Str("sdk", sdkName).Msg("Failed to marshal analysis")
				continue
			}

			// Cache the analysis
			key := fmt.Sprintf("sdk:%s", sdkName)
			if err := w.cache.Set(key, string(analysisJSON), w.config.CacheTTL); err != nil {
				w.logger.Error().Err(err).Str("sdk", sdkName).Msg("Failed to cache SDK analysis")
			} else {
				w.logger.Info().Str("sdk", sdkName).Msg("SDK analysis cached")
			}
		}
	}

	// Cache project summaries
	projects := []string{"gremlin-arrow-flight", "claude-code-gui"}
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

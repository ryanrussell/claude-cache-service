package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/ryanrussell/claude-cache-service/internal/claude"
)

// ClaudeAnalyzer implements the Analyzer interface using Claude API
type ClaudeAnalyzer struct {
	client  *claude.Client
	logger  zerolog.Logger
	version string
}

// NewClaudeAnalyzer creates a new Claude-based analyzer
func NewClaudeAnalyzer(apiKey, model string, logger zerolog.Logger) *ClaudeAnalyzer {
	return &ClaudeAnalyzer{
		client:  claude.NewClient(apiKey, model, logger),
		logger:  logger,
		version: "1.0.0",
	}
}

// AnalyzeCode analyzes a single SDK's code
func (a *ClaudeAnalyzer) AnalyzeCode(ctx context.Context, request AnalysisRequest) (*SDKAnalysis, error) {
	startTime := time.Now()

	// Generate analysis prompt
	prompt := claude.SDKAnalysisPrompt(request.SDKName, request.Version, request.Code)

	messages := []claude.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Count tokens before sending
	tokenCount, err := a.client.CountTokens(ctx, messages)
	if err != nil {
		a.logger.Warn().Err(err).Msg("Failed to count tokens")
	}

	a.logger.Info().
		Str("sdk", request.SDKName).
		Str("version", request.Version).
		Int("estimated_tokens", tokenCount).
		Msg("Analyzing SDK with Claude")

	// Send request to Claude
	response, err := a.client.SendMessage(ctx, messages, "", 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze SDK: %w", err)
	}

	// Extract JSON from response
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("empty response from Claude")
	}

	analysisJSON := response.Content[0].Text

	// Parse the analysis
	var analysis SDKAnalysis
	if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
		// Try to extract JSON from markdown code block
		analysisJSON = extractJSONFromMarkdown(analysisJSON)
		if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
			return nil, fmt.Errorf("failed to parse analysis: %w", err)
		}
	}

	// Add metadata
	analysis.TokensUsed = response.Usage.InputTokens + response.Usage.OutputTokens
	analysis.AnalyzedAt = time.Now()
	analysis.AnalysisVersion = a.version

	duration := time.Since(startTime)
	a.logger.Info().
		Str("sdk", request.SDKName).
		Dur("duration", duration).
		Int("tokens_used", analysis.TokensUsed).
		Msg("SDK analysis completed")

	return &analysis, nil
}

// BatchAnalyze analyzes multiple SDKs in batch for cost optimization
func (a *ClaudeAnalyzer) BatchAnalyze(ctx context.Context, requests []AnalysisRequest) (*BatchAnalysisResult, error) {
	// For now, implement sequential analysis
	// TODO: Implement actual batch API when available

	result := &BatchAnalysisResult{
		JobID:   generateJobID(),
		Status:  "processing",
		Results: make(map[string]*SDKAnalysis),
		Errors:  make(map[string]string),
	}

	totalTokens := 0

	for _, req := range requests {
		analysis, err := a.AnalyzeCode(ctx, req)
		if err != nil {
			result.Errors[req.SDKName] = err.Error()
			a.logger.Error().
				Err(err).
				Str("sdk", req.SDKName).
				Msg("Failed to analyze SDK in batch")
			continue
		}

		result.Results[req.SDKName] = analysis
		totalTokens += analysis.TokensUsed
	}

	now := time.Now()
	result.Status = "completed"
	result.TotalTokens = totalTokens
	result.CompletedAt = &now

	return result, nil
}

// GetBatchStatus checks the status of a batch job
func (a *ClaudeAnalyzer) GetBatchStatus(ctx context.Context, jobID string) (*BatchAnalysisResult, error) {
	// TODO: Implement when batch API is available
	return nil, fmt.Errorf("batch status checking not yet implemented")
}

// CountTokens estimates token usage before sending request
func (a *ClaudeAnalyzer) CountTokens(ctx context.Context, request AnalysisRequest) (int, error) {
	prompt := claude.SDKAnalysisPrompt(request.SDKName, request.Version, request.Code)
	messages := []claude.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	return a.client.CountTokens(ctx, messages)
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(text string) string {
	// Look for ```json blocks
	jsonStart := "```json"
	jsonEnd := "```"

	startIdx := 0
	for {
		idx := findString(text[startIdx:], jsonStart)
		if idx == -1 {
			break
		}
		idx += startIdx

		start := idx + len(jsonStart)
		end := findString(text[start:], jsonEnd)
		if end == -1 {
			break
		}
		end += start

		// Trim whitespace from extracted content
		return trimString(text[start:end])
	}

	// Try without json tag
	startIdx = 0
	for {
		idx := findString(text[startIdx:], "```")
		if idx == -1 {
			break
		}
		idx += startIdx

		start := idx + 3
		end := findString(text[start:], "```")
		if end == -1 {
			break
		}
		end += start

		// Check if it looks like JSON
		content := trimString(text[start:end])
		if len(content) > 0 && (content[0] == '{' || content[0] == '[') {
			return content
		}

		startIdx = end + 3
	}

	return text
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

func trimString(s string) string {
	// Trim leading whitespace
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	// Trim trailing whitespace
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

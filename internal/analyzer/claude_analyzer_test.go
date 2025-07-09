package analyzer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/ryanrussell/claude-cache-service/internal/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCode(t *testing.T) {
	// Create mock response
	mockAnalysis := SDKAnalysis{
		Language:       "python",
		EnvelopeFormat: "JSON envelope with headers and items",
		Transport: TransportDetails{
			Type:                "http",
			Protocols:           []string{"https"},
			RetryMechanism:      "exponential backoff with jitter",
			QueueImplementation: "in-memory queue with disk overflow",
		},
		EventTypes: []string{"error", "transaction", "profile"},
		ErrorPatterns: []ErrorPattern{
			{
				Name:        "context_manager",
				Pattern:     "with sentry_sdk.push_scope()",
				Description: "Context manager for scope management",
			},
		},
		Integrations:    []string{"django", "flask", "celery"},
		Features:        []string{"breadcrumbs", "attachments", "sessions"},
		ProtocolVersion: "7",
		CachingPatterns: []CachingPattern{
			{
				Type:        "envelope",
				Location:    "transport layer",
				Description: "Caches envelopes during network failures",
			},
		},
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock Claude response
		analysisJSON, _ := json.Marshal(mockAnalysis)
		response := claude.Response{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []claude.ContentBlock{
				{Type: "text", Text: string(analysisJSON)},
			},
			Usage: claude.Usage{
				InputTokens:  100,
				OutputTokens: 200,
			},
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create analyzer with mock client
	logger := zerolog.Nop()
	client := claude.NewClient("test-key", "claude-3-opus", logger)
	client.BaseURL = server.URL

	analyzer := &ClaudeAnalyzer{
		client:  client,
		logger:  logger,
		version: "1.0.0",
	}

	// Test analyze code
	ctx := context.Background()
	request := AnalysisRequest{
		SDKName:    "sentry-python",
		Version:    "1.0.0",
		CommitHash: "abc123",
		Code: map[string]string{
			"transport.py": "def send_envelope():\n    pass",
			"client.py":    "class Client:\n    pass",
		},
	}

	analysis, err := analyzer.AnalyzeCode(ctx, request)

	require.NoError(t, err)
	assert.NotNil(t, analysis)
	assert.Equal(t, "python", analysis.Language)
	assert.Equal(t, "JSON envelope with headers and items", analysis.EnvelopeFormat)
	assert.Equal(t, "http", analysis.Transport.Type)
	assert.Contains(t, analysis.EventTypes, "error")
	assert.Len(t, analysis.ErrorPatterns, 1)
	assert.Equal(t, 300, analysis.TokensUsed)
	assert.Equal(t, "1.0.0", analysis.AnalysisVersion)
	assert.WithinDuration(t, time.Now(), analysis.AnalyzedAt, 5*time.Second)
}

func TestAnalyzeCodeWithMarkdown(t *testing.T) {
	// Test JSON extraction from markdown
	mockAnalysisJSON := `{
		"language": "go",
		"envelope_format": "binary format",
		"transport": {
			"type": "grpc",
			"protocols": ["grpc"],
			"retry_mechanism": "simple retry",
			"queue_implementation": "channel-based"
		},
		"event_types": ["error"],
		"error_patterns": [],
		"integrations": [],
		"features": ["concurrent"],
		"protocol_version": "8",
		"caching_patterns": []
	}`

	// Create test server that returns JSON in markdown
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := claude.Response{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []claude.ContentBlock{
				{
					Type: "text",
					Text: "Here's the analysis:\n\n```json\n" + mockAnalysisJSON + "\n```\n\nThe SDK uses modern patterns.",
				},
			},
			Usage: claude.Usage{
				InputTokens:  50,
				OutputTokens: 100,
			},
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create analyzer
	logger := zerolog.Nop()
	client := claude.NewClient("test-key", "claude-3-opus", logger)
	client.BaseURL = server.URL

	analyzer := &ClaudeAnalyzer{
		client:  client,
		logger:  logger,
		version: "1.0.0",
	}

	// Test
	ctx := context.Background()
	request := AnalysisRequest{
		SDKName: "sentry-go",
		Version: "1.0.0",
		Code: map[string]string{
			"main.go": "package main",
		},
	}

	analysis, err := analyzer.AnalyzeCode(ctx, request)

	require.NoError(t, err)
	assert.Equal(t, "go", analysis.Language)
	assert.Equal(t, "grpc", analysis.Transport.Type)
	assert.Equal(t, "8", analysis.ProtocolVersion)
}

func TestBatchAnalyze(t *testing.T) {
	callCount := 0

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Return different analysis based on call count
		var analysis SDKAnalysis
		if callCount == 1 {
			analysis = SDKAnalysis{
				Language:        "python",
				ProtocolVersion: "7",
			}
		} else {
			analysis = SDKAnalysis{
				Language:        "javascript",
				ProtocolVersion: "7",
			}
		}

		analysisJSON, _ := json.Marshal(analysis)
		response := claude.Response{
			Content: []claude.ContentBlock{
				{Type: "text", Text: string(analysisJSON)},
			},
			Usage: claude.Usage{
				InputTokens:  100,
				OutputTokens: 100,
			},
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create analyzer
	logger := zerolog.Nop()
	client := claude.NewClient("test-key", "claude-3-opus", logger)
	client.BaseURL = server.URL

	analyzer := &ClaudeAnalyzer{
		client:  client,
		logger:  logger,
		version: "1.0.0",
	}

	// Test batch analyze
	ctx := context.Background()
	requests := []AnalysisRequest{
		{
			SDKName: "sentry-python",
			Version: "1.0.0",
			Code:    map[string]string{"main.py": "import sentry_sdk"},
		},
		{
			SDKName: "sentry-javascript",
			Version: "2.0.0",
			Code:    map[string]string{"index.js": "const Sentry = require('@sentry/node')"},
		},
	}

	result, err := analyzer.BatchAnalyze(ctx, requests)

	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.Results, 2)
	assert.Len(t, result.Errors, 0)
	assert.Equal(t, 400, result.TotalTokens) // 200 + 200
	assert.NotNil(t, result.CompletedAt)

	// Check individual results
	pythonAnalysis := result.Results["sentry-python"]
	assert.NotNil(t, pythonAnalysis)
	assert.Equal(t, "python", pythonAnalysis.Language)

	jsAnalysis := result.Results["sentry-javascript"]
	assert.NotNil(t, jsAnalysis)
	assert.Equal(t, "javascript", jsAnalysis.Language)
}

func TestCountTokens(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called for token counting
		t.Fatal("Server should not be called for token counting")
	}))
	defer server.Close()

	// Create analyzer
	logger := zerolog.Nop()
	client := claude.NewClient("test-key", "claude-3-opus", logger)
	client.BaseURL = server.URL

	analyzer := &ClaudeAnalyzer{
		client:  client,
		logger:  logger,
		version: "1.0.0",
	}

	// Test token counting
	ctx := context.Background()
	request := AnalysisRequest{
		SDKName: "sentry-python",
		Version: "1.0.0",
		Code: map[string]string{
			"transport.py": "def send_envelope():\n    pass",
			"client.py":    "class Client:\n    pass",
		},
	}

	count, err := analyzer.CountTokens(ctx, request)

	require.NoError(t, err)
	assert.Greater(t, count, 100) // Should include prompt template
}

func TestExtractJSONFromMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "json with tag",
			input:    "Some text\n```json\n{\"key\": \"value\"}\n```\nMore text",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "json without tag",
			input:    "Some text\n```\n{\"key\": \"value\"}\n```\nMore text",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "no markdown blocks",
			input:    "{\"key\": \"value\"}",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "multiple blocks",
			input:    "```python\nprint('hello')\n```\n\n```json\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSONFromMarkdown(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

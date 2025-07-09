package claude

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	logger := zerolog.Nop()
	client := NewClient("test-api-key", "claude-3-opus", logger)

	assert.Equal(t, "test-api-key", client.apiKey)
	assert.Equal(t, defaultBaseURL, client.BaseURL)
	assert.Equal(t, "claude-3-opus", client.model)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.limiter)
}

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name           string
		messages       []Message
		serverResponse interface{}
		statusCode     int
		expectedError  bool
		errorMessage   string
	}{
		{
			name: "successful request",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			serverResponse: Response{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello! How can I help you?"},
				},
				Model: "claude-3-opus",
				Usage: Usage{
					InputTokens:  10,
					OutputTokens: 20,
				},
			},
			statusCode:    http.StatusOK,
			expectedError: false,
		},
		{
			name: "rate limit error",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			serverResponse: ErrorResponse{
				Type:    "rate_limit_error",
				Message: "Rate limit exceeded",
			},
			statusCode:    http.StatusTooManyRequests,
			expectedError: true,
			errorMessage:  "Rate limit exceeded",
		},
		{
			name: "authentication error",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			serverResponse: ErrorResponse{
				Type:    "authentication_error",
				Message: "Invalid API key",
			},
			statusCode:    http.StatusUnauthorized,
			expectedError: true,
			errorMessage:  "Invalid API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
				assert.Equal(t, apiVersion, r.Header.Get("anthropic-version"))

				// Send response
				w.WriteHeader(tt.statusCode)
				if err := json.NewEncoder(w).Encode(tt.serverResponse); err != nil {
					t.Fatalf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			// Create client with test server
			logger := zerolog.Nop()
			client := NewClient("test-api-key", "claude-3-opus", logger)
			client.BaseURL = server.URL

			// Send message
			ctx := context.Background()
			resp, err := client.SendMessage(ctx, tt.messages, "", 100)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "msg_123", resp.ID)
				assert.Equal(t, 10, resp.Usage.InputTokens)
				assert.Equal(t, 20, resp.Usage.OutputTokens)
			}
		})
	}
}

func TestRetryLogic(t *testing.T) {
	callCount := 0

	// Create test server that fails twice then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if callCount < 3 {
			// Fail with server error (retryable)
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(ErrorResponse{
				Type:    "internal_server_error",
				Message: "Server error",
			}); err != nil {
				t.Fatalf("Failed to encode error response: %v", err)
			}
		} else {
			// Succeed on third attempt
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(Response{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Success after retries"},
				},
			}); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		}
	}))
	defer server.Close()

	// Create client with test server
	logger := zerolog.Nop()
	client := NewClient("test-api-key", "claude-3-opus", logger)
	client.BaseURL = server.URL

	// Override retry delay for faster tests
	originalDelay := RetryDelay
	RetryDelay = 10 * time.Millisecond
	defer func() { RetryDelay = originalDelay }()

	// Send message
	ctx := context.Background()
	resp, err := client.SendMessage(ctx, []Message{{Role: "user", Content: "Test"}}, "", 100)

	// Should succeed after retries
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 3, callCount)
}

func TestCountTokens(t *testing.T) {
	logger := zerolog.Nop()
	client := NewClient("test-api-key", "claude-3-opus", logger)

	messages := []Message{
		{Role: "user", Content: "Hello, how are you?"},
		{Role: "assistant", Content: "I'm doing well, thank you!"},
	}

	ctx := context.Background()
	count, err := client.CountTokens(ctx, messages)

	require.NoError(t, err)
	// Approximate count should be around 15-20 tokens
	assert.Greater(t, count, 10)
	assert.Less(t, count, 30)
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 429,
		Type:       "rate_limit_error",
		Message:    "Too many requests",
	}

	assert.Equal(t, "Claude API error 429 (rate_limit_error): Too many requests", err.Error())
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "rate limit error",
			err:       &APIError{StatusCode: 429},
			retryable: true,
		},
		{
			name:      "server error",
			err:       &APIError{StatusCode: 500},
			retryable: true,
		},
		{
			name:      "bad gateway",
			err:       &APIError{StatusCode: 502},
			retryable: true,
		},
		{
			name:      "authentication error",
			err:       &APIError{StatusCode: 401},
			retryable: false,
		},
		{
			name:      "bad request",
			err:       &APIError{StatusCode: 400},
			retryable: false,
		},
		{
			name:      "network error",
			err:       assert.AnError,
			retryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, isRetryableError(tt.err))
		})
	}
}

package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

const (
	defaultBaseURL = "https://api.anthropic.com"
	apiVersion     = "2023-06-01"
	maxRetries     = 3
)

var (
	RetryDelay = time.Second // Exported for testing
)

// Client represents a Claude API client
type Client struct {
	apiKey     string
	BaseURL    string // Exported for testing
	httpClient *http.Client
	limiter    *rate.Limiter
	logger     zerolog.Logger
	model      string
}

// NewClient creates a new Claude API client
func NewClient(apiKey, model string, logger zerolog.Logger) *Client {
	if model == "" {
		model = "claude-3-opus-20240229"
	}

	return &Client{
		apiKey:  apiKey,
		BaseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		// Claude API limits: 50 RPM for tier 1
		limiter: rate.NewLimiter(rate.Every(time.Minute/50), 5), // 50 RPM with burst of 5
		logger:  logger,
		model:   model,
	}
}

// Message represents a message in the Claude API
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request represents a Claude API request
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
}

// Response represents a Claude API response
type Response struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model"`
	Usage   Usage          `json:"usage"`
}

// ContentBlock represents a content block in the response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// SendMessage sends a message to Claude API
func (c *Client) SendMessage(ctx context.Context, messages []Message, system string, maxTokens int) (*Response, error) {
	// Rate limiting
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	request := Request{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: maxTokens,
		System:    system,
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.doRequest(ctx, "/v1/messages", request)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return nil, err
		}

		// Exponential backoff
		delay := RetryDelay * time.Duration(1<<attempt)
		c.logger.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Dur("delay", delay).
			Msg("Retrying Claude API request")

		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) doRequest(ctx context.Context, endpoint string, payload interface{}) (*Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+endpoint, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Type:       errResp.Type,
			Message:    errResp.Message,
		}
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// APIError represents a Claude API error
type APIError struct {
	StatusCode int
	Type       string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Claude API error %d (%s): %s", e.StatusCode, e.Type, e.Message)
}

func isRetryableError(err error) bool {
	apiErr, ok := err.(*APIError)
	if !ok {
		return true // Network errors are retryable
	}

	// Retry on rate limit or server errors
	return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
}

// CountTokens estimates token count for messages
func (c *Client) CountTokens(ctx context.Context, messages []Message) (int, error) {
	// Simple approximation: ~4 characters per token
	// In production, use tiktoken or Claude's token counting endpoint
	totalChars := 0
	for _, msg := range messages {
		totalChars += len(msg.Role) + len(msg.Content) + 10 // overhead
	}
	return totalChars / 4, nil
}

// createRequest creates a new HTTP request with auth headers
func (c *Client) createRequest(ctx context.Context, endpoint string, body []byte) (*http.Request, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequestWithContext(ctx, "POST", c.BaseURL+endpoint, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, "GET", c.BaseURL+endpoint, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	return req, nil
}

// handleErrorResponse processes error responses from the API
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API error (status %d): failed to read error body", resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Type:       errResp.Type,
		Message:    errResp.Message,
	}
}

package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BatchRequest represents a request in the batch API
type BatchRequest struct {
	CustomID string  `json:"custom_id"`
	Method   string  `json:"method"`
	URL      string  `json:"url"`
	Body     Request `json:"body"`
}

// BatchResponse represents the batch API response
type BatchResponse struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	ProcessingStatus string `json:"processing_status"`
	RequestCounts    struct {
		Processing int `json:"processing"`
		Succeeded  int `json:"succeeded"`
		Failed     int `json:"failed"`
		Total      int `json:"total"`
	} `json:"request_counts"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ResultsURL  string     `json:"results_url,omitempty"`
}

// BatchResult represents a single result from batch processing
type BatchResult struct {
	CustomID string    `json:"custom_id"`
	Response *Response `json:"response,omitempty"`
	Error    *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CreateBatch creates a new batch job
func (c *Client) CreateBatch(ctx context.Context, requests []BatchRequest) (*BatchResponse, error) {
	payload := map[string]interface{}{
		"requests": requests,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	// Batch API doesn't count against rate limits
	req, err := c.createRequest(ctx, "/v1/batches", jsonData)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.handleErrorResponse(resp)
	}

	var batchResp BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode batch response: %w", err)
	}

	c.logger.Info().
		Str("batch_id", batchResp.ID).
		Int("total_requests", batchResp.RequestCounts.Total).
		Msg("Batch job created")

	return &batchResp, nil
}

// GetBatchStatus retrieves the status of a batch job
func (c *Client) GetBatchStatus(ctx context.Context, batchID string) (*BatchResponse, error) {
	req, err := c.createRequest(ctx, fmt.Sprintf("/v1/batches/%s", batchID), nil)
	if err != nil {
		return nil, err
	}
	req.Method = "GET"

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch status: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var batchResp BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode batch status: %w", err)
	}

	return &batchResp, nil
}

// GetBatchResults retrieves the results of a completed batch job
func (c *Client) GetBatchResults(ctx context.Context, resultsURL string) ([]BatchResult, error) {
	// Results are typically stored in a separate location
	// This is a simplified implementation
	req, err := c.createRequest(ctx, resultsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Method = "GET"

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch results: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var results []BatchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode batch results: %w", err)
	}

	return results, nil
}

// CancelBatch cancels a batch job
func (c *Client) CancelBatch(ctx context.Context, batchID string) error {
	req, err := c.createRequest(ctx, fmt.Sprintf("/v1/batches/%s/cancel", batchID), nil)
	if err != nil {
		return err
	}
	req.Method = "POST"

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to cancel batch: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return c.handleErrorResponse(resp)
	}

	c.logger.Info().
		Str("batch_id", batchID).
		Msg("Batch job cancelled")

	return nil
}

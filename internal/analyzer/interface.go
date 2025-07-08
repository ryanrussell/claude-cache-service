package analyzer

import (
	"context"
	"time"
)

// SDKAnalysis represents the analyzed result from Claude
type SDKAnalysis struct {
	Language        string           `json:"language"`
	EnvelopeFormat  string           `json:"envelope_format"`
	Transport       TransportDetails `json:"transport"`
	EventTypes      []string         `json:"event_types"`
	ErrorPatterns   []ErrorPattern   `json:"error_patterns"`
	Integrations    []string         `json:"integrations"`
	Features        []string         `json:"features"`
	ProtocolVersion string           `json:"protocol_version"`
	CachingPatterns []CachingPattern `json:"caching_patterns"`
	TokensUsed      int              `json:"tokens_used"`
	AnalyzedAt      time.Time        `json:"analyzed_at"`
	AnalysisVersion string           `json:"analysis_version"`
}

// TransportDetails contains transport implementation details
type TransportDetails struct {
	Type                string   `json:"type"`
	Protocols           []string `json:"protocols"`
	RetryMechanism      string   `json:"retry_mechanism"`
	QueueImplementation string   `json:"queue_implementation"`
}

// ErrorPattern represents common error handling patterns
type ErrorPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

// CachingPattern represents caching strategies found in the SDK
type CachingPattern struct {
	Type        string `json:"type"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

// AnalysisRequest represents a request to analyze SDK code
type AnalysisRequest struct {
	SDKName    string            `json:"sdk_name"`
	Version    string            `json:"version"`
	Code       map[string]string `json:"code"` // filename -> content
	CommitHash string            `json:"commit_hash"`
}

// BatchAnalysisResult represents results from batch analysis
type BatchAnalysisResult struct {
	JobID       string                  `json:"job_id"`
	Status      string                  `json:"status"`
	Results     map[string]*SDKAnalysis `json:"results"`
	Errors      map[string]string       `json:"errors"`
	TotalTokens int                     `json:"total_tokens"`
	CompletedAt *time.Time              `json:"completed_at,omitempty"`
}

// Analyzer defines the interface for SDK analysis
type Analyzer interface {
	// AnalyzeCode analyzes a single SDK's code
	AnalyzeCode(ctx context.Context, request AnalysisRequest) (*SDKAnalysis, error)

	// BatchAnalyze analyzes multiple SDKs in batch for cost optimization
	BatchAnalyze(ctx context.Context, requests []AnalysisRequest) (*BatchAnalysisResult, error)

	// GetBatchStatus checks the status of a batch job
	GetBatchStatus(ctx context.Context, jobID string) (*BatchAnalysisResult, error)

	// CountTokens estimates token usage before sending request
	CountTokens(ctx context.Context, request AnalysisRequest) (int, error)
}

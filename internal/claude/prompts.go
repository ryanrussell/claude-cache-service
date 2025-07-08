package claude

import (
	"fmt"
	"strings"
)

// SDKAnalysisPrompt generates a prompt for analyzing SDK code
func SDKAnalysisPrompt(sdkName, version string, codeFiles map[string]string) string {
	var codeSnippets []string
	for filename, content := range codeFiles {
		// Limit file content to prevent token overflow
		truncatedContent := content
		if len(content) > 10000 {
			truncatedContent = content[:10000] + "\n... [truncated]"
		}
		codeSnippets = append(codeSnippets, fmt.Sprintf("File: %s\n```\n%s\n```", filename, truncatedContent))
	}

	systemPrompt := `You are an expert SDK analyzer specializing in Sentry SDKs. Your task is to analyze SDK code and extract key patterns and implementation details.

Focus on:
1. Envelope format and structure
2. Transport implementation (HTTP, queue, retry logic)
3. Error handling patterns
4. Protocol versions and compatibility
5. Caching strategies
6. Key features and integrations

Provide a structured analysis in JSON format.`

	userPrompt := fmt.Sprintf(`Analyze the following %s SDK (version %s) code and extract implementation patterns:

%s

Provide your analysis in the following JSON format:
{
  "language": "detected programming language",
  "envelope_format": "description of envelope format used",
  "transport": {
    "type": "http/grpc/other",
    "protocols": ["list of protocols"],
    "retry_mechanism": "description of retry logic",
    "queue_implementation": "description of queue if any"
  },
  "event_types": ["list of supported event types"],
  "error_patterns": [
    {
      "name": "pattern name",
      "pattern": "code pattern",
      "description": "what it does"
    }
  ],
  "integrations": ["list of framework integrations"],
  "features": ["list of key features"],
  "protocol_version": "detected protocol version",
  "caching_patterns": [
    {
      "type": "cache type",
      "location": "where it's used",
      "description": "how it works"
    }
  ]
}`, sdkName, version, strings.Join(codeSnippets, "\n\n"))

	return systemPrompt + "\n\n" + userPrompt
}

// BatchAnalysisPrompt creates a prompt for batch SDK analysis
func BatchAnalysisPrompt(requests []PromptBatchRequest) string {
	systemPrompt := `You are an expert SDK analyzer. Analyze multiple SDK code samples and provide structured analysis for each.

For each SDK, focus on:
1. Core implementation patterns
2. Transport and protocol details
3. Error handling approaches
4. Unique features

Provide analysis in a consistent JSON format for easy comparison.`

	var sdkSections []string
	for _, req := range requests {
		section := fmt.Sprintf("SDK: %s (version %s)\nFiles: %d",
			req.SDKName, req.Version, len(req.CodeFiles))
		sdkSections = append(sdkSections, section)
	}

	userPrompt := fmt.Sprintf(`Analyze the following SDKs:

%s

Provide analysis for each SDK in a JSON array format, maintaining consistency across all analyses.`,
		strings.Join(sdkSections, "\n\n"))

	return systemPrompt + "\n\n" + userPrompt
}

// PromptBatchRequest represents a single SDK in a batch analysis request for prompts
type PromptBatchRequest struct {
	SDKName   string
	Version   string
	CodeFiles map[string]string
}

// CostOptimizedPrompt creates a token-efficient prompt for basic analysis
func CostOptimizedPrompt(sdkName string, keyFiles []string) string {
	return fmt.Sprintf(`Quick analysis of %s SDK. Focus only on:
1. Transport type (http/grpc/other)
2. Protocol version
3. Main error handling pattern
4. Envelope format

Files to check: %s

Respond with brief JSON containing only these 4 fields.`, sdkName, strings.Join(keyFiles, ", "))
}

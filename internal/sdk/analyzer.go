package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/ryanrussell/claude-cache-service/internal/analyzer"
	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/git"
)

// Analyzer handles SDK analysis operations
type Analyzer struct {
	git     *git.Client
	claude  analyzer.Analyzer
	cache   *cache.Manager
	logger  zerolog.Logger
	configs *ConfigList
}

// NewAnalyzer creates a new SDK analyzer
func NewAnalyzer(gitClient *git.Client, claudeAnalyzer analyzer.Analyzer, cacheManager *cache.Manager, logger zerolog.Logger) (*Analyzer, error) {
	configs, err := LoadConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to load SDK configs: %w", err)
	}

	return &Analyzer{
		git:     gitClient,
		claude:  claudeAnalyzer,
		cache:   cacheManager,
		logger:  logger,
		configs: configs,
	}, nil
}

// AnalysisResult represents the result of analyzing an SDK
type AnalysisResult struct {
	SDK      Config
	Analysis *analyzer.SDKAnalysis
	Error    error
}

// AnalyzeSDK analyzes a single SDK
func (a *Analyzer) AnalyzeSDK(ctx context.Context, sdk Config) (*analyzer.SDKAnalysis, error) {
	a.logger.Info().
		Str("sdk", sdk.Name).
		Str("url", sdk.URL).
		Msg("Starting SDK analysis")

	// Clone or update the repository
	branch := sdk.Branch
	if branch == "" {
		branch = "main"
	}

	if err := a.git.Clone(ctx, sdk.URL, branch); err != nil {
		return nil, fmt.Errorf("failed to clone/update repository: %w", err)
	}

	// Get repository path
	repoPath := a.git.GetRepoPath(sdk.URL)

	// Extract relevant files
	codeFiles, err := a.extractCodeFiles(repoPath, sdk)
	if err != nil {
		return nil, fmt.Errorf("failed to extract code files: %w", err)
	}

	a.logger.Debug().
		Str("sdk", sdk.Name).
		Int("files", len(codeFiles)).
		Msg("Extracted code files for analysis")

	// Get latest commit info
	latestCommit, err := a.git.GetLatestCommit(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit: %w", err)
	}

	// Prepare analysis request
	request := analyzer.AnalysisRequest{
		SDKName:    sdk.Name,
		Version:    latestCommit.Hash[:7], // Use short commit hash as version
		Code:       codeFiles,
		CommitHash: latestCommit.Hash,
	}

	// Analyze with Claude
	analysis, err := a.claude.AnalyzeCode(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze SDK: %w", err)
	}

	a.logger.Info().
		Str("sdk", sdk.Name).
		Int("tokens_used", analysis.TokensUsed).
		Msg("SDK analysis completed")

	return analysis, nil
}

// AnalyzeAllSDKs analyzes all active SDKs
func (a *Analyzer) AnalyzeAllSDKs(ctx context.Context) []AnalysisResult {
	activeSDKs := a.configs.GetActiveSDKs()
	results := make([]AnalysisResult, 0, len(activeSDKs))

	a.logger.Info().
		Int("count", len(activeSDKs)).
		Msg("Starting analysis of active SDKs")

	// Prepare batch requests for cost optimization
	batchSize := 5 // Process 5 SDKs at a time
	for i := 0; i < len(activeSDKs); i += batchSize {
		end := i + batchSize
		if end > len(activeSDKs) {
			end = len(activeSDKs)
		}

		batch := activeSDKs[i:end]
		batchResults := a.analyzeBatch(ctx, batch)
		results = append(results, batchResults...)
	}

	return results
}

// analyzeBatch analyzes a batch of SDKs
func (a *Analyzer) analyzeBatch(ctx context.Context, sdks []Config) []AnalysisResult {
	var requests []analyzer.AnalysisRequest
	sdkMap := make(map[string]Config)

	// Prepare batch requests
	for _, sdk := range sdks {
		// Clone/update repository
		branch := sdk.Branch
		if branch == "" {
			branch = "main"
		}

		if err := a.git.Clone(ctx, sdk.URL, branch); err != nil {
			a.logger.Error().
				Err(err).
				Str("sdk", sdk.Name).
				Msg("Failed to clone repository")
			continue
		}

		// Extract code files
		repoPath := a.git.GetRepoPath(sdk.URL)
		codeFiles, err := a.extractCodeFiles(repoPath, sdk)
		if err != nil {
			a.logger.Error().
				Err(err).
				Str("sdk", sdk.Name).
				Msg("Failed to extract code files")
			continue
		}

		// Get latest commit
		latestCommit, err := a.git.GetLatestCommit(ctx, repoPath)
		if err != nil {
			a.logger.Error().
				Err(err).
				Str("sdk", sdk.Name).
				Msg("Failed to get latest commit")
			continue
		}

		request := analyzer.AnalysisRequest{
			SDKName:    sdk.Name,
			Version:    latestCommit.Hash[:7],
			Code:       codeFiles,
			CommitHash: latestCommit.Hash,
		}

		requests = append(requests, request)
		sdkMap[sdk.Name] = sdk
	}

	// Batch analyze
	batchResult, err := a.claude.BatchAnalyze(ctx, requests)
	if err != nil {
		a.logger.Error().
			Err(err).
			Msg("Batch analysis failed, falling back to individual analysis")

		// Fall back to individual analysis
		var results []AnalysisResult
		for _, sdk := range sdks {
			analysis, err := a.AnalyzeSDK(ctx, sdk)
			results = append(results, AnalysisResult{
				SDK:      sdk,
				Analysis: analysis,
				Error:    err,
			})
		}
		return results
	}

	// Convert batch results to analysis results
	var results []AnalysisResult
	for sdkName, analysis := range batchResult.Results {
		sdk := sdkMap[sdkName]
		results = append(results, AnalysisResult{
			SDK:      sdk,
			Analysis: analysis,
			Error:    nil,
		})
	}

	// Add errors
	for sdkName, errMsg := range batchResult.Errors {
		sdk := sdkMap[sdkName]
		results = append(results, AnalysisResult{
			SDK:      sdk,
			Analysis: nil,
			Error:    fmt.Errorf("%s", errMsg),
		})
	}

	return results
}

// NeedsUpdate checks if an SDK needs to be updated
func (a *Analyzer) NeedsUpdate(ctx context.Context, sdk Config) (bool, error) {
	// Check cache for last analysis
	cacheKey := fmt.Sprintf("sdk:%s:last_analyzed", sdk.Name)
	lastAnalyzedStr, err := a.cache.Get(cacheKey)
	if err != nil {
		// Not in cache, needs update
		return true, nil
	}

	lastAnalyzed, err := time.Parse(time.RFC3339, lastAnalyzedStr)
	if err != nil {
		// Invalid timestamp, needs update
		return true, nil
	}

	// Check if repository has updates since last analysis
	repoPath := a.git.GetRepoPath(sdk.URL)
	if _, err := os.Stat(repoPath); err != nil {
		// Repository doesn't exist, needs clone and update
		return true, nil
	}

	// Pull latest changes
	if err := a.git.Pull(ctx, repoPath); err != nil {
		a.logger.Warn().
			Err(err).
			Str("sdk", sdk.Name).
			Msg("Failed to pull latest changes")
	}

	// Get commits since last analysis
	commits, err := a.git.GetCommitsSince(ctx, repoPath, lastAnalyzed)
	if err != nil {
		return false, fmt.Errorf("failed to check for updates: %w", err)
	}

	// If there are new commits, needs update
	return len(commits) > 0, nil
}

// extractCodeFiles extracts relevant code files from the repository
func (a *Analyzer) extractCodeFiles(repoPath string, sdk Config) (map[string]string, error) {
	codeFiles := make(map[string]string)

	// If key files are specified, read those first
	if len(sdk.KeyFiles) > 0 {
		for _, keyFile := range sdk.KeyFiles {
			filePath := filepath.Join(repoPath, keyFile)
			content, err := os.ReadFile(filePath)
			if err != nil {
				a.logger.Warn().
					Err(err).
					Str("file", keyFile).
					Msg("Failed to read key file")
				continue
			}
			codeFiles[keyFile] = string(content)
		}
	}

	// Walk the repository and find matching files
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			// Skip common non-code directories
			if strings.Contains(path, "/.git/") ||
				strings.Contains(path, "/node_modules/") ||
				strings.Contains(path, "/vendor/") ||
				strings.Contains(path, "/__pycache__/") ||
				strings.Contains(path, "/.pytest_cache/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files that are too large
		if info.Size() > 100*1024 { // 100KB limit per file
			return nil
		}

		// Check if file matches patterns
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}

		for _, pattern := range sdk.Patterns {
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				continue
			}
			if matched {
				// Limit total files to prevent token overflow
				if len(codeFiles) >= 50 {
					return filepath.SkipAll
				}

				content, err := os.ReadFile(path)
				if err != nil {
					a.logger.Warn().
						Err(err).
						Str("file", relPath).
						Msg("Failed to read file")
					continue
				}

				codeFiles[relPath] = string(content)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	// If no files found, return error
	if len(codeFiles) == 0 {
		return nil, fmt.Errorf("no matching files found in repository")
	}

	return codeFiles, nil
}

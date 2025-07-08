package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	logger := zerolog.Nop()
	workDir := "/tmp/test"

	client := NewClient(workDir, logger)

	assert.Equal(t, workDir, client.workDir)
}

func TestGetRepoPath(t *testing.T) {
	logger := zerolog.Nop()
	workDir := "/tmp/test"
	client := NewClient(workDir, logger)

	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "github https url",
			repoURL:  "https://github.com/getsentry/sentry-go",
			expected: "/tmp/test/sentry-go",
		},
		{
			name:     "github https url with .git",
			repoURL:  "https://github.com/getsentry/sentry-go.git",
			expected: "/tmp/test/sentry-go",
		},
		{
			name:     "github ssh url",
			repoURL:  "git@github.com:getsentry/sentry-python.git",
			expected: "/tmp/test/sentry-python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.GetRepoPath(tt.repoURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitOperations(t *testing.T) {
	// This test requires a real git operation, so we'll create a test repo
	tempDir := t.TempDir()
	logger := zerolog.Nop()
	client := NewClient(tempDir, logger)

	// Create a test repository
	testRepoPath := filepath.Join(tempDir, "test-repo")
	repo, err := git.PlainInit(testRepoPath, false)
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(testRepoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Add and commit
	w, err := repo.Worktree()
	require.NoError(t, err)

	_, err = w.Add("test.txt")
	require.NoError(t, err)

	commit, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Test GetLatestCommit
	ctx := context.Background()
	latestCommit, err := client.GetLatestCommit(ctx, testRepoPath)
	require.NoError(t, err)
	assert.Equal(t, commit.String(), latestCommit.Hash)
	assert.Equal(t, "Initial commit", latestCommit.Message)

	// Test GetCommitsSince
	since := time.Now().Add(-1 * time.Hour)
	commits, err := client.GetCommitsSince(ctx, testRepoPath, since)
	require.NoError(t, err)
	assert.Len(t, commits, 1)
	assert.Equal(t, "Initial commit", commits[0].Message)

	// Test GetChangedFiles
	files, err := client.GetChangedFiles(ctx, testRepoPath, since)
	require.NoError(t, err)
	assert.Contains(t, files, "test.txt")
}

func TestCloneNonExistentRepo(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.Nop()
	client := NewClient(tempDir, logger)

	ctx := context.Background()
	err := client.Clone(ctx, "https://github.com/nonexistent/repo.git", "main")
	assert.Error(t, err)
}

func TestGetRepoName(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "https url without .git",
			repoURL:  "https://github.com/getsentry/sentry-go",
			expected: "sentry-go",
		},
		{
			name:     "https url with .git",
			repoURL:  "https://github.com/getsentry/sentry-go.git",
			expected: "sentry-go",
		},
		{
			name:     "ssh url",
			repoURL:  "git@github.com:getsentry/sentry-python.git",
			expected: "sentry-python",
		},
		{
			name:     "simple name",
			repoURL:  "my-repo",
			expected: "my-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRepoName(tt.repoURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

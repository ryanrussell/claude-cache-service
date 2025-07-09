package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rs/zerolog"
)

// Client handles Git operations for SDK repositories
type Client struct {
	workDir string
	logger  zerolog.Logger
}

// NewClient creates a new Git client
func NewClient(workDir string, logger zerolog.Logger) *Client {
	return &Client{
		workDir: workDir,
		logger:  logger,
	}
}

// Clone clones a repository to the specified path
func (g *Client) Clone(ctx context.Context, repoURL, branch string) error {
	repoName := getRepoName(repoURL)
	repoPath := filepath.Join(g.workDir, repoName)

	// Check if repo already exists
	if _, err := os.Stat(repoPath); err == nil {
		g.logger.Info().
			Str("repo", repoName).
			Str("path", repoPath).
			Msg("Repository already exists, pulling latest changes")
		return g.Pull(ctx, repoPath)
	}

	g.logger.Info().
		Str("repo", repoName).
		Str("url", repoURL).
		Str("branch", branch).
		Msg("Cloning repository")

	opts := &git.CloneOptions{
		URL:               repoURL,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Progress:          nil, // Suppress progress output
	}

	if branch != "" && branch != "main" && branch != "master" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		opts.SingleBranch = true
	}

	_, err := git.PlainCloneContext(ctx, repoPath, false, opts)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	g.logger.Info().
		Str("repo", repoName).
		Msg("Repository cloned successfully")

	return nil
}

// Pull pulls the latest changes for a repository
func (g *Client) Pull(ctx context.Context, repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	g.logger.Debug().
		Str("path", repoPath).
		Msg("Pulling latest changes")

	err = w.PullContext(ctx, &git.PullOptions{
		RemoteName: "origin",
		Progress:   nil,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull changes: %w", err)
	}

	if err == git.NoErrAlreadyUpToDate {
		g.logger.Debug().
			Str("path", repoPath).
			Msg("Repository is already up to date")
	} else {
		g.logger.Info().
			Str("path", repoPath).
			Msg("Repository updated successfully")
	}

	return nil
}

// Commit represents a git commit
type Commit struct {
	Hash      string
	Author    string
	Message   string
	Timestamp time.Time
	Files     []string
}

// GetCommitsSince returns all commits since the specified time
func (g *Client) GetCommitsSince(ctx context.Context, repoPath string, since time.Time) ([]Commit, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	cIter, err := repo.Log(&git.LogOptions{
		From:  ref.Hash(),
		Since: &since,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	var commits []Commit
	err = cIter.ForEach(func(c *object.Commit) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		commit := Commit{
			Hash:      c.Hash.String(),
			Author:    c.Author.Email,
			Message:   c.Message,
			Timestamp: c.Author.When,
		}

		// Get changed files
		parent, err := c.Parent(0)
		switch err {
		case object.ErrParentNotFound:
			// Initial commit, all files are new
			files, err := c.Files()
			if err != nil {
				return err
			}
			err = files.ForEach(func(f *object.File) error {
				commit.Files = append(commit.Files, f.Name)
				return nil
			})
			if err != nil {
				return err
			}
		case nil:
			// Get diff between commit and parent
			patch, err := parent.Patch(c)
			if err != nil {
				return err
			}
			for _, fileStat := range patch.Stats() {
				commit.Files = append(commit.Files, fileStat.Name)
			}
		}

		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// GetChangedFiles returns all files changed since the specified time
func (g *Client) GetChangedFiles(ctx context.Context, repoPath string, since time.Time) ([]string, error) {
	commits, err := g.GetCommitsSince(ctx, repoPath, since)
	if err != nil {
		return nil, err
	}

	// Use map to deduplicate files
	fileMap := make(map[string]bool)
	for _, commit := range commits {
		for _, file := range commit.Files {
			fileMap[file] = true
		}
	}

	// Convert map to slice
	var files []string
	for file := range fileMap {
		files = append(files, file)
	}

	return files, nil
}

// GetRepoPath returns the local path for a repository
func (g *Client) GetRepoPath(repoURL string) string {
	repoName := getRepoName(repoURL)
	return filepath.Join(g.workDir, repoName)
}

// GetLatestCommit returns the latest commit for a repository
func (g *Client) GetLatestCommit(ctx context.Context, repoPath string) (*Commit, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	return &Commit{
		Hash:      commit.Hash.String(),
		Author:    commit.Author.Email,
		Message:   commit.Message,
		Timestamp: commit.Author.When,
	}, nil
}

// getRepoName extracts repository name from URL
func getRepoName(repoURL string) string {
	// Extract repo name from URL
	// e.g., https://github.com/getsentry/sentry-go -> sentry-go
	name := filepath.Base(repoURL)
	if filepath.Ext(name) == ".git" {
		name = name[:len(name)-4]
	}
	return name
}

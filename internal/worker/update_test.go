package worker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/config"
)

func TestNewUpdateWorker(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	cacheManager, err := cache.NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	cfg := &config.Config{
		UpdateSchedule: "0 2 * * 0",
		CacheTTL:       time.Hour,
	}

	worker := NewUpdateWorker(cacheManager, logger, cfg)
	assert.NotNil(t, worker)
	assert.NotNil(t, worker.cache)
	assert.NotNil(t, worker.cron)
}

func TestUpdateCache(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	cacheManager, err := cache.NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	cfg := &config.Config{
		UpdateSchedule: "0 2 * * 0",
		CacheTTL:       time.Hour,
	}

	worker := NewUpdateWorker(cacheManager, logger, cfg)

	ctx := context.Background()
	err = worker.updateCache(ctx)
	assert.NoError(t, err)

	// Verify that SDKs were cached
	// Only check the SDKs that are actually analyzed in the new implementation
	sdks := []string{"sentry-go", "sentry-python", "sentry-javascript"}
	for _, sdk := range sdks {
		key := "sdk:" + sdk
		value, err := cacheManager.Get(key)
		assert.NoError(t, err, "SDK %s should be cached", sdk)
		// The new implementation stores JSON analysis data
		assert.Contains(t, value, "language")
		assert.Contains(t, value, "envelope_format")
		assert.Contains(t, value, "protocol_version")
	}

	// Verify that projects were cached
	projects := []string{"gremlin-arrow-flight", "claude-code-gui"}
	for _, project := range projects {
		key := "project:" + project
		value, err := cacheManager.Get(key)
		assert.NoError(t, err, "Project %s should be cached", project)
		assert.Contains(t, value, project)
		assert.Contains(t, value, "cache_hits")
		assert.Contains(t, value, "token_savings")
	}
}

func TestUpdateCacheWithCancellation(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	cacheManager, err := cache.NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	cfg := &config.Config{
		UpdateSchedule: "0 2 * * 0",
		CacheTTL:       time.Hour,
	}

	worker := NewUpdateWorker(cacheManager, logger, cfg)

	// Create a context that we'll cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = worker.updateCache(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update cancelled")
}

func TestWorkerStartStop(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	cacheManager, err := cache.NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	cfg := &config.Config{
		UpdateSchedule: "* * * * * *", // Every second for testing
		CacheTTL:       time.Hour,
	}

	worker := NewUpdateWorker(cacheManager, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Start worker in background
	done := make(chan bool)
	go func() {
		worker.Start(ctx)
		done <- true
	}()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop the worker
	cancel()

	// Wait for it to finish
	select {
	case <-done:
		// Successfully stopped
	case <-time.After(5 * time.Second):
		t.Fatal("Worker did not stop in time")
	}
}

func TestCronLogger(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	cronLog := &cronLogger{logger: logger}

	// Should not panic
	cronLog.Printf("Test message: %s", "test")
	cronLog.Printf("Test number: %d", 42)
}

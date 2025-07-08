package cache

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()

	manager, err := NewManager(tempDir, logger)
	require.NoError(t, err)
	require.NotNil(t, manager)

	err = manager.Close()
	require.NoError(t, err)
}

func TestCacheOperations(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	manager, err := NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := manager.Close()
		require.NoError(t, err)
	}()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"
		ttl := 1 * time.Hour

		// Set value
		err := manager.Set(key, value, ttl)
		assert.NoError(t, err)

		// Get value
		result, err := manager.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("Get Non-existent Key", func(t *testing.T) {
		_, err := manager.Get("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key not found")
	})

	t.Run("Delete Key", func(t *testing.T) {
		key := "delete-test"
		value := "to-be-deleted"

		// Set value
		err := manager.Set(key, value, 0)
		assert.NoError(t, err)

		// Delete value
		err = manager.Delete(key)
		assert.NoError(t, err)

		// Try to get deleted value
		_, err = manager.Get(key)
		assert.Error(t, err)
	})

	t.Run("TTL Expiration", func(t *testing.T) {
		key := "ttl-test"
		value := "expires-soon"
		ttl := 100 * time.Millisecond

		// Set value with short TTL
		err := manager.Set(key, value, ttl)
		assert.NoError(t, err)

		// Value should exist immediately
		result, err := manager.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, value, result)

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Value should be expired
		_, err = manager.Get(key)
		assert.Error(t, err)
	})
}

func TestCacheStatistics(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	manager, err := NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := manager.Close()
		require.NoError(t, err)
	}()

	// Initial stats should be zero
	stats := manager.GetStats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
	assert.Equal(t, int64(0), stats.Sets)

	// Set a value
	err = manager.Set("key1", "value1", 0)
	assert.NoError(t, err)

	stats = manager.GetStats()
	assert.Equal(t, int64(1), stats.Sets)
	assert.Equal(t, int64(1), stats.ItemCount)

	// Get existing value (hit)
	_, err = manager.Get("key1")
	assert.NoError(t, err)

	stats = manager.GetStats()
	assert.Equal(t, int64(1), stats.Hits)

	// Get non-existent value (miss)
	_, err = manager.Get("non-existent")
	assert.Error(t, err)

	stats = manager.GetStats()
	assert.Equal(t, int64(1), stats.Misses)

	// Delete value
	err = manager.Delete("key1")
	assert.NoError(t, err)

	stats = manager.GetStats()
	assert.Equal(t, int64(1), stats.Deletes)
	assert.Equal(t, int64(0), stats.ItemCount)
}

func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	manager, err := NewManager(tempDir, logger)
	require.NoError(t, err)
	defer func() {
		err := manager.Close()
		require.NoError(t, err)
	}()

	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "concurrent-key"
			value := "value"
			
			// Perform multiple operations
			for j := 0; j < 100; j++ {
				_ = manager.Set(key, value, 0)
				_, _ = manager.Get(key)
				_ = manager.Delete(key)
			}
			
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Stats should be consistent
	stats := manager.GetStats()
	assert.True(t, stats.Sets > 0)
	assert.True(t, stats.Hits+stats.Misses > 0)
}

func BenchmarkCacheSet(b *testing.B) {
	tempDir := b.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	manager, err := NewManager(tempDir, logger)
	require.NoError(b, err)
	defer func() {
		_ = manager.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench-key"
		value := "bench-value"
		_ = manager.Set(key, value, 0)
	}
}

func BenchmarkCacheGet(b *testing.B) {
	tempDir := b.TempDir()
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)

	manager, err := NewManager(tempDir, logger)
	require.NoError(b, err)
	defer func() {
		_ = manager.Close()
	}()

	// Pre-populate cache
	key := "bench-key"
	value := "bench-value"
	_ = manager.Set(key, value, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.Get(key)
	}
}
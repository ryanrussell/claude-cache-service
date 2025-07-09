package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Test default configuration
	cfg, err := Load()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check defaults
	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.False(t, cfg.Debug)
	assert.Equal(t, "./cache", cfg.CacheDir)
	assert.Equal(t, "0 2 * * 0", cfg.UpdateSchedule)
	assert.Equal(t, 7*24*time.Hour, cfg.CacheTTL)
	assert.Equal(t, int64(1<<30), cfg.MaxCacheSize)
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"PORT":              "9090",
		"DEBUG":             "true",
		"CACHE_DIR":         "/tmp/cache",
		"UPDATE_SCHEDULE":   "0 0 * * *",
		"CACHE_TTL":         "1h",
		"MAX_CACHE_SIZE":    "2147483648",
		"CLAUDE_API_KEY":    "test-key",
		"CLAUDE_MODEL":      "test-model",
		"CLAUDE_TIMEOUT":    "10m",
		"MAX_CONCURRENT":    "20",
		"WORKER_POOL_SIZE":  "10",
		"ENABLE_ANALYTICS":  "false",
		"ANALYTICS_DB_PATH": "/tmp/analytics.db",
	}

	// Set env vars
	for k, v := range envVars {
		require.NoError(t, os.Setenv(k, v))
		defer func(key string) {
			require.NoError(t, os.Unsetenv(key))
		}(k)
	}

	cfg, err := Load()
	assert.NoError(t, err)

	// Check overridden values
	assert.Equal(t, "9090", cfg.Port)
	assert.True(t, cfg.Debug)
	assert.Equal(t, "/tmp/cache", cfg.CacheDir)
	assert.Equal(t, "0 0 * * *", cfg.UpdateSchedule)
	assert.Equal(t, 1*time.Hour, cfg.CacheTTL)
	assert.Equal(t, int64(2147483648), cfg.MaxCacheSize)
	assert.Equal(t, "test-key", cfg.ClaudeAPIKey)
	assert.Equal(t, "test-model", cfg.ClaudeModel)
	assert.Equal(t, 10*time.Minute, cfg.ClaudeTimeout)
	assert.Equal(t, 20, cfg.MaxConcurrent)
	assert.Equal(t, 10, cfg.WorkerPoolSize)
	assert.False(t, cfg.EnableAnalytics)
	assert.Equal(t, "/tmp/analytics.db", cfg.AnalyticsDBPath)
}

func TestLoadConfigWithAlternativeAPIKey(t *testing.T) {
	// Test with ANTHROPIC_API_KEY when CLAUDE_API_KEY is not set
	require.NoError(t, os.Setenv("ANTHROPIC_API_KEY", "anthropic-key"))
	defer func() {
		require.NoError(t, os.Unsetenv("ANTHROPIC_API_KEY"))
	}()

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "anthropic-key", cfg.ClaudeAPIKey)
}

func TestGetEnv(t *testing.T) {
	// Test with existing env var
	require.NoError(t, os.Setenv("TEST_VAR", "test-value"))
	defer func() {
		require.NoError(t, os.Unsetenv("TEST_VAR"))
	}()

	value := getEnv("TEST_VAR", "default")
	assert.Equal(t, "test-value", value)

	// Test with non-existing env var
	value = getEnv("NON_EXISTENT", "default")
	assert.Equal(t, "default", value)
}

func TestGetBoolEnv(t *testing.T) {
	// Test true values
	trueValues := []string{"true", "True", "TRUE", "1", "yes"}
	for _, v := range trueValues {
		require.NoError(t, os.Setenv("BOOL_VAR", v))
		result := getBoolEnv("BOOL_VAR", false)
		assert.True(t, result, "Value %s should be true", v)
		require.NoError(t, os.Unsetenv("BOOL_VAR"))
	}

	// Test false values
	falseValues := []string{"false", "False", "FALSE", "0", "no"}
	for _, v := range falseValues {
		require.NoError(t, os.Setenv("BOOL_VAR", v))
		result := getBoolEnv("BOOL_VAR", true)
		assert.False(t, result, "Value %s should be false", v)
		require.NoError(t, os.Unsetenv("BOOL_VAR"))
	}

	// Test invalid value (should return default)
	require.NoError(t, os.Setenv("BOOL_VAR", "invalid"))
	result := getBoolEnv("BOOL_VAR", true)
	assert.True(t, result)
	require.NoError(t, os.Unsetenv("BOOL_VAR"))

	// Test non-existent var
	result = getBoolEnv("NON_EXISTENT", true)
	assert.True(t, result)
}

func TestGetIntEnv(t *testing.T) {
	// Test valid integer
	require.NoError(t, os.Setenv("INT_VAR", "42"))
	defer func() {
		require.NoError(t, os.Unsetenv("INT_VAR"))
	}()

	value := getIntEnv("INT_VAR", 0)
	assert.Equal(t, 42, value)

	// Test invalid integer
	require.NoError(t, os.Setenv("INT_VAR", "not-a-number"))
	value = getIntEnv("INT_VAR", 10)
	assert.Equal(t, 10, value)

	// Test non-existent var
	value = getIntEnv("NON_EXISTENT", 99)
	assert.Equal(t, 99, value)
}

func TestGetInt64Env(t *testing.T) {
	// Test valid int64
	require.NoError(t, os.Setenv("INT64_VAR", "9223372036854775807"))
	defer func() {
		require.NoError(t, os.Unsetenv("INT64_VAR"))
	}()

	value := getInt64Env("INT64_VAR", 0)
	assert.Equal(t, int64(9223372036854775807), value)

	// Test invalid int64
	require.NoError(t, os.Setenv("INT64_VAR", "invalid"))
	value = getInt64Env("INT64_VAR", 100)
	assert.Equal(t, int64(100), value)
}

func TestGetDurationEnv(t *testing.T) {
	// Test valid duration
	require.NoError(t, os.Setenv("DURATION_VAR", "5m30s"))
	defer func() {
		require.NoError(t, os.Unsetenv("DURATION_VAR"))
	}()

	value := getDurationEnv("DURATION_VAR", time.Second)
	assert.Equal(t, 5*time.Minute+30*time.Second, value)

	// Test invalid duration
	require.NoError(t, os.Setenv("DURATION_VAR", "invalid"))
	value = getDurationEnv("DURATION_VAR", time.Hour)
	assert.Equal(t, time.Hour, value)

	// Test non-existent var
	value = getDurationEnv("NON_EXISTENT", time.Minute)
	assert.Equal(t, time.Minute, value)
}

package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the service.
type Config struct {
	// Server configuration
	Port    string
	Version string
	Debug   bool

	// Cache configuration
	CacheDir        string
	UpdateSchedule  string
	CacheTTL        time.Duration
	MaxCacheSize    int64

	// Claude API configuration
	ClaudeAPIKey   string
	ClaudeModel    string
	ClaudeTimeout  time.Duration

	// Performance configuration
	MaxConcurrent  int
	WorkerPoolSize int

	// Analytics configuration
	EnableAnalytics bool
	AnalyticsDBPath string
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{
		// Defaults
		Port:            getEnv("PORT", "8080"),
		Version:         getEnv("VERSION", "1.0.0"),
		Debug:           getBoolEnv("DEBUG", false),
		CacheDir:        getEnv("CACHE_DIR", "./cache"),
		UpdateSchedule:  getEnv("UPDATE_SCHEDULE", "0 2 * * 0"), // Weekly at 2 AM
		CacheTTL:        getDurationEnv("CACHE_TTL", 7*24*time.Hour),
		MaxCacheSize:    getInt64Env("MAX_CACHE_SIZE", 1<<30), // 1GB
		ClaudeAPIKey:    getEnv("CLAUDE_API_KEY", ""),
		ClaudeModel:     getEnv("CLAUDE_MODEL", "claude-3-5-sonnet-20241022"),
		ClaudeTimeout:   getDurationEnv("CLAUDE_TIMEOUT", 5*time.Minute),
		MaxConcurrent:   getIntEnv("MAX_CONCURRENT", 10),
		WorkerPoolSize:  getIntEnv("WORKER_POOL_SIZE", 5),
		EnableAnalytics: getBoolEnv("ENABLE_ANALYTICS", true),
		AnalyticsDBPath: getEnv("ANALYTICS_DB_PATH", "./analytics.db"),
	}

	// Validate required configuration
	if cfg.ClaudeAPIKey == "" {
		cfg.ClaudeAPIKey = getEnv("ANTHROPIC_API_KEY", "") // Alternative env var
	}

	return cfg, nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

func getIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func getInt64Env(key string, defaultValue int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	int64Value, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return int64Value
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}
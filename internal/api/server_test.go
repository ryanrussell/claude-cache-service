package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/config"
)

func setupTestServer(t *testing.T) (*Server, *cache.Manager) {
	gin.SetMode(gin.TestMode)

	tempDir := t.TempDir()
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)

	cfg := &config.Config{
		Port:    "8080",
		Version: "test",
		Debug:   false,
	}

	cacheManager, err := cache.NewManager(tempDir, logger)
	require.NoError(t, err)

	server := NewServer(cfg, cacheManager, logger)
	return server, cacheManager
}

func TestHealthEndpoint(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "test", response["version"])
	assert.NotNil(t, response["cache"])
}

func TestCacheSummaryEndpoint(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Add some test data
	err := cacheManager.Set("test-key", "test-value", 0)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "/api/v1/cache/summary", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Cache summary retrieved successfully", response.Message)
	assert.NotNil(t, response.Data)
}

func TestGetProjectCache(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Add test project data
	projectData := `{"project": "test-project", "data": "test"}`
	err := cacheManager.Set("project:test-project", projectData, 0)
	require.NoError(t, err)

	// Test successful get
	req, _ := http.NewRequest("GET", "/api/v1/cache/project/test-project", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, projectData, response.Data)

	// Test not found
	req, _ = http.NewRequest("GET", "/api/v1/cache/project/non-existent", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errorResponse ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, "not_found", errorResponse.Error)
}

func TestGetSDKCache(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Add test SDK data
	sdkData := `{"sdk": "sentry-go", "version": "1.0.0"}`
	err := cacheManager.Set("sdk:sentry-go", sdkData, 0)
	require.NoError(t, err)

	// Test successful get
	req, _ := http.NewRequest("GET", "/api/v1/cache/sdk/sentry-go", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, sdkData, response.Data)
}

func TestDeleteCacheKey(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Add test data
	err := cacheManager.Set("delete-me", "value", 0)
	require.NoError(t, err)

	// Delete the key
	req, _ := http.NewRequest("DELETE", "/api/v1/cache/key/delete-me", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify key is deleted
	_, err = cacheManager.Get("delete-me")
	assert.Error(t, err)
}

func TestRequestIDMiddleware(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Test with provided request ID
	req, _ := http.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Request-ID", "test-request-id")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, "test-request-id", w.Header().Get("X-Request-ID"))

	// Test without request ID (should generate one)
	req, _ = http.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	requestID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, requestID)
	assert.Len(t, requestID, 36) // UUID length
}

func TestCORSMiddleware(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Test CORS headers
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))

	// Test OPTIONS request
	req, _ = http.NewRequest("OPTIONS", "/health", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRefreshCache(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	req, _ := http.NewRequest("POST", "/api/v1/cache/refresh", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response SuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Cache refresh initiated", response.Message)
}

func TestAnalyticsEndpoints(t *testing.T) {
	server, cacheManager := setupTestServer(t)
	defer func() {
		err := cacheManager.Close()
		require.NoError(t, err)
	}()

	// Test usage analytics
	req, _ := http.NewRequest("GET", "/api/v1/analytics/usage", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test performance analytics
	req, _ = http.NewRequest("GET", "/api/v1/analytics/performance", nil)
	w = httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

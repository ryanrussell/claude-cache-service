// +build integration

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationFullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverAddr := "http://localhost:8080"
	
	// Wait for server to be ready
	waitForServer(t, serverAddr)

	// Test health endpoint
	resp, err := http.Get(serverAddr + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test cache summary
	resp, err = http.Get(serverAddr + "/api/v1/cache/summary")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var summary map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&summary)
	require.NoError(t, err)
	assert.NotNil(t, summary["data"])

	// Wait for initial cache update
	time.Sleep(2 * time.Second)

	// Test SDK cache retrieval
	resp, err = http.Get(serverAddr + "/api/v1/cache/sdk/sentry-go")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		var sdkResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&sdkResponse)
		require.NoError(t, err)
		assert.NotNil(t, sdkResponse["data"])
	}

	// Test project cache retrieval
	resp, err = http.Get(serverAddr + "/api/v1/cache/project/gremlin-arrow-flight")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		var projectResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&projectResponse)
		require.NoError(t, err)
		assert.NotNil(t, projectResponse["data"])
	}
}

func waitForServer(t *testing.T, addr string) {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(addr + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatal("Server did not start in time")
}

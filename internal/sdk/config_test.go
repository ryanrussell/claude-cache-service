package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigs(t *testing.T) {
	configs, err := LoadConfigs()
	require.NoError(t, err)
	assert.NotNil(t, configs)
	assert.NotEmpty(t, configs.SDKs)

	// Check that we have at least some expected SDKs
	foundGo := false
	foundPython := false
	foundJS := false

	for _, sdk := range configs.SDKs {
		switch sdk.Name {
		case "sentry-go":
			foundGo = true
			assert.Equal(t, "go", sdk.Language)
			assert.Contains(t, sdk.Patterns, "*.go")
		case "sentry-python":
			foundPython = true
			assert.Equal(t, "python", sdk.Language)
			assert.Contains(t, sdk.Patterns, "*.py")
		case "sentry-javascript":
			foundJS = true
			assert.Equal(t, "javascript", sdk.Language)
		}
	}

	assert.True(t, foundGo, "sentry-go should be in the config")
	assert.True(t, foundPython, "sentry-python should be in the config")
	assert.True(t, foundJS, "sentry-javascript should be in the config")
}

func TestGetActiveSDKs(t *testing.T) {
	configs, err := LoadConfigs()
	require.NoError(t, err)

	activeSDKs := configs.GetActiveSDKs()
	assert.NotEmpty(t, activeSDKs)

	// All returned SDKs should be active
	for _, sdk := range activeSDKs {
		assert.True(t, sdk.Active)
	}
}

func TestFindSDK(t *testing.T) {
	configs, err := LoadConfigs()
	require.NoError(t, err)

	// Test finding existing SDK
	sdk, found := configs.FindSDK("sentry-go")
	assert.True(t, found)
	assert.NotNil(t, sdk)
	assert.Equal(t, "sentry-go", sdk.Name)

	// Test finding non-existent SDK
	sdk, found = configs.FindSDK("non-existent-sdk")
	assert.False(t, found)
	assert.Nil(t, sdk)
}

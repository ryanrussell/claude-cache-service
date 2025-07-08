package sdk

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config represents an SDK configuration
type Config struct {
	Name     string   `yaml:"name"`
	URL      string   `yaml:"url"`
	Language string   `yaml:"language"`
	Patterns []string `yaml:"patterns"`
	KeyFiles []string `yaml:"key_files,omitempty"`
	Branch   string   `yaml:"branch,omitempty"`
	Active   bool     `yaml:"active"`
}

// ConfigList represents the list of all SDK configurations
type ConfigList struct {
	SDKs []Config `yaml:"sdks"`
}

//go:embed sdks.yaml
var sdksYAML string

// LoadConfigs loads the SDK configurations from the embedded YAML
func LoadConfigs() (*ConfigList, error) {
	var configs ConfigList
	if err := yaml.Unmarshal([]byte(sdksYAML), &configs); err != nil {
		return nil, fmt.Errorf("failed to parse SDK configs: %w", err)
	}
	return &configs, nil
}

// GetActiveSDKs returns only the active SDK configurations
func (c *ConfigList) GetActiveSDKs() []Config {
	var active []Config
	for _, sdk := range c.SDKs {
		if sdk.Active {
			active = append(active, sdk)
		}
	}
	return active
}

// FindSDK finds an SDK configuration by name
func (c *ConfigList) FindSDK(name string) (*Config, bool) {
	for _, sdk := range c.SDKs {
		if sdk.Name == name {
			return &sdk, true
		}
	}
	return nil, false
}

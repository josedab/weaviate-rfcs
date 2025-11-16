//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// DevelopmentConfig represents the development mode configuration
// This implements RFC 0015 for improved developer experience
type DevelopmentConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Storage configuration for development
	Storage StorageConfig `json:"storage" yaml:"storage"`

	// Schema configuration
	Schema SchemaConfig `json:"schema" yaml:"schema"`

	// Vectorizer configuration
	Vectorizers map[string]VectorizerConfig `json:"vectorizers" yaml:"vectorizers"`

	// Fixtures configuration for test data
	Fixtures FixturesConfig `json:"fixtures" yaml:"fixtures"`

	// Hot reload configuration
	HotReload HotReloadConfig `json:"hotReload" yaml:"hotReload"`

	// Debug configuration
	Debug DebugDevConfig `json:"debug" yaml:"debug"`
}

// StorageConfig defines storage behavior in development mode
type StorageConfig struct {
	// Type of storage: "memory", "disk", or "hybrid"
	Type string `json:"type" yaml:"type"`

	// Path for disk storage (only used if type is "disk" or "hybrid")
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Whether to persist data between restarts
	Persist bool `json:"persist" yaml:"persist"`
}

// SchemaConfig defines schema behavior in development mode
type SchemaConfig struct {
	// AutoReload enables automatic schema reloading when files change
	AutoReload bool `json:"autoReload" yaml:"autoReload"`

	// WatchDirectory is the directory to watch for schema changes
	WatchDirectory string `json:"watchDirectory,omitempty" yaml:"watchDirectory,omitempty"`

	// ValidateOnLoad validates schema on load
	ValidateOnLoad bool `json:"validateOnLoad" yaml:"validateOnLoad"`
}

// VectorizerConfig defines vectorizer behavior in development mode
type VectorizerConfig struct {
	// Mock enables mock vectorizer (returns random vectors)
	Mock bool `json:"mock" yaml:"mock"`

	// Dimensions for mock vectors
	Dimensions int `json:"dimensions,omitempty" yaml:"dimensions,omitempty"`

	// MockLatency adds artificial latency (in milliseconds) to simulate real vectorizers
	MockLatency int `json:"mockLatency,omitempty" yaml:"mockLatency,omitempty"`
}

// FixturesConfig defines test data fixtures
type FixturesConfig struct {
	// Enabled enables fixture loading
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Directory containing fixture files
	Directory string `json:"directory,omitempty" yaml:"directory,omitempty"`

	// AutoLoad automatically loads fixtures on startup
	AutoLoad bool `json:"autoLoad" yaml:"autoLoad"`

	// ClearBeforeLoad clears existing data before loading fixtures
	ClearBeforeLoad bool `json:"clearBeforeLoad" yaml:"clearBeforeLoad"`
}

// HotReloadConfig defines hot reload behavior
type HotReloadConfig struct {
	// Enabled enables hot reload
	Enabled bool `json:"enabled" yaml:"enabled"`

	// WatchPaths are paths to watch for changes
	WatchPaths []string `json:"watchPaths,omitempty" yaml:"watchPaths,omitempty"`

	// DebounceMs is the debounce time in milliseconds before reloading
	DebounceMs int `json:"debounceMs,omitempty" yaml:"debounceMs,omitempty"`
}

// DebugDevConfig defines debug-specific development settings
type DebugDevConfig struct {
	// EnableQueryExplain enables query explain endpoint
	EnableQueryExplain bool `json:"enableQueryExplain" yaml:"enableQueryExplain"`

	// LogAllQueries logs all queries with execution plans
	LogAllQueries bool `json:"logAllQueries" yaml:"logAllQueries"`

	// EnableProfiling enables pprof endpoints
	EnableProfiling bool `json:"enableProfiling" yaml:"enableProfiling"`

	// EnableMetrics enables detailed metrics
	EnableMetrics bool `json:"enableMetrics" yaml:"enableMetrics"`
}

// DefaultDevelopmentConfig returns the default development configuration
func DefaultDevelopmentConfig() DevelopmentConfig {
	return DevelopmentConfig{
		Enabled: false,
		Storage: StorageConfig{
			Type:    "memory",
			Persist: false,
		},
		Schema: SchemaConfig{
			AutoReload:     false,
			ValidateOnLoad: true,
		},
		Vectorizers: make(map[string]VectorizerConfig),
		Fixtures: FixturesConfig{
			Enabled:         false,
			AutoLoad:        false,
			ClearBeforeLoad: true,
		},
		HotReload: HotReloadConfig{
			Enabled:    false,
			DebounceMs: 1000,
		},
		Debug: DebugDevConfig{
			EnableQueryExplain: true,
			LogAllQueries:      false,
			EnableProfiling:    false,
			EnableMetrics:      true,
		},
	}
}

// LoadDevelopmentConfig loads development configuration from a file
func LoadDevelopmentConfig(path string) (*DevelopmentConfig, error) {
	// Start with defaults
	config := DefaultDevelopmentConfig()

	// If no path provided, look for weaviate.dev.yaml in current directory
	if path == "" {
		path = "weaviate.dev.yaml"
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return &config, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read development config file")
	}

	// Parse YAML
	var fileConfig struct {
		Development DevelopmentConfig `yaml:"development"`
	}
	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return nil, errors.Wrap(err, "failed to parse development config")
	}

	// Override defaults with file config
	if fileConfig.Development.Enabled {
		config = fileConfig.Development
	}

	// Resolve relative paths
	if config.Schema.WatchDirectory != "" && !filepath.IsAbs(config.Schema.WatchDirectory) {
		baseDir := filepath.Dir(path)
		config.Schema.WatchDirectory = filepath.Join(baseDir, config.Schema.WatchDirectory)
	}

	if config.Fixtures.Directory != "" && !filepath.IsAbs(config.Fixtures.Directory) {
		baseDir := filepath.Dir(path)
		config.Fixtures.Directory = filepath.Join(baseDir, config.Fixtures.Directory)
	}

	return &config, nil
}

// Validate checks if the development configuration is valid
func (c *DevelopmentConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Validate storage type
	if c.Storage.Type != "memory" && c.Storage.Type != "disk" && c.Storage.Type != "hybrid" {
		return errors.Errorf("invalid storage type: %s (must be 'memory', 'disk', or 'hybrid')", c.Storage.Type)
	}

	// Validate schema watch directory if auto-reload is enabled
	if c.Schema.AutoReload && c.Schema.WatchDirectory == "" {
		return errors.New("schema.watchDirectory must be set when schema.autoReload is enabled")
	}

	// Validate fixtures directory if enabled
	if c.Fixtures.Enabled && c.Fixtures.Directory == "" {
		return errors.New("fixtures.directory must be set when fixtures are enabled")
	}

	// Validate hot reload paths
	if c.HotReload.Enabled && len(c.HotReload.WatchPaths) == 0 {
		return errors.New("hotReload.watchPaths must be set when hot reload is enabled")
	}

	return nil
}

// IsDevelopmentMode returns true if development mode is enabled
func (c *DevelopmentConfig) IsDevelopmentMode() bool {
	return c.Enabled
}

// IsInMemoryStorage returns true if using in-memory storage
func (c *DevelopmentConfig) IsInMemoryStorage() bool {
	return c.Storage.Type == "memory"
}

// ShouldAutoLoadFixtures returns true if fixtures should be auto-loaded
func (c *DevelopmentConfig) ShouldAutoLoadFixtures() bool {
	return c.Enabled && c.Fixtures.Enabled && c.Fixtures.AutoLoad
}

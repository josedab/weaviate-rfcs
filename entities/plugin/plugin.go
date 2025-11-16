package plugin

import (
	"context"
	"time"
)

// Plugin is the base interface that all plugins must implement
type Plugin interface {
	// Metadata
	Name() string
	Version() string
	Type() PluginType

	// Lifecycle
	Init(ctx context.Context, config Config) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Health
	Health() HealthStatus
}

// PluginType represents the type of plugin
type PluginType string

const (
	PluginTypeVectorizer  PluginType = "vectorizer"
	PluginTypeTransformer PluginType = "transformer"
	PluginTypeReranker    PluginType = "reranker"
	PluginTypeGenerator   PluginType = "generator"
	PluginTypeStorage     PluginType = "storage"
	PluginTypeAuth        PluginType = "auth"
)

// HealthStatus represents the health state of a plugin
type HealthStatus struct {
	Status      HealthState
	Message     string
	LastChecked time.Time
}

// HealthState represents the health state
type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateUnknown   HealthState = "unknown"
)

// Config represents plugin configuration
type Config map[string]interface{}

// GetString retrieves a string value from config
func (c Config) GetString(key string) (string, error) {
	val, ok := c[key]
	if !ok {
		return "", ErrConfigKeyNotFound{Key: key}
	}
	str, ok := val.(string)
	if !ok {
		return "", ErrConfigTypeMismatch{Key: key, Expected: "string"}
	}
	return str, nil
}

// GetInt retrieves an int value from config
func (c Config) GetInt(key string) (int, error) {
	val, ok := c[key]
	if !ok {
		return 0, ErrConfigKeyNotFound{Key: key}
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, ErrConfigTypeMismatch{Key: key, Expected: "int"}
	}
}

// GetBool retrieves a bool value from config
func (c Config) GetBool(key string) (bool, error) {
	val, ok := c[key]
	if !ok {
		return false, ErrConfigKeyNotFound{Key: key}
	}
	b, ok := val.(bool)
	if !ok {
		return false, ErrConfigTypeMismatch{Key: key, Expected: "bool"}
	}
	return b, nil
}

// GetStringWithDefault retrieves a string value or returns default
func (c Config) GetStringWithDefault(key, defaultValue string) string {
	val, err := c.GetString(key)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetIntWithDefault retrieves an int value or returns default
func (c Config) GetIntWithDefault(key string, defaultValue int) int {
	val, err := c.GetInt(key)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetBoolWithDefault retrieves a bool value or returns default
func (c Config) GetBoolWithDefault(key string, defaultValue bool) bool {
	val, err := c.GetBool(key)
	if err != nil {
		return defaultValue
	}
	return val
}

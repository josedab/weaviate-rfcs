package plugin

import "fmt"

// ErrConfigKeyNotFound is returned when a config key is not found
type ErrConfigKeyNotFound struct {
	Key string
}

func (e ErrConfigKeyNotFound) Error() string {
	return fmt.Sprintf("config key not found: %s", e.Key)
}

// ErrConfigTypeMismatch is returned when config value type doesn't match
type ErrConfigTypeMismatch struct {
	Key      string
	Expected string
}

func (e ErrConfigTypeMismatch) Error() string {
	return fmt.Sprintf("config type mismatch for key %s: expected %s", e.Key, e.Expected)
}

// ErrPluginNotFound is returned when a plugin is not found
type ErrPluginNotFound struct {
	Name string
}

func (e ErrPluginNotFound) Error() string {
	return fmt.Sprintf("plugin not found: %s", e.Name)
}

// ErrPluginAlreadyExists is returned when trying to register an existing plugin
type ErrPluginAlreadyExists struct {
	Name string
}

func (e ErrPluginAlreadyExists) Error() string {
	return fmt.Sprintf("plugin already exists: %s", e.Name)
}

// ErrUnsupportedRuntime is returned when plugin runtime is not supported
type ErrUnsupportedRuntime struct {
	Runtime string
}

func (e ErrUnsupportedRuntime) Error() string {
	return fmt.Sprintf("unsupported plugin runtime: %s", e.Runtime)
}

// ErrResourceLimitExceeded is returned when a resource limit is exceeded
type ErrResourceLimitExceeded struct {
	Resource string
	Limit    string
}

func (e ErrResourceLimitExceeded) Error() string {
	return fmt.Sprintf("resource limit exceeded for %s: %s", e.Resource, e.Limit)
}

// ErrCPULimitExceeded is returned when CPU time limit is exceeded
var ErrCPULimitExceeded = fmt.Errorf("CPU time limit exceeded")

// ErrMemoryLimitExceeded is returned when memory limit is exceeded
var ErrMemoryLimitExceeded = fmt.Errorf("memory limit exceeded")

// ErrInvalidManifest is returned when plugin manifest is invalid
type ErrInvalidManifest struct {
	Reason string
}

func (e ErrInvalidManifest) Error() string {
	return fmt.Sprintf("invalid plugin manifest: %s", e.Reason)
}

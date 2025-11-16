package plugin

import (
	"context"
	"sync"
)

// Registry manages registered plugins
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register registers a plugin with the given name
func (r *Registry) Register(name string, plugin Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; exists {
		return ErrPluginAlreadyExists{Name: name}
	}

	r.plugins[name] = plugin
	return nil
}

// Unregister removes a plugin from the registry
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return ErrPluginNotFound{Name: name}
	}

	delete(r.plugins, name)
	return nil
}

// Get retrieves a plugin by name
func (r *Registry) Get(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	if !exists {
		return nil, ErrPluginNotFound{Name: name}
	}

	return plugin, nil
}

// GetVectorizer retrieves a vectorizer plugin by name
func (r *Registry) GetVectorizer(name string) (VectorizerPlugin, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	vectorizer, ok := plugin.(VectorizerPlugin)
	if !ok {
		return nil, ErrInvalidManifest{Reason: "plugin is not a vectorizer"}
	}

	return vectorizer, nil
}

// GetTransformer retrieves a transformer plugin by name
func (r *Registry) GetTransformer(name string) (TransformerPlugin, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	transformer, ok := plugin.(TransformerPlugin)
	if !ok {
		return nil, ErrInvalidManifest{Reason: "plugin is not a transformer"}
	}

	return transformer, nil
}

// GetReranker retrieves a reranker plugin by name
func (r *Registry) GetReranker(name string) (RerankerPlugin, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	reranker, ok := plugin.(RerankerPlugin)
	if !ok {
		return nil, ErrInvalidManifest{Reason: "plugin is not a reranker"}
	}

	return reranker, nil
}

// GetGenerator retrieves a generator plugin by name
func (r *Registry) GetGenerator(name string) (GeneratorPlugin, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	generator, ok := plugin.(GeneratorPlugin)
	if !ok {
		return nil, ErrInvalidManifest{Reason: "plugin is not a generator"}
	}

	return generator, nil
}

// GetStorage retrieves a storage plugin by name
func (r *Registry) GetStorage(name string) (StoragePlugin, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	storage, ok := plugin.(StoragePlugin)
	if !ok {
		return nil, ErrInvalidManifest{Reason: "plugin is not a storage plugin"}
	}

	return storage, nil
}

// GetAuth retrieves an auth plugin by name
func (r *Registry) GetAuth(name string) (AuthPlugin, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	auth, ok := plugin.(AuthPlugin)
	if !ok {
		return nil, ErrInvalidManifest{Reason: "plugin is not an auth plugin"}
	}

	return auth, nil
}

// List returns all registered plugin names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}

	return names
}

// ListByType returns all plugins of a specific type
func (r *Registry) ListByType(pluginType PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []Plugin
	for _, plugin := range r.plugins {
		if plugin.Type() == pluginType {
			plugins = append(plugins, plugin)
		}
	}

	return plugins
}

// Swap atomically replaces an old plugin with a new one (for hot-reload)
func (r *Registry) Swap(name string, oldPlugin, newPlugin Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, exists := r.plugins[name]
	if !exists {
		return ErrPluginNotFound{Name: name}
	}

	// Verify we're swapping the correct plugin
	if current != oldPlugin {
		return ErrInvalidManifest{Reason: "plugin mismatch during swap"}
	}

	r.plugins[name] = newPlugin
	return nil
}

// HealthCheck checks health of all registered plugins
func (r *Registry) HealthCheck(ctx context.Context) map[string]HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]HealthStatus)
	for name, plugin := range r.plugins {
		results[name] = plugin.Health()
	}

	return results
}

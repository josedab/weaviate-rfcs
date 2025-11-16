package plugin

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/weaviate/weaviate/entities/plugin"
)

// Manager orchestrates all plugin operations
type Manager struct {
	registry  *plugin.Registry
	loader    *Loader
	lifecycle *LifecycleManager
	sandboxes map[string]*Sandbox
}

// NewManager creates a new plugin manager
func NewManager() *Manager {
	return &Manager{
		registry:  plugin.NewRegistry(),
		loader:    NewLoader(),
		lifecycle: NewLifecycleManager(),
		sandboxes: make(map[string]*Sandbox),
	}
}

// Load loads a plugin from a manifest file
func (m *Manager) Load(ctx context.Context, manifestPath string) error {
	// Load manifest
	manifest, err := plugin.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	return m.LoadFromManifest(ctx, manifest)
}

// LoadFromManifest loads a plugin from a manifest object
func (m *Manager) LoadFromManifest(ctx context.Context, manifest *plugin.Manifest) error {
	name := manifest.Metadata.Name

	// Check if plugin already exists
	if _, err := m.registry.Get(name); err == nil {
		return plugin.ErrPluginAlreadyExists{Name: name}
	}

	// Create sandbox
	limits, err := ParseResourceLimits(manifest.Spec.Resources)
	if err != nil {
		return fmt.Errorf("failed to parse resource limits: %w", err)
	}
	sandbox := NewSandbox(name, limits)
	m.sandboxes[name] = sandbox

	// Load plugin
	p, err := m.loader.Load(ctx, manifest, sandbox)
	if err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	// Track in lifecycle manager
	m.lifecycle.Track(name, p)

	// Build config from manifest
	config := buildConfig(manifest)

	// Initialize plugin
	if err := m.lifecycle.Initialize(ctx, name, config); err != nil {
		m.lifecycle.Untrack(name)
		delete(m.sandboxes, name)
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Start plugin
	if err := m.lifecycle.Start(ctx, name); err != nil {
		m.lifecycle.Untrack(name)
		delete(m.sandboxes, name)
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	// Register plugin
	if err := m.registry.Register(name, p); err != nil {
		// Stop plugin if registration fails
		_ = m.lifecycle.Stop(ctx, name)
		m.lifecycle.Untrack(name)
		delete(m.sandboxes, name)
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	return nil
}

// Unload unloads a plugin
func (m *Manager) Unload(ctx context.Context, name string) error {
	// Get plugin
	p, err := m.registry.Get(name)
	if err != nil {
		return err
	}

	// Stop plugin
	if err := m.lifecycle.Stop(ctx, name); err != nil {
		return fmt.Errorf("failed to stop plugin: %w", err)
	}

	// Unregister plugin
	if err := m.registry.Unregister(name); err != nil {
		return fmt.Errorf("failed to unregister plugin: %w", err)
	}

	// Untrack from lifecycle
	m.lifecycle.Untrack(name)

	// Remove sandbox
	delete(m.sandboxes, name)

	_ = p // plugin reference

	return nil
}

// Reload reloads a plugin (hot-reload)
func (m *Manager) Reload(ctx context.Context, name string, newManifestPath string) error {
	// Load new manifest
	newManifest, err := plugin.LoadManifest(newManifestPath)
	if err != nil {
		return fmt.Errorf("failed to load new manifest: %w", err)
	}

	return m.ReloadFromManifest(ctx, name, newManifest)
}

// ReloadFromManifest reloads a plugin from a manifest object
func (m *Manager) ReloadFromManifest(ctx context.Context, name string, newManifest *plugin.Manifest) error {
	// Get current plugin
	oldPlugin, err := m.registry.Get(name)
	if err != nil {
		return err
	}

	// Create new sandbox
	limits, err := ParseResourceLimits(newManifest.Spec.Resources)
	if err != nil {
		return fmt.Errorf("failed to parse resource limits: %w", err)
	}
	newSandbox := NewSandbox(name, limits)

	// Load new plugin
	newPlugin, err := m.loader.Load(ctx, newManifest, newSandbox)
	if err != nil {
		return fmt.Errorf("failed to load new plugin: %w", err)
	}

	// Track new plugin
	newName := name + "-new"
	m.lifecycle.Track(newName, newPlugin)

	// Build config
	config := buildConfig(newManifest)

	// Initialize new plugin
	if err := m.lifecycle.Initialize(ctx, newName, config); err != nil {
		m.lifecycle.Untrack(newName)
		return fmt.Errorf("failed to initialize new plugin: %w", err)
	}

	// Start new plugin
	if err := m.lifecycle.Start(ctx, newName); err != nil {
		m.lifecycle.Untrack(newName)
		return fmt.Errorf("failed to start new plugin: %w", err)
	}

	// Drain requests from old plugin
	if err := m.lifecycle.DrainRequests(oldPlugin); err != nil {
		// Stop new plugin and abort reload
		_ = m.lifecycle.Stop(ctx, newName)
		m.lifecycle.Untrack(newName)
		return fmt.Errorf("failed to drain requests from old plugin: %w", err)
	}

	// Swap plugins atomically
	if err := m.registry.Swap(name, oldPlugin, newPlugin); err != nil {
		// Stop new plugin and abort reload
		_ = m.lifecycle.Stop(ctx, newName)
		m.lifecycle.Untrack(newName)
		return fmt.Errorf("failed to swap plugins: %w", err)
	}

	// Stop old plugin
	if err := oldPlugin.Stop(ctx); err != nil {
		// Log error but don't fail - new plugin is already active
		fmt.Printf("warning: failed to stop old plugin: %v\n", err)
	}

	// Update lifecycle tracking
	m.lifecycle.Untrack(newName)
	m.lifecycle.Track(name, newPlugin)
	if err := m.lifecycle.SetState(name, StateRunning); err != nil {
		fmt.Printf("warning: failed to set plugin state: %v\n", err)
	}

	// Update sandbox
	m.sandboxes[name] = newSandbox

	return nil
}

// Get retrieves a plugin by name
func (m *Manager) Get(name string) (plugin.Plugin, error) {
	return m.registry.Get(name)
}

// GetVectorizer retrieves a vectorizer plugin
func (m *Manager) GetVectorizer(name string) (plugin.VectorizerPlugin, error) {
	return m.registry.GetVectorizer(name)
}

// GetTransformer retrieves a transformer plugin
func (m *Manager) GetTransformer(name string) (plugin.TransformerPlugin, error) {
	return m.registry.GetTransformer(name)
}

// GetReranker retrieves a reranker plugin
func (m *Manager) GetReranker(name string) (plugin.RerankerPlugin, error) {
	return m.registry.GetReranker(name)
}

// GetGenerator retrieves a generator plugin
func (m *Manager) GetGenerator(name string) (plugin.GeneratorPlugin, error) {
	return m.registry.GetGenerator(name)
}

// GetStorage retrieves a storage plugin
func (m *Manager) GetStorage(name string) (plugin.StoragePlugin, error) {
	return m.registry.GetStorage(name)
}

// GetAuth retrieves an auth plugin
func (m *Manager) GetAuth(name string) (plugin.AuthPlugin, error) {
	return m.registry.GetAuth(name)
}

// List returns all registered plugin names
func (m *Manager) List() []string {
	return m.registry.List()
}

// ListByType returns all plugins of a specific type
func (m *Manager) ListByType(pluginType plugin.PluginType) []plugin.Plugin {
	return m.registry.ListByType(pluginType)
}

// HealthCheck checks health of all plugins
func (m *Manager) HealthCheck(ctx context.Context) map[string]plugin.HealthStatus {
	return m.registry.HealthCheck(ctx)
}

// LoadDirectory loads all plugins from a directory
func (m *Manager) LoadDirectory(ctx context.Context, dir string) error {
	// Find all plugin.yaml files in directory
	manifests, err := filepath.Glob(filepath.Join(dir, "*/plugin.yaml"))
	if err != nil {
		return fmt.Errorf("failed to find plugin manifests: %w", err)
	}

	// Load each plugin
	for _, manifestPath := range manifests {
		if err := m.Load(ctx, manifestPath); err != nil {
			return fmt.Errorf("failed to load plugin from %s: %w", manifestPath, err)
		}
	}

	return nil
}

// buildConfig builds plugin config from manifest
func buildConfig(manifest *plugin.Manifest) plugin.Config {
	config := make(plugin.Config)

	// Add default values from schema
	for key, schema := range manifest.Spec.Config {
		if schema.Default != nil {
			config[key] = schema.Default
		}
	}

	return config
}

// Registry returns the plugin registry
func (m *Manager) Registry() *plugin.Registry {
	return m.registry
}

// Lifecycle returns the lifecycle manager
func (m *Manager) Lifecycle() *LifecycleManager {
	return m.lifecycle
}

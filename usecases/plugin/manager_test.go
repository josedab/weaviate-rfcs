package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/entities/plugin"
)

// MockPlugin is a mock implementation for testing
type MockPlugin struct {
	name    string
	version string
	pType   plugin.PluginType
	started bool
	stopped bool
}

func (m *MockPlugin) Name() string {
	return m.name
}

func (m *MockPlugin) Version() string {
	return m.version
}

func (m *MockPlugin) Type() plugin.PluginType {
	return m.pType
}

func (m *MockPlugin) Init(ctx context.Context, config plugin.Config) error {
	return nil
}

func (m *MockPlugin) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *MockPlugin) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func (m *MockPlugin) Health() plugin.HealthStatus {
	return plugin.HealthStatus{
		Status:      plugin.HealthStateHealthy,
		Message:     "mock plugin healthy",
		LastChecked: time.Now(),
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager()
	require.NotNil(t, manager)
	require.NotNil(t, manager.registry)
	require.NotNil(t, manager.loader)
	require.NotNil(t, manager.lifecycle)
}

func TestParseResourceLimits(t *testing.T) {
	tests := []struct {
		name     string
		input    plugin.ResourceLimits
		wantErr  bool
		checkMem uint64
	}{
		{
			name:     "parse memory in Gi",
			input:    plugin.ResourceLimits{Memory: "2Gi"},
			wantErr:  false,
			checkMem: 2 * 1024 * 1024 * 1024,
		},
		{
			name:     "parse memory in Mi",
			input:    plugin.ResourceLimits{Memory: "512Mi"},
			wantErr:  false,
			checkMem: 512 * 1024 * 1024,
		},
		{
			name:     "parse memory in G",
			input:    plugin.ResourceLimits{Memory: "1G"},
			wantErr:  false,
			checkMem: 1 * 1000 * 1000 * 1000,
		},
		{
			name:     "default values when empty",
			input:    plugin.ResourceLimits{},
			wantErr:  false,
			checkMem: 2 * 1024 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits, err := ParseResourceLimits(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.checkMem, limits.MaxMemory)
		})
	}
}

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name     string
		manifest *plugin.Manifest
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid manifest",
			manifest: &plugin.Manifest{
				APIVersion: "weaviate.io/v1",
				Kind:       "Plugin",
				Metadata: plugin.ManifestMetadata{
					Name:    "test-plugin",
					Version: "1.0.0",
				},
				Spec: plugin.ManifestSpec{
					Type:    plugin.PluginTypeVectorizer,
					Runtime: plugin.RuntimeWASM,
					Binary:  "./test.wasm",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			manifest: &plugin.Manifest{
				APIVersion: "weaviate.io/v1",
				Kind:       "Plugin",
				Metadata: plugin.ManifestMetadata{
					Version: "1.0.0",
				},
				Spec: plugin.ManifestSpec{
					Type:    plugin.PluginTypeVectorizer,
					Runtime: plugin.RuntimeWASM,
					Binary:  "./test.wasm",
				},
			},
			wantErr: true,
			errMsg:  "metadata.name is required",
		},
		{
			name: "invalid runtime",
			manifest: &plugin.Manifest{
				APIVersion: "weaviate.io/v1",
				Kind:       "Plugin",
				Metadata: plugin.ManifestMetadata{
					Name:    "test-plugin",
					Version: "1.0.0",
				},
				Spec: plugin.ManifestSpec{
					Type:    plugin.PluginTypeVectorizer,
					Runtime: "invalid",
					Binary:  "./test.wasm",
				},
			},
			wantErr: true,
			errMsg:  "invalid runtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLifecycleManager(t *testing.T) {
	lm := NewLifecycleManager()
	mock := &MockPlugin{
		name:    "test-plugin",
		version: "1.0.0",
		pType:   plugin.PluginTypeVectorizer,
	}

	t.Run("track and get state", func(t *testing.T) {
		lm.Track("test", mock)
		state, err := lm.GetState("test")
		require.NoError(t, err)
		assert.Equal(t, StateLoading, state)
	})

	t.Run("set state", func(t *testing.T) {
		err := lm.SetState("test", StateRunning)
		require.NoError(t, err)

		state, err := lm.GetState("test")
		require.NoError(t, err)
		assert.Equal(t, StateRunning, state)
	})

	t.Run("track in-flight requests", func(t *testing.T) {
		err := lm.BeginRequest("test")
		require.NoError(t, err)

		count, err := lm.GetInFlightRequests("test")
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		lm.EndRequest("test")

		count, err = lm.GetInFlightRequests("test")
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("untrack plugin", func(t *testing.T) {
		lm.Untrack("test")
		_, err := lm.GetState("test")
		require.Error(t, err)
	})
}

func TestRegistry(t *testing.T) {
	registry := plugin.NewRegistry()
	mock := &MockPlugin{
		name:    "test-plugin",
		version: "1.0.0",
		pType:   plugin.PluginTypeVectorizer,
	}

	t.Run("register plugin", func(t *testing.T) {
		err := registry.Register("test", mock)
		require.NoError(t, err)
	})

	t.Run("get plugin", func(t *testing.T) {
		p, err := registry.Get("test")
		require.NoError(t, err)
		assert.Equal(t, "test-plugin", p.Name())
	})

	t.Run("list plugins", func(t *testing.T) {
		names := registry.List()
		assert.Contains(t, names, "test")
	})

	t.Run("list by type", func(t *testing.T) {
		plugins := registry.ListByType(plugin.PluginTypeVectorizer)
		assert.Len(t, plugins, 1)
		assert.Equal(t, "test-plugin", plugins[0].Name())
	})

	t.Run("duplicate registration", func(t *testing.T) {
		err := registry.Register("test", mock)
		require.Error(t, err)
		assert.IsType(t, plugin.ErrPluginAlreadyExists{}, err)
	})

	t.Run("unregister plugin", func(t *testing.T) {
		err := registry.Unregister("test")
		require.NoError(t, err)

		_, err = registry.Get("test")
		require.Error(t, err)
		assert.IsType(t, plugin.ErrPluginNotFound{}, err)
	})
}

func TestConfig(t *testing.T) {
	config := plugin.Config{
		"stringKey": "value",
		"intKey":    42,
		"boolKey":   true,
	}

	t.Run("get string", func(t *testing.T) {
		val, err := config.GetString("stringKey")
		require.NoError(t, err)
		assert.Equal(t, "value", val)
	})

	t.Run("get int", func(t *testing.T) {
		val, err := config.GetInt("intKey")
		require.NoError(t, err)
		assert.Equal(t, 42, val)
	})

	t.Run("get bool", func(t *testing.T) {
		val, err := config.GetBool("boolKey")
		require.NoError(t, err)
		assert.True(t, val)
	})

	t.Run("get with default", func(t *testing.T) {
		val := config.GetStringWithDefault("missingKey", "default")
		assert.Equal(t, "default", val)
	})

	t.Run("missing key error", func(t *testing.T) {
		_, err := config.GetString("missingKey")
		require.Error(t, err)
		assert.IsType(t, plugin.ErrConfigKeyNotFound{}, err)
	})

	t.Run("type mismatch error", func(t *testing.T) {
		_, err := config.GetInt("stringKey")
		require.Error(t, err)
		assert.IsType(t, plugin.ErrConfigTypeMismatch{}, err)
	})
}

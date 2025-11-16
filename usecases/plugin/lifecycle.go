package plugin

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weaviate/weaviate/entities/plugin"
)

// LifecycleManager manages plugin lifecycle and graceful transitions
type LifecycleManager struct {
	mu      sync.RWMutex
	plugins map[string]*PluginState
}

// PluginState tracks the state of a plugin
type PluginState struct {
	plugin       plugin.Plugin
	state        PluginStateType
	inFlightReqs int64
	mu           sync.RWMutex
}

// PluginStateType represents the lifecycle state of a plugin
type PluginStateType int

const (
	StateLoading PluginStateType = iota
	StateRunning
	StateDraining
	StateStopped
)

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		plugins: make(map[string]*PluginState),
	}
}

// Track starts tracking a plugin
func (lm *LifecycleManager) Track(name string, p plugin.Plugin) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.plugins[name] = &PluginState{
		plugin:       p,
		state:        StateLoading,
		inFlightReqs: 0,
	}
}

// SetState updates the plugin state
func (lm *LifecycleManager) SetState(name string, state PluginStateType) error {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return plugin.ErrPluginNotFound{Name: name}
	}

	ps.mu.Lock()
	ps.state = state
	ps.mu.Unlock()

	return nil
}

// GetState retrieves the plugin state
func (lm *LifecycleManager) GetState(name string) (PluginStateType, error) {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return StateStopped, plugin.ErrPluginNotFound{Name: name}
	}

	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.state, nil
}

// BeginRequest increments the in-flight request counter
func (lm *LifecycleManager) BeginRequest(name string) error {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return plugin.ErrPluginNotFound{Name: name}
	}

	ps.mu.RLock()
	state := ps.state
	ps.mu.RUnlock()

	// Don't allow new requests if draining or stopped
	if state == StateDraining || state == StateStopped {
		return plugin.ErrInvalidManifest{Reason: "plugin is not accepting new requests"}
	}

	atomic.AddInt64(&ps.inFlightReqs, 1)
	return nil
}

// EndRequest decrements the in-flight request counter
func (lm *LifecycleManager) EndRequest(name string) {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return
	}

	atomic.AddInt64(&ps.inFlightReqs, -1)
}

// DrainRequests waits for all in-flight requests to complete
func (lm *LifecycleManager) DrainRequests(p plugin.Plugin) error {
	// Find the plugin state
	lm.mu.RLock()
	var ps *PluginState
	var name string
	for n, state := range lm.plugins {
		if state.plugin == p {
			ps = state
			name = n
			break
		}
	}
	lm.mu.RUnlock()

	if ps == nil {
		return plugin.ErrPluginNotFound{Name: name}
	}

	// Set state to draining
	ps.mu.Lock()
	ps.state = StateDraining
	ps.mu.Unlock()

	// Wait for in-flight requests to complete (with timeout)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return plugin.ErrInvalidManifest{Reason: "timeout waiting for requests to drain"}
		case <-ticker.C:
			if atomic.LoadInt64(&ps.inFlightReqs) == 0 {
				return nil
			}
		}
	}
}

// DrainRequestsByName waits for all in-flight requests to complete for a named plugin
func (lm *LifecycleManager) DrainRequestsByName(name string) error {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return plugin.ErrPluginNotFound{Name: name}
	}

	return lm.DrainRequests(ps.plugin)
}

// Untrack stops tracking a plugin
func (lm *LifecycleManager) Untrack(name string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	delete(lm.plugins, name)
}

// GetInFlightRequests returns the number of in-flight requests for a plugin
func (lm *LifecycleManager) GetInFlightRequests(name string) (int64, error) {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return 0, plugin.ErrPluginNotFound{Name: name}
	}

	return atomic.LoadInt64(&ps.inFlightReqs), nil
}

// Initialize initializes a plugin
func (lm *LifecycleManager) Initialize(ctx context.Context, name string, config plugin.Config) error {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return plugin.ErrPluginNotFound{Name: name}
	}

	// Initialize the plugin
	if err := ps.plugin.Init(ctx, config); err != nil {
		return err
	}

	return nil
}

// Start starts a plugin
func (lm *LifecycleManager) Start(ctx context.Context, name string) error {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return plugin.ErrPluginNotFound{Name: name}
	}

	// Start the plugin
	if err := ps.plugin.Start(ctx); err != nil {
		return err
	}

	// Update state to running
	ps.mu.Lock()
	ps.state = StateRunning
	ps.mu.Unlock()

	return nil
}

// Stop stops a plugin
func (lm *LifecycleManager) Stop(ctx context.Context, name string) error {
	lm.mu.RLock()
	ps, exists := lm.plugins[name]
	lm.mu.RUnlock()

	if !exists {
		return plugin.ErrPluginNotFound{Name: name}
	}

	// Drain requests first
	if err := lm.DrainRequests(ps.plugin); err != nil {
		return err
	}

	// Stop the plugin
	if err := ps.plugin.Stop(ctx); err != nil {
		return err
	}

	// Update state to stopped
	ps.mu.Lock()
	ps.state = StateStopped
	ps.mu.Unlock()

	return nil
}

package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/weaviate/weaviate/entities/plugin"
)

// Sandbox provides isolation and resource limits for plugins
type Sandbox struct {
	name   string
	limits *SandboxLimits
}

// SandboxLimits defines resource constraints for a sandbox
type SandboxLimits struct {
	MaxMemory      uint64        // in bytes
	MaxCPUTime     time.Duration // max CPU time per operation
	MaxFileSize    uint64        // max file size in bytes
	AllowedSyscalls []string      // list of allowed system calls (for native plugins)
}

// NewSandbox creates a new sandbox with the given limits
func NewSandbox(name string, limits *SandboxLimits) *Sandbox {
	return &Sandbox{
		name:   name,
		limits: limits,
	}
}

// ParseResourceLimits converts manifest resource limits to sandbox limits
func ParseResourceLimits(resources plugin.ResourceLimits) (*SandboxLimits, error) {
	limits := &SandboxLimits{
		MaxMemory:   2 * 1024 * 1024 * 1024, // default 2GB
		MaxCPUTime:  30 * time.Second,        // default 30s per operation
		MaxFileSize: 100 * 1024 * 1024,       // default 100MB
	}

	// Parse memory limit (e.g., "2Gi", "512Mi")
	if resources.Memory != "" {
		memory, err := parseMemoryLimit(resources.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid memory limit: %w", err)
		}
		limits.MaxMemory = memory
	}

	// Parse CPU limit (e.g., "1000m", "2")
	if resources.CPU != "" {
		cpuTime, err := parseCPULimit(resources.CPU)
		if err != nil {
			return nil, fmt.Errorf("invalid CPU limit: %w", err)
		}
		limits.MaxCPUTime = cpuTime
	}

	return limits, nil
}

// parseMemoryLimit parses memory strings like "2Gi", "512Mi"
func parseMemoryLimit(limit string) (uint64, error) {
	if len(limit) < 2 {
		return 0, fmt.Errorf("invalid memory format: %s", limit)
	}

	// Simple parser for common formats
	var multiplier uint64
	var numStr string

	if len(limit) >= 2 && limit[len(limit)-1] == 'i' {
		// Binary units (Ki, Mi, Gi)
		switch limit[len(limit)-2] {
		case 'K':
			multiplier = 1024
			numStr = limit[:len(limit)-2]
		case 'M':
			multiplier = 1024 * 1024
			numStr = limit[:len(limit)-2]
		case 'G':
			multiplier = 1024 * 1024 * 1024
			numStr = limit[:len(limit)-2]
		default:
			return 0, fmt.Errorf("unknown memory unit: %s", limit)
		}
	} else {
		// Decimal units (K, M, G)
		switch limit[len(limit)-1] {
		case 'K':
			multiplier = 1000
			numStr = limit[:len(limit)-1]
		case 'M':
			multiplier = 1000 * 1000
			numStr = limit[:len(limit)-1]
		case 'G':
			multiplier = 1000 * 1000 * 1000
			numStr = limit[:len(limit)-1]
		default:
			return 0, fmt.Errorf("unknown memory unit: %s", limit)
		}
	}

	var num uint64
	_, err := fmt.Sscanf(numStr, "%d", &num)
	if err != nil {
		return 0, fmt.Errorf("invalid memory number: %w", err)
	}

	return num * multiplier, nil
}

// parseCPULimit parses CPU strings like "1000m" (millicores) or "2" (cores)
func parseCPULimit(limit string) (time.Duration, error) {
	// For simplicity, treat CPU limit as max operation time
	// "1000m" = 1 core = 30s max operation time
	// "2000m" = 2 cores = 60s max operation time

	if len(limit) > 0 && limit[len(limit)-1] == 'm' {
		// Millicores
		var millicores int
		_, err := fmt.Sscanf(limit[:len(limit)-1], "%d", &millicores)
		if err != nil {
			return 0, fmt.Errorf("invalid CPU format: %w", err)
		}
		// Convert millicores to max operation time (simplistic)
		return time.Duration(millicores/1000) * 30 * time.Second, nil
	}

	// Whole cores
	var cores int
	_, err := fmt.Sscanf(limit, "%d", &cores)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU format: %w", err)
	}

	return time.Duration(cores) * 30 * time.Second, nil
}

// Execute runs a function within the sandbox constraints
func (s *Sandbox) Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error) {
	// Create a context with timeout based on CPU limits
	ctx, cancel := context.WithTimeout(ctx, s.limits.MaxCPUTime)
	defer cancel()

	// Execute the function
	resultChan := make(chan interface{}, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := fn(ctx)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Wait for completion or timeout
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return nil, plugin.ErrCPULimitExceeded
		}
		return nil, ctx.Err()
	}
}

// Name returns the sandbox name
func (s *Sandbox) Name() string {
	return s.name
}

// Limits returns the sandbox limits
func (s *Sandbox) Limits() *SandboxLimits {
	return s.limits
}

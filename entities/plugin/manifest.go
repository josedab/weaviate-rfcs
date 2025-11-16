package plugin

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest represents a plugin manifest (plugin.yaml)
type Manifest struct {
	APIVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   ManifestMetadata `yaml:"metadata"`
	Spec       ManifestSpec     `yaml:"spec"`
}

// ManifestMetadata contains plugin metadata
type ManifestMetadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Author      string `yaml:"author"`
	Description string `yaml:"description"`
}

// ManifestSpec contains plugin specification
type ManifestSpec struct {
	Type         PluginType              `yaml:"type"`
	Runtime      RuntimeType             `yaml:"runtime"`
	Binary       string                  `yaml:"binary"`
	Dependencies []Dependency            `yaml:"dependencies,omitempty"`
	Resources    ResourceLimits          `yaml:"resources,omitempty"`
	Capabilities map[string]interface{}  `yaml:"capabilities,omitempty"`
	Config       map[string]ConfigSchema `yaml:"config,omitempty"`
}

// RuntimeType represents the plugin runtime environment
type RuntimeType string

const (
	RuntimeWASM   RuntimeType = "wasm"
	RuntimeGRPC   RuntimeType = "grpc"
	RuntimeNative RuntimeType = "native"
)

// Dependency represents a plugin dependency
type Dependency struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// ResourceLimits defines resource constraints for a plugin
type ResourceLimits struct {
	Memory string `yaml:"memory,omitempty"` // e.g., "2Gi"
	CPU    string `yaml:"cpu,omitempty"`    // e.g., "1000m"
}

// ConfigSchema defines the schema for a configuration parameter
type ConfigSchema struct {
	Type     string      `yaml:"type"`
	Required bool        `yaml:"required,omitempty"`
	Secret   bool        `yaml:"secret,omitempty"`
	Default  interface{} `yaml:"default,omitempty"`
}

// LoadManifest loads a plugin manifest from a file
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// Validate validates the manifest
func (m *Manifest) Validate() error {
	if m.APIVersion == "" {
		return ErrInvalidManifest{Reason: "apiVersion is required"}
	}

	if m.Kind != "Plugin" {
		return ErrInvalidManifest{Reason: "kind must be 'Plugin'"}
	}

	if m.Metadata.Name == "" {
		return ErrInvalidManifest{Reason: "metadata.name is required"}
	}

	if m.Metadata.Version == "" {
		return ErrInvalidManifest{Reason: "metadata.version is required"}
	}

	if m.Spec.Type == "" {
		return ErrInvalidManifest{Reason: "spec.type is required"}
	}

	if m.Spec.Runtime == "" {
		return ErrInvalidManifest{Reason: "spec.runtime is required"}
	}

	// Validate runtime type
	switch m.Spec.Runtime {
	case RuntimeWASM, RuntimeGRPC, RuntimeNative:
		// Valid runtime
	default:
		return ErrInvalidManifest{Reason: fmt.Sprintf("invalid runtime: %s", m.Spec.Runtime)}
	}

	// Validate plugin type
	switch m.Spec.Type {
	case PluginTypeVectorizer, PluginTypeTransformer, PluginTypeReranker,
		PluginTypeGenerator, PluginTypeStorage, PluginTypeAuth:
		// Valid plugin type
	default:
		return ErrInvalidManifest{Reason: fmt.Sprintf("invalid plugin type: %s", m.Spec.Type)}
	}

	if m.Spec.Binary == "" {
		return ErrInvalidManifest{Reason: "spec.binary is required"}
	}

	// Validate required config fields
	for key, schema := range m.Spec.Config {
		if schema.Required && schema.Default == nil {
			// This is fine - required fields without defaults must be provided at runtime
			_ = key
		}
	}

	return nil
}

// ParseManifestYAML parses manifest from YAML bytes
func ParseManifestYAML(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return &manifest, nil
}

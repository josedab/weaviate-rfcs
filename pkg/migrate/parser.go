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

package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parser handles parsing of migration files
type Parser struct {
	migrationsDir string
}

// NewParser creates a new parser instance
func NewParser(migrationsDir string) *Parser {
	return &Parser{
		migrationsDir: migrationsDir,
	}
}

// ParseMigration parses a single migration file
func (p *Parser) ParseMigration(filePath string) (*Migration, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file %s: %w", filePath, err)
	}

	var migration Migration
	if err := yaml.Unmarshal(data, &migration); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", filePath, err)
	}

	if err := p.validateMigration(&migration); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", filePath, err)
	}

	return &migration, nil
}

// ParseAllMigrations parses all migration files in the migrations directory
func (p *Parser) ParseAllMigrations() ([]Migration, error) {
	files, err := os.ReadDir(p.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []Migration
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}

		filePath := filepath.Join(p.migrationsDir, file.Name())
		migration, err := p.ParseMigration(filePath)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, *migration)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// validateMigration validates a migration's structure
func (p *Parser) validateMigration(m *Migration) error {
	if m.Version <= 0 {
		return fmt.Errorf("version must be positive, got %d", m.Version)
	}

	if m.Description == "" {
		return fmt.Errorf("description is required")
	}

	if len(m.Operations) == 0 {
		return fmt.Errorf("at least one operation is required")
	}

	for i, op := range m.Operations {
		if err := p.validateOperation(&op, i); err != nil {
			return err
		}
	}

	return nil
}

// validateOperation validates a single operation
func (p *Parser) validateOperation(op *Operation, index int) error {
	if op.Type == "" {
		return fmt.Errorf("operation %d: type is required", index)
	}

	validOperations := map[string]bool{
		OperationAddProperty:              true,
		OperationUpdateVectorIndexConfig:  true,
		OperationReindexProperty:          true,
		OperationAddClass:                 true,
		OperationUpdateClass:              true,
		OperationDeleteProperty:           true,
		OperationRestoreVectorIndexConfig: true,
		OperationEnableCompression:        true,
		OperationDisableCompression:       true,
		OperationRestoreFromBackup:        true,
	}

	if !validOperations[op.Type] {
		return fmt.Errorf("operation %d: unknown operation type '%s'", index, op.Type)
	}

	// Validate class is specified for most operations
	if op.Class == "" && len(op.Classes) == 0 {
		return fmt.Errorf("operation %d: class or classes is required for operation type '%s'", index, op.Type)
	}

	return nil
}

// ParseConfig parses the weaviate-migrate.yaml configuration file
func ParseConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Set defaults
	if config.MigrationsDir == "" {
		config.MigrationsDir = "./migrations"
	}
	if config.MaxConcurrentOperations == 0 {
		config.MaxConcurrentOperations = 1
	}
	if config.OperationTimeout == "" {
		config.OperationTimeout = "30m"
	}
	if config.MigrationTimeout == "" {
		config.MigrationTimeout = "2h"
	}

	return &config, nil
}

// GetNextMigrationVersion determines the next migration version number
func (p *Parser) GetNextMigrationVersion() (int, error) {
	migrations, err := p.ParseAllMigrations()
	if err != nil {
		return 0, err
	}

	if len(migrations) == 0 {
		return 1, nil
	}

	// Find the highest version
	maxVersion := 0
	for _, m := range migrations {
		if m.Version > maxVersion {
			maxVersion = m.Version
		}
	}

	return maxVersion + 1, nil
}

// GenerateMigrationFilename generates a migration filename
func GenerateMigrationFilename(version int, name string) string {
	// Sanitize name: replace spaces with hyphens, lowercase
	sanitized := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	sanitized = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, sanitized)

	return fmt.Sprintf("%03d_%s.yaml", version, sanitized)
}

// ExtractVersionFromFilename extracts version number from filename
func ExtractVersionFromFilename(filename string) (int, error) {
	// Expected format: 001_migration-name.yaml
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid filename format")
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse version number: %w", err)
	}

	return version, nil
}

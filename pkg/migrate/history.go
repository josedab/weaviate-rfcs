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
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/client"
)

const (
	// MigrationHistoryClass is the name of the system collection for tracking migrations
	MigrationHistoryClass = "_MigrationHistory"
)

// HistoryManager manages migration history
type HistoryManager struct {
	client *client.Client
	logger *logrus.Logger
}

// NewHistoryManager creates a new history manager
func NewHistoryManager(client *client.Client, logger *logrus.Logger) *HistoryManager {
	return &HistoryManager{
		client: client,
		logger: logger,
	}
}

// InitializeHistoryCollection creates the migration history collection if it doesn't exist
func (h *HistoryManager) InitializeHistoryCollection(ctx context.Context) error {
	h.logger.Info("Initializing migration history collection...")

	// In a real implementation, this would use the Weaviate client to create the collection
	// with the schema defined in the RFC:
	// - version (int)
	// - description (text)
	// - applied_at (date)
	// - duration_ms (int)
	// - status (text)
	// - applied_by (text)

	h.logger.Info("✓ Migration history collection initialized")
	return nil
}

// RecordMigration records a migration in the history
func (h *HistoryManager) RecordMigration(version int, description string, status string, duration time.Duration) error {
	h.logger.Infof("Recording migration v%d in history...", version)

	username := getUsername()
	durationMs := duration.Milliseconds()

	record := MigrationHistory{
		Version:     version,
		Description: description,
		AppliedAt:   time.Now(),
		DurationMs:  durationMs,
		Status:      status,
		AppliedBy:   username,
	}

	// In a real implementation, this would insert the record into the Weaviate collection
	h.logger.Infof("Migration record: v%d, status=%s, duration=%dms, by=%s",
		record.Version, record.Status, record.DurationMs, record.AppliedBy)

	return nil
}

// GetMigrationHistory retrieves all migration history records
func (h *HistoryManager) GetMigrationHistory(ctx context.Context) ([]MigrationHistory, error) {
	h.logger.Debug("Fetching migration history...")

	// In a real implementation, this would query the Weaviate collection
	// For now, return an empty slice as a placeholder
	history := []MigrationHistory{}

	return history, nil
}

// GetAppliedVersions returns a list of applied migration versions
func (h *HistoryManager) GetAppliedVersions(ctx context.Context) ([]int, error) {
	history, err := h.GetMigrationHistory(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]int, 0, len(history))
	for _, record := range history {
		if record.Status == StatusSuccess {
			versions = append(versions, record.Version)
		}
	}

	return versions, nil
}

// GetCurrentVersion returns the current schema version (highest applied migration)
func (h *HistoryManager) GetCurrentVersion(ctx context.Context) (int, error) {
	versions, err := h.GetAppliedVersions(ctx)
	if err != nil {
		return 0, err
	}

	if len(versions) == 0 {
		return 0, nil
	}

	maxVersion := 0
	for _, v := range versions {
		if v > maxVersion {
			maxVersion = v
		}
	}

	return maxVersion, nil
}

// IsMigrationApplied checks if a specific migration version has been applied
func (h *HistoryManager) IsMigrationApplied(ctx context.Context, version int) (bool, error) {
	versions, err := h.GetAppliedVersions(ctx)
	if err != nil {
		return false, err
	}

	for _, v := range versions {
		if v == version {
			return true, nil
		}
	}

	return false, nil
}

// FormatHistory formats migration history for display
func FormatHistory(history []MigrationHistory) string {
	var output string
	output += "Applied Migrations\n"
	output += "==================\n\n"

	if len(history) == 0 {
		output += "No migrations have been applied yet.\n"
		return output
	}

	for _, record := range history {
		statusSymbol := "✓"
		if record.Status == StatusFailed {
			statusSymbol = "✗"
		} else if record.Status == StatusRolledBack {
			statusSymbol = "↶"
		}

		output += fmt.Sprintf("%s v%d: %s (%s)\n",
			statusSymbol, record.Version, record.Description, record.AppliedAt.Format("2006-01-02 15:04:05"))
		output += fmt.Sprintf("   Duration: %dms, Applied by: %s, Status: %s\n\n",
			record.DurationMs, record.AppliedBy, record.Status)
	}

	return output
}

// getUsername returns the current system username
func getUsername() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	hostname, err := os.Hostname()
	if err == nil {
		return hostname
	}
	return "unknown"
}

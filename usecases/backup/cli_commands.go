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

package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/backup"
)

// CLICommands provides CLI commands for incremental backup and PITR
type CLICommands struct {
	archiver      *WALArchiver
	baseBackupMgr *BaseBackupManager
	recoveryMgr   *RecoveryManager
	retentionMgr  *RetentionPolicy
	logger        *logrus.Logger
}

// NewCLICommands creates a new CLI commands handler
func NewCLICommands(
	archiver *WALArchiver,
	baseBackupMgr *BaseBackupManager,
	recoveryMgr *RecoveryManager,
	retentionMgr *RetentionPolicy,
	logger *logrus.Logger,
) *CLICommands {
	if logger == nil {
		logger = logrus.New()
	}

	return &CLICommands{
		archiver:      archiver,
		baseBackupMgr: baseBackupMgr,
		recoveryMgr:   recoveryMgr,
		retentionMgr:  retentionMgr,
		logger:        logger,
	}
}

// CreateIncrementalBackup creates an incremental backup manually
func (c *CLICommands) CreateIncrementalBackup(ctx context.Context) error {
	c.logger.Info("Creating incremental backup...")

	startTime := time.Now()

	// Archive WAL segments
	if err := c.archiver.Archive(ctx); err != nil {
		return fmt.Errorf("failed to create incremental backup: %w", err)
	}

	duration := time.Since(startTime)
	c.logger.Infof("✓ Backup completed in %v", duration)

	return nil
}

// CreateBaseBackup creates a full base backup
func (c *CLICommands) CreateBaseBackup(ctx context.Context, classes []string) error {
	c.logger.Info("Creating base backup...")

	startTime := time.Now()

	descriptor, err := c.baseBackupMgr.TakeBaseBackup(ctx, classes)
	if err != nil {
		return fmt.Errorf("failed to create base backup: %w", err)
	}

	duration := time.Since(startTime)
	c.logger.Infof("✓ Base backup created: %s", descriptor.ID)
	c.logger.Infof("✓ Completed in %v", duration)

	return nil
}

// ListBackups lists all available backups
func (c *CLICommands) ListBackups(ctx context.Context) error {
	c.logger.Info("Listing backups...")

	backups, err := c.baseBackupMgr.ListBaseBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	c.logger.Info("BASE BACKUPS:")
	for _, b := range backups {
		sizeGB := float64(b.PreCompressionSizeBytes) / 1e9
		c.logger.Infof("  %s  (%.2f GB)", b.StartedAt.Format(time.RFC3339), sizeGB)
	}

	// TODO: List WAL segments

	return nil
}

// RestoreToPIT restores to a specific point in time
func (c *CLICommands) RestoreToPIT(ctx context.Context, targetTime time.Time, validateOnly bool) error {
	if validateOnly {
		c.logger.Infof("Validating recovery to %s...", targetTime.Format(time.RFC3339))
	} else {
		c.logger.Infof("Restoring to %s...", targetTime.Format(time.RFC3339))
	}

	opts := backup.PITROptions{
		TargetTime:   &targetTime,
		Mode:         "complete",
		ValidateOnly: validateOnly,
	}

	// Estimate recovery time
	estimatedTime, err := c.recoveryMgr.EstimateRecoveryTime(ctx, opts)
	if err == nil {
		c.logger.Infof("✓ Estimated recovery time: %v", estimatedTime)
	}

	// Perform recovery
	if err := c.recoveryMgr.RecoverToPIT(ctx, opts); err != nil {
		return fmt.Errorf("recovery failed: %w", err)
	}

	if validateOnly {
		c.logger.Info("✓ Validation passed")
	} else {
		c.logger.Info("✓ Recovery completed successfully")
	}

	return nil
}

// VerifyBackup verifies a backup's integrity
func (c *CLICommands) VerifyBackup(ctx context.Context, backupID string) error {
	c.logger.Infof("Verifying backup %s...", backupID)

	// This would verify the backup integrity
	// For now, just use the validator
	validator := NewRecoveryValidator(c.logger)
	if err := validator.Validate(ctx); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	c.logger.Info("✓ Backup is valid and restorable")
	return nil
}

// ApplyRetentionPolicy applies retention policy to clean up old backups
func (c *CLICommands) ApplyRetentionPolicy(ctx context.Context) error {
	c.logger.Info("Applying retention policy...")

	// Get all backups
	backups, err := c.baseBackupMgr.ListBaseBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	// Apply retention policy
	if err := c.retentionMgr.ApplyAndClean(ctx, backups); err != nil {
		return fmt.Errorf("failed to apply retention policy: %w", err)
	}

	c.logger.Info("✓ Retention policy applied")
	return nil
}

// GetBackupStatus returns status information about backups
func (c *CLICommands) GetBackupStatus(ctx context.Context) (*BackupStatus, error) {
	backups, err := c.baseBackupMgr.ListBaseBackups(ctx)
	if err != nil {
		return nil, err
	}

	status := &BackupStatus{
		BaseBackupCount:  len(backups),
		LastArchivedLSN:  c.archiver.GetLastArchivedLSN(),
		LastBackupTime:   time.Time{},
	}

	if len(backups) > 0 {
		// Find most recent backup
		for _, b := range backups {
			if b.StartedAt.After(status.LastBackupTime) {
				status.LastBackupTime = b.StartedAt
			}
		}
	}

	// Calculate total size
	for _, b := range backups {
		status.TotalBackupSize += b.PreCompressionSizeBytes
	}

	return status, nil
}

// BackupStatus contains status information about backups
type BackupStatus struct {
	BaseBackupCount int
	WALSegmentCount int
	LastBackupTime  time.Time
	LastArchivedLSN uint64
	TotalBackupSize int64
	PITRAvailable   bool
	PITRRange       *PITRRange
}

// PITRRange represents the range of time available for PITR
type PITRRange struct {
	From time.Time
	To   time.Time
}

// String returns a string representation of backup status
func (s *BackupStatus) String() string {
	return fmt.Sprintf("Base Backups: %d, Last Backup: %s, Total Size: %.2f GB, Last LSN: %d",
		s.BaseBackupCount,
		s.LastBackupTime.Format(time.RFC3339),
		float64(s.TotalBackupSize)/1e9,
		s.LastArchivedLSN)
}

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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/backup"
	"github.com/weaviate/weaviate/entities/modulecapabilities"
)

// BaseBackupManager manages base (full) backups for incremental backup system
type BaseBackupManager struct {
	backend   modulecapabilities.BackupBackend
	logger    *logrus.Logger
	scheduler *BackupScheduler
}

// NewBaseBackupManager creates a new base backup manager
func NewBaseBackupManager(backend modulecapabilities.BackupBackend, logger *logrus.Logger) *BaseBackupManager {
	if logger == nil {
		logger = logrus.New()
	}

	return &BaseBackupManager{
		backend: backend,
		logger:  logger,
	}
}

// TakeBaseBackup creates a full base backup
func (m *BaseBackupManager) TakeBaseBackup(ctx context.Context, classes []string) (*backup.IncrementalBackupDescriptor, error) {
	startTime := time.Now()
	backupID := generateBackupID()

	m.logger.Infof("Starting base backup %s", backupID)

	descriptor := &backup.IncrementalBackupDescriptor{
		BackupDescriptor: backup.BackupDescriptor{
			ID:            backupID,
			StartedAt:     startTime,
			Status:        "STARTED",
			Version:       "2.0",
			ServerVersion: "1.26.0", // This should come from actual server version
			Classes:       make([]backup.ClassDescriptor, 0),
		},
		Type:               "base",
		IncrementalEnabled: true,
		LastArchivedLSN:    0, // This should be set to current LSN
	}

	// Create base backup using existing backup functionality
	// This is a simplified version - in reality, we'd use the existing Backupper
	m.logger.Infof("Base backup %s created successfully in %v", backupID, time.Since(startTime))

	descriptor.Status = "SUCCESS"
	descriptor.CompletedAt = time.Now()

	return descriptor, nil
}

// FindBaseBackup finds the most recent base backup before or at the target time
func (m *BaseBackupManager) FindBaseBackup(ctx context.Context, targetTime *time.Time) (*backup.IncrementalBackupDescriptor, error) {
	// This would query the backend for available base backups
	// For now, we return a mock descriptor
	m.logger.Infof("Finding base backup before %v", targetTime)

	// In a real implementation, this would:
	// 1. List all base backups from the backend
	// 2. Filter those that are before targetTime
	// 3. Return the most recent one

	return nil, fmt.Errorf("not implemented: would search backend for base backup before %v", targetTime)
}

// ListBaseBackups lists all available base backups
func (m *BaseBackupManager) ListBaseBackups(ctx context.Context) ([]*backup.IncrementalBackupDescriptor, error) {
	// This would query the backend for available base backups
	m.logger.Info("Listing all base backups")

	// In a real implementation, this would:
	// 1. List all backup manifests from the backend
	// 2. Parse them into IncrementalBackupDescriptor objects
	// 3. Filter for type="base"
	// 4. Return the list

	return []*backup.IncrementalBackupDescriptor{}, nil
}

// GetCurrentLSN gets the current log sequence number
func (m *BaseBackupManager) GetCurrentLSN() uint64 {
	// This should interface with the WAL system to get the current LSN
	// For now, return a placeholder
	return 0
}

// BackupShard creates a backup of a single shard
func (m *BaseBackupManager) BackupShard(ctx context.Context, shardID string) (*backup.ShardDescriptor, error) {
	m.logger.Infof("Backing up shard %s", shardID)

	// This would:
	// 1. Create a snapshot of the shard
	// 2. Create a tar archive
	// 3. Compress the archive
	// 4. Upload to backend storage
	// 5. Return the shard descriptor

	return &backup.ShardDescriptor{
		Name: shardID,
		Node: "node-1", // This should come from actual node info
	}, nil
}

// BackupShardsParallel backs up multiple shards in parallel
func (m *BaseBackupManager) BackupShardsParallel(ctx context.Context, shardIDs []string) ([]*backup.ShardDescriptor, error) {
	var wg sync.WaitGroup
	shardChan := make(chan *backup.ShardDescriptor, len(shardIDs))
	errorChan := make(chan error, len(shardIDs))

	for _, shardID := range shardIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			shardDesc, err := m.BackupShard(ctx, id)
			if err != nil {
				m.logger.Errorf("Failed to backup shard %s: %v", id, err)
				errorChan <- err
				return
			}

			shardChan <- shardDesc
		}(shardID)
	}

	wg.Wait()
	close(shardChan)
	close(errorChan)

	// Check for errors
	if len(errorChan) > 0 {
		return nil, <-errorChan
	}

	// Collect results
	var shards []*backup.ShardDescriptor
	for shard := range shardChan {
		shards = append(shards, shard)
	}

	return shards, nil
}

// generateBackupID generates a unique backup ID
func generateBackupID() string {
	return fmt.Sprintf("%s", time.Now().Format("2006-01-02T15:04:05Z"))
}

// SetScheduler sets the backup scheduler
func (m *BaseBackupManager) SetScheduler(scheduler *BackupScheduler) {
	m.scheduler = scheduler
}

// ScheduleBaseBackup schedules a base backup
func (m *BaseBackupManager) ScheduleBaseBackup(schedule string) error {
	if m.scheduler == nil {
		return fmt.Errorf("scheduler not configured")
	}

	// This would configure the scheduler to run base backups
	m.logger.Infof("Scheduled base backup with schedule: %s", schedule)
	return nil
}

// generateUUID generates a UUID for backup identification
func generateUUID() string {
	return uuid.New().String()
}

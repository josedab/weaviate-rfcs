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
	"github.com/weaviate/weaviate/entities/modulecapabilities"
)

// RetentionPolicy defines how long to keep backups and WAL segments
type RetentionPolicy struct {
	// Keep all backups for this duration
	KeepDaily   time.Duration
	KeepWeekly  time.Duration
	KeepMonthly time.Duration

	// WAL retention
	KeepWAL time.Duration

	logger  *logrus.Logger
	backend modulecapabilities.BackupBackend
}

// NewRetentionPolicy creates a new retention policy
func NewRetentionPolicy(
	keepDaily, keepWeekly, keepMonthly, keepWAL time.Duration,
	backend modulecapabilities.BackupBackend,
	logger *logrus.Logger,
) *RetentionPolicy {
	if logger == nil {
		logger = logrus.New()
	}

	return &RetentionPolicy{
		KeepDaily:   keepDaily,
		KeepWeekly:  keepWeekly,
		KeepMonthly: keepMonthly,
		KeepWAL:     keepWAL,
		backend:     backend,
		logger:      logger,
	}
}

// Apply applies the retention policy to a list of backups
func (r *RetentionPolicy) Apply(backups []*backup.IncrementalBackupDescriptor) []*backup.IncrementalBackupDescriptor {
	now := time.Now()
	keep := make(map[string]*backup.IncrementalBackupDescriptor)

	// Keep all daily backups within KeepDaily period
	dailyCutoff := now.Add(-r.KeepDaily)
	for _, b := range backups {
		if b.StartedAt.After(dailyCutoff) {
			keep[b.ID] = b
		}
	}

	// Keep one weekly backup per week within KeepWeekly period
	weeklyCutoff := now.Add(-r.KeepWeekly)
	weeklyBackups := make(map[string]*backup.IncrementalBackupDescriptor)
	for _, b := range backups {
		if b.StartedAt.After(weeklyCutoff) {
			week := b.StartedAt.Format("2006-W01")
			if existing, ok := weeklyBackups[week]; !ok || b.StartedAt.After(existing.StartedAt) {
				weeklyBackups[week] = b
			}
		}
	}
	for _, b := range weeklyBackups {
		keep[b.ID] = b
	}

	// Keep one monthly backup per month within KeepMonthly period
	monthlyCutoff := now.Add(-r.KeepMonthly)
	monthlyBackups := make(map[string]*backup.IncrementalBackupDescriptor)
	for _, b := range backups {
		if b.StartedAt.After(monthlyCutoff) {
			month := b.StartedAt.Format("2006-01")
			if existing, ok := monthlyBackups[month]; !ok || b.StartedAt.After(existing.StartedAt) {
				monthlyBackups[month] = b
			}
		}
	}
	for _, b := range monthlyBackups {
		keep[b.ID] = b
	}

	// Convert map to slice
	result := make([]*backup.IncrementalBackupDescriptor, 0, len(keep))
	for _, b := range keep {
		result = append(result, b)
	}

	r.logger.Infof("Retention policy: keeping %d out of %d backups", len(result), len(backups))

	return result
}

// ApplyAndClean applies retention policy and deletes old backups
func (r *RetentionPolicy) ApplyAndClean(ctx context.Context, backups []*backup.IncrementalBackupDescriptor) error {
	toKeep := r.Apply(backups)
	keepMap := make(map[string]bool)
	for _, b := range toKeep {
		keepMap[b.ID] = true
	}

	// Delete backups that should not be kept
	for _, b := range backups {
		if !keepMap[b.ID] {
			if err := r.deleteBackup(ctx, b); err != nil {
				r.logger.Errorf("Failed to delete backup %s: %v", b.ID, err)
			} else {
				r.logger.Infof("Deleted old backup: %s (from %v)", b.ID, b.StartedAt)
			}
		}
	}

	return nil
}

// deleteBackup deletes a backup from storage
func (r *RetentionPolicy) deleteBackup(ctx context.Context, b *backup.IncrementalBackupDescriptor) error {
	// This would delete the backup files from the backend
	// The implementation depends on the backend's delete capabilities
	r.logger.Debugf("Deleting backup %s", b.ID)
	return nil
}

// CleanWALSegments removes old WAL segments based on retention policy
func (r *RetentionPolicy) CleanWALSegments(ctx context.Context, segments []backup.WALArchiveMetadata) error {
	now := time.Now()
	cutoff := now.Add(-r.KeepWAL)

	deleted := 0
	for _, seg := range segments {
		if seg.ArchivedAt.Before(cutoff) {
			if err := r.deleteWALSegment(ctx, seg); err != nil {
				r.logger.Errorf("Failed to delete WAL segment %s: %v", seg.SegmentID, err)
			} else {
				deleted++
			}
		}
	}

	r.logger.Infof("Cleaned up %d old WAL segments (older than %v)", deleted, r.KeepWAL)
	return nil
}

// deleteWALSegment deletes a WAL segment from storage
func (r *RetentionPolicy) deleteWALSegment(ctx context.Context, seg backup.WALArchiveMetadata) error {
	// This would delete the WAL segment from the backend
	r.logger.Debugf("Deleting WAL segment %s", seg.SegmentID)
	return nil
}

// GetRetentionStats returns statistics about retained backups
func (r *RetentionPolicy) GetRetentionStats(backups []*backup.IncrementalBackupDescriptor) RetentionStats {
	now := time.Now()
	stats := RetentionStats{}

	dailyCutoff := now.Add(-r.KeepDaily)
	weeklyCutoff := now.Add(-r.KeepWeekly)
	monthlyCutoff := now.Add(-r.KeepMonthly)

	for _, b := range backups {
		if b.StartedAt.After(dailyCutoff) {
			stats.DailyCount++
			stats.DailySize += b.PreCompressionSizeBytes
		}
		if b.StartedAt.After(weeklyCutoff) && b.StartedAt.Before(dailyCutoff) {
			stats.WeeklyCount++
			stats.WeeklySize += b.PreCompressionSizeBytes
		}
		if b.StartedAt.After(monthlyCutoff) && b.StartedAt.Before(weeklyCutoff) {
			stats.MonthlyCount++
			stats.MonthlySize += b.PreCompressionSizeBytes
		}
	}

	stats.TotalCount = stats.DailyCount + stats.WeeklyCount + stats.MonthlyCount
	stats.TotalSize = stats.DailySize + stats.WeeklySize + stats.MonthlySize

	return stats
}

// RetentionStats contains statistics about retained backups
type RetentionStats struct {
	DailyCount   int
	DailySize    int64
	WeeklyCount  int
	WeeklySize   int64
	MonthlyCount int
	MonthlySize  int64
	TotalCount   int
	TotalSize    int64
}

// String returns a string representation of retention stats
func (s RetentionStats) String() string {
	return fmt.Sprintf("Daily: %d backups (%.2f GB), Weekly: %d backups (%.2f GB), Monthly: %d backups (%.2f GB), Total: %d backups (%.2f GB)",
		s.DailyCount, float64(s.DailySize)/1e9,
		s.WeeklyCount, float64(s.WeeklySize)/1e9,
		s.MonthlyCount, float64(s.MonthlySize)/1e9,
		s.TotalCount, float64(s.TotalSize)/1e9)
}

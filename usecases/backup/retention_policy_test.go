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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/weaviate/weaviate/entities/backup"
)

func TestRetentionPolicy_Apply(t *testing.T) {
	now := time.Now()

	policy := NewRetentionPolicy(
		7*24*time.Hour,  // Keep daily for 7 days
		30*24*time.Hour, // Keep weekly for 30 days
		365*24*time.Hour, // Keep monthly for 365 days
		7*24*time.Hour,  // Keep WAL for 7 days
		nil,
		logrus.New(),
	)

	backups := []*backup.IncrementalBackupDescriptor{
		// Daily backups (within 7 days)
		{BackupDescriptor: backup.BackupDescriptor{ID: "1", StartedAt: now.Add(-1 * 24 * time.Hour)}},
		{BackupDescriptor: backup.BackupDescriptor{ID: "2", StartedAt: now.Add(-2 * 24 * time.Hour)}},
		{BackupDescriptor: backup.BackupDescriptor{ID: "3", StartedAt: now.Add(-3 * 24 * time.Hour)}},

		// Weekly backups (older than 7 days, within 30 days)
		{BackupDescriptor: backup.BackupDescriptor{ID: "4", StartedAt: now.Add(-10 * 24 * time.Hour)}},
		{BackupDescriptor: backup.BackupDescriptor{ID: "5", StartedAt: now.Add(-15 * 24 * time.Hour)}},

		// Old backups (should be removed)
		{BackupDescriptor: backup.BackupDescriptor{ID: "6", StartedAt: now.Add(-400 * 24 * time.Hour)}},
	}

	kept := policy.Apply(backups)

	// Should keep daily backups (3) + weekly backups (2)
	assert.GreaterOrEqual(t, len(kept), 3)
	assert.Less(t, len(kept), len(backups))

	// Check that very old backup is not kept
	hasOldBackup := false
	for _, b := range kept {
		if b.ID == "6" {
			hasOldBackup = true
		}
	}
	assert.False(t, hasOldBackup, "Very old backup should not be kept")
}

func TestRetentionPolicy_GetRetentionStats(t *testing.T) {
	now := time.Now()

	policy := NewRetentionPolicy(
		7*24*time.Hour,
		30*24*time.Hour,
		365*24*time.Hour,
		7*24*time.Hour,
		nil,
		logrus.New(),
	)

	backups := []*backup.IncrementalBackupDescriptor{
		{
			BackupDescriptor: backup.BackupDescriptor{
				ID:                      "1",
				StartedAt:               now.Add(-1 * 24 * time.Hour),
				PreCompressionSizeBytes: 1000000000, // 1 GB
			},
		},
		{
			BackupDescriptor: backup.BackupDescriptor{
				ID:                      "2",
				StartedAt:               now.Add(-2 * 24 * time.Hour),
				PreCompressionSizeBytes: 2000000000, // 2 GB
			},
		},
	}

	stats := policy.GetRetentionStats(backups)

	assert.Equal(t, 2, stats.DailyCount)
	assert.Equal(t, int64(3000000000), stats.DailySize)
	assert.Equal(t, 2, stats.TotalCount)
	assert.NotEmpty(t, stats.String())
}

func TestRetentionStats_String(t *testing.T) {
	stats := RetentionStats{
		DailyCount:   3,
		DailySize:    3000000000, // 3 GB
		WeeklyCount:  2,
		WeeklySize:   2000000000, // 2 GB
		MonthlyCount: 1,
		MonthlySize:  1000000000, // 1 GB
		TotalCount:   6,
		TotalSize:    6000000000, // 6 GB
	}

	str := stats.String()
	assert.Contains(t, str, "Daily: 3 backups")
	assert.Contains(t, str, "Weekly: 2 backups")
	assert.Contains(t, str, "Monthly: 1 backups")
	assert.Contains(t, str, "Total: 6 backups")
}

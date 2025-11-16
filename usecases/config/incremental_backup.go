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

package config

import (
	"time"
)

// BackupConfig contains configuration for the backup system
type BackupConfig struct {
	Base        BaseBackupConfig        `json:"base" yaml:"base"`
	Incremental IncrementalBackupConfig `json:"incremental" yaml:"incremental"`
}

// BaseBackupConfig contains configuration for base (full) backups
type BaseBackupConfig struct {
	Enabled     bool             `json:"enabled" yaml:"enabled"`
	Schedule    string           `json:"schedule" yaml:"schedule"` // Cron format: "0 0 * * *" for daily at midnight
	Retention   RetentionConfig  `json:"retention" yaml:"retention"`
	Compression CompressionConfig `json:"compression" yaml:"compression"`
	Encryption  EncryptionConfig `json:"encryption" yaml:"encryption"`
}

// IncrementalBackupConfig contains configuration for incremental backups (WAL archiving)
type IncrementalBackupConfig struct {
	Enabled   bool          `json:"enabled" yaml:"enabled"`
	Interval  time.Duration `json:"interval" yaml:"interval"` // Archive WAL segments every N seconds
	Retention time.Duration `json:"retention" yaml:"retention"` // How long to keep WAL archives
	Storage   StorageConfig `json:"storage" yaml:"storage"`
}

// RetentionConfig defines backup retention policies
type RetentionConfig struct {
	Daily   time.Duration `json:"daily" yaml:"daily"`   // e.g., 7 days
	Weekly  time.Duration `json:"weekly" yaml:"weekly"` // e.g., 30 days
	Monthly time.Duration `json:"monthly" yaml:"monthly"` // e.g., 365 days
}

// CompressionConfig defines compression settings
type CompressionConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Algorithm string `json:"algorithm" yaml:"algorithm"` // "gzip" or "zstd"
	Level     int    `json:"level" yaml:"level"`         // Compression level (1-9 for gzip, 1-22 for zstd)
}

// EncryptionConfig defines encryption settings
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Algorithm string `json:"algorithm" yaml:"algorithm"` // e.g., "AES-256-GCM"
	KeySource string `json:"keySource" yaml:"keySource"` // "env", "vault", "kms"
}

// StorageConfig defines storage backend configuration
type StorageConfig struct {
	Backend     string              `json:"backend" yaml:"backend"` // "s3", "gcs", "azure", "filesystem"
	Bucket      string              `json:"bucket" yaml:"bucket"`
	Prefix      string              `json:"prefix" yaml:"prefix"`
	Region      string              `json:"region" yaml:"region"`
	Replication ReplicationConfig   `json:"replication" yaml:"replication"`
}

// ReplicationConfig defines cross-region replication
type ReplicationConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Regions []string `json:"regions" yaml:"regions"` // e.g., ["us-west-2", "eu-central-1"]
}

// DefaultBackupConfig returns default backup configuration
func DefaultBackupConfig() BackupConfig {
	return BackupConfig{
		Base: BaseBackupConfig{
			Enabled:  true,
			Schedule: "0 0 * * *", // Daily at midnight
			Retention: RetentionConfig{
				Daily:   7 * 24 * time.Hour,
				Weekly:  30 * 24 * time.Hour,
				Monthly: 365 * 24 * time.Hour,
			},
			Compression: CompressionConfig{
				Enabled:   true,
				Algorithm: "zstd",
				Level:     3,
			},
			Encryption: EncryptionConfig{
				Enabled:   false,
				Algorithm: "AES-256-GCM",
				KeySource: "env",
			},
		},
		Incremental: IncrementalBackupConfig{
			Enabled:   false, // Disabled by default for backward compatibility
			Interval:  60 * time.Second,
			Retention: 7 * 24 * time.Hour,
			Storage: StorageConfig{
				Backend: "s3",
				Bucket:  "weaviate-backups",
				Prefix:  "cluster/",
				Region:  "us-east-1",
				Replication: ReplicationConfig{
					Enabled: false,
					Regions: []string{},
				},
			},
		},
	}
}

// Validate validates the backup configuration
func (c BackupConfig) Validate() error {
	// Validate compression algorithm
	if c.Base.Compression.Enabled {
		if c.Base.Compression.Algorithm != "gzip" && c.Base.Compression.Algorithm != "zstd" {
			return ErrInvalidCompressionAlgorithm
		}
	}

	// Validate encryption key source
	if c.Base.Encryption.Enabled {
		if c.Base.Encryption.KeySource != "env" &&
		   c.Base.Encryption.KeySource != "vault" &&
		   c.Base.Encryption.KeySource != "kms" {
			return ErrInvalidEncryptionKeySource
		}
	}

	// Validate storage backend
	validBackends := map[string]bool{
		"s3":         true,
		"gcs":        true,
		"azure":      true,
		"filesystem": true,
	}
	if !validBackends[c.Incremental.Storage.Backend] {
		return ErrInvalidStorageBackend
	}

	return nil
}

var (
	ErrInvalidCompressionAlgorithm = &ConfigError{msg: "invalid compression algorithm, must be 'gzip' or 'zstd'"}
	ErrInvalidEncryptionKeySource  = &ConfigError{msg: "invalid encryption key source, must be 'env', 'vault', or 'kms'"}
	ErrInvalidStorageBackend       = &ConfigError{msg: "invalid storage backend, must be 's3', 'gcs', 'azure', or 'filesystem'"}
)

// ConfigError represents a configuration error
type ConfigError struct {
	msg string
}

func (e *ConfigError) Error() string {
	return e.msg
}

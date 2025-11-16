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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBackupConfig(t *testing.T) {
	config := DefaultBackupConfig()

	assert.True(t, config.Base.Enabled)
	assert.Equal(t, "0 0 * * *", config.Base.Schedule)
	assert.Equal(t, 7*24*time.Hour, config.Base.Retention.Daily)
	assert.Equal(t, 30*24*time.Hour, config.Base.Retention.Weekly)
	assert.Equal(t, 365*24*time.Hour, config.Base.Retention.Monthly)

	assert.True(t, config.Base.Compression.Enabled)
	assert.Equal(t, "zstd", config.Base.Compression.Algorithm)
	assert.Equal(t, 3, config.Base.Compression.Level)

	assert.False(t, config.Base.Encryption.Enabled)
	assert.Equal(t, "AES-256-GCM", config.Base.Encryption.Algorithm)
	assert.Equal(t, "env", config.Base.Encryption.KeySource)

	assert.False(t, config.Incremental.Enabled)
	assert.Equal(t, 60*time.Second, config.Incremental.Interval)
	assert.Equal(t, 7*24*time.Hour, config.Incremental.Retention)
	assert.Equal(t, "s3", config.Incremental.Storage.Backend)
}

func TestBackupConfig_Validate_ValidConfig(t *testing.T) {
	config := DefaultBackupConfig()
	err := config.Validate()
	require.NoError(t, err)
}

func TestBackupConfig_Validate_InvalidCompression(t *testing.T) {
	config := DefaultBackupConfig()
	config.Base.Compression.Algorithm = "invalid"

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidCompressionAlgorithm, err)
}

func TestBackupConfig_Validate_InvalidEncryptionKeySource(t *testing.T) {
	config := DefaultBackupConfig()
	config.Base.Encryption.Enabled = true
	config.Base.Encryption.KeySource = "invalid"

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidEncryptionKeySource, err)
}

func TestBackupConfig_Validate_InvalidStorageBackend(t *testing.T) {
	config := DefaultBackupConfig()
	config.Incremental.Storage.Backend = "invalid"

	err := config.Validate()
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidStorageBackend, err)
}

func TestBackupConfig_Validate_ValidStorageBackends(t *testing.T) {
	config := DefaultBackupConfig()

	backends := []string{"s3", "gcs", "azure", "filesystem"}
	for _, backend := range backends {
		config.Incremental.Storage.Backend = backend
		err := config.Validate()
		assert.NoError(t, err, "Backend %s should be valid", backend)
	}
}

func TestBackupConfig_Validate_ValidCompressionAlgorithms(t *testing.T) {
	config := DefaultBackupConfig()

	algorithms := []string{"gzip", "zstd"}
	for _, algo := range algorithms {
		config.Base.Compression.Algorithm = algo
		err := config.Validate()
		assert.NoError(t, err, "Algorithm %s should be valid", algo)
	}
}

func TestBackupConfig_Validate_ValidEncryptionKeySources(t *testing.T) {
	config := DefaultBackupConfig()
	config.Base.Encryption.Enabled = true

	sources := []string{"env", "vault", "kms"}
	for _, source := range sources {
		config.Base.Encryption.KeySource = source
		err := config.Validate()
		assert.NoError(t, err, "Key source %s should be valid", source)
	}
}

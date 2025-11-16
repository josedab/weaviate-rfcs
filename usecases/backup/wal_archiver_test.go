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
	"github.com/stretchr/testify/require"
)

func TestNewWALArchiver(t *testing.T) {
	config := WALArchiverConfig{
		WALPath:          "/tmp/wal",
		ArchiveInterval:  60 * time.Second,
		CompressionLevel: 3,
		CompressionAlgo:  "zstd",
		Logger:           logrus.New(),
	}

	archiver := NewWALArchiver(config)

	assert.NotNil(t, archiver)
	assert.Equal(t, "/tmp/wal", archiver.walPath)
	assert.Equal(t, 60*time.Second, archiver.archiveInterval)
	assert.Equal(t, "zstd", archiver.compressionAlgo)
	assert.Equal(t, 3, archiver.compressionLevel)
}

func TestWALArchiver_Compress_Gzip(t *testing.T) {
	config := WALArchiverConfig{
		WALPath:         "/tmp/wal",
		CompressionAlgo: "gzip",
		Logger:          logrus.New(),
	}

	archiver := NewWALArchiver(config)

	testData := []byte("This is test data for compression")
	compressed, err := archiver.compress(testData)

	require.NoError(t, err)
	assert.NotNil(t, compressed)
	assert.Less(t, len(compressed), len(testData)+50) // Should be compressed or similar size
}

func TestWALArchiver_Compress_Zstd(t *testing.T) {
	config := WALArchiverConfig{
		WALPath:          "/tmp/wal",
		CompressionAlgo:  "zstd",
		CompressionLevel: 3,
		Logger:           logrus.New(),
	}

	archiver := NewWALArchiver(config)

	testData := []byte("This is test data for compression with zstd algorithm")
	compressed, err := archiver.compress(testData)

	require.NoError(t, err)
	assert.NotNil(t, compressed)
}

func TestWALArchiver_Encrypt(t *testing.T) {
	encryptionKey := []byte("test-encryption-key-32-bytes!!!")

	config := WALArchiverConfig{
		WALPath:       "/tmp/wal",
		EncryptionKey: encryptionKey,
		Logger:        logrus.New(),
	}

	archiver := NewWALArchiver(config)

	testData := []byte("This is test data for encryption")
	encrypted, err := archiver.encrypt(testData)

	require.NoError(t, err)
	assert.NotNil(t, encrypted)
	assert.NotEqual(t, testData, encrypted)
	assert.Greater(t, len(encrypted), len(testData)) // Encrypted data includes nonce
}

func TestWALArchiver_GetLastArchivedLSN(t *testing.T) {
	config := WALArchiverConfig{
		WALPath: "/tmp/wal",
		Logger:  logrus.New(),
	}

	archiver := NewWALArchiver(config)

	lsn := archiver.GetLastArchivedLSN()
	assert.Equal(t, uint64(0), lsn)
}

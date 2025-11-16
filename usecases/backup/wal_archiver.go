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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/backup"
	"github.com/weaviate/weaviate/entities/modulecapabilities"
)

// WALArchiver handles continuous archiving of WAL segments
type WALArchiver struct {
	walPath          string
	backend          modulecapabilities.BackupBackend
	lastArchivedLSN  uint64
	archiveInterval  time.Duration
	compressionLevel int
	compressionAlgo  string // "gzip" or "zstd"
	encryptionKey    []byte
	deleteAfterArchive bool
	logger           *logrus.Logger
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// WALArchiverConfig contains configuration for WAL archiver
type WALArchiverConfig struct {
	WALPath            string
	Backend            modulecapabilities.BackupBackend
	ArchiveInterval    time.Duration
	CompressionLevel   int
	CompressionAlgo    string
	EncryptionKey      []byte
	DeleteAfterArchive bool
	Logger             *logrus.Logger
}

// NewWALArchiver creates a new WAL archiver
func NewWALArchiver(config WALArchiverConfig) *WALArchiver {
	ctx, cancel := context.WithCancel(context.Background())

	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	if config.CompressionAlgo == "" {
		config.CompressionAlgo = "zstd"
	}

	if config.ArchiveInterval == 0 {
		config.ArchiveInterval = 60 * time.Second
	}

	return &WALArchiver{
		walPath:            config.WALPath,
		backend:            config.Backend,
		archiveInterval:    config.ArchiveInterval,
		compressionLevel:   config.CompressionLevel,
		compressionAlgo:    config.CompressionAlgo,
		encryptionKey:      config.EncryptionKey,
		deleteAfterArchive: config.DeleteAfterArchive,
		logger:             config.Logger,
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start begins the continuous archiving loop
func (a *WALArchiver) Start() {
	ticker := time.NewTicker(a.archiveInterval)
	defer ticker.Stop()

	a.logger.Infof("WAL archiver started with interval %v", a.archiveInterval)

	for {
		select {
		case <-ticker.C:
			if err := a.Archive(a.ctx); err != nil {
				a.logger.Errorf("WAL archiving failed: %v", err)
			}
		case <-a.ctx.Done():
			a.logger.Info("WAL archiver stopped")
			return
		}
	}
}

// Stop stops the WAL archiver
func (a *WALArchiver) Stop() {
	a.cancel()
}

// Archive archives new WAL segments
func (a *WALArchiver) Archive(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Find new WAL segments
	segments, err := a.findNewSegments()
	if err != nil {
		return fmt.Errorf("failed to find new segments: %w", err)
	}

	if len(segments) == 0 {
		a.logger.Debug("No new WAL segments to archive")
		return nil
	}

	a.logger.Infof("Found %d new WAL segments to archive", len(segments))

	for _, segment := range segments {
		if err := a.archiveSegment(ctx, segment); err != nil {
			return fmt.Errorf("failed to archive segment %s: %w", segment.FilePath, err)
		}

		// Update watermark
		a.lastArchivedLSN = segment.EndLSN

		// Delete local segment if configured
		if a.deleteAfterArchive {
			if err := os.Remove(segment.FilePath); err != nil {
				a.logger.Warnf("Failed to delete archived segment %s: %v", segment.FilePath, err)
			}
		}
	}

	return nil
}

// findNewSegments finds WAL segments that haven't been archived yet
func (a *WALArchiver) findNewSegments() ([]*backup.WALSegment, error) {
	var segments []*backup.WALSegment

	// List all WAL files in the directory
	files, err := filepath.Glob(filepath.Join(a.walPath, "*.wal"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			a.logger.Warnf("Failed to stat file %s: %v", file, err)
			continue
		}

		// Create segment descriptor
		segment := &backup.WALSegment{
			FilePath:  file,
			Size:      info.Size(),
			StartTime: info.ModTime(), // Approximation
			EndTime:   time.Now(),
		}

		// Calculate checksum
		checksum, err := a.calculateChecksum(file)
		if err != nil {
			a.logger.Warnf("Failed to calculate checksum for %s: %v", file, err)
			continue
		}
		segment.Checksum = checksum

		segments = append(segments, segment)
	}

	return segments, nil
}

// archiveSegment compresses, encrypts, and uploads a WAL segment
func (a *WALArchiver) archiveSegment(ctx context.Context, segment *backup.WALSegment) error {
	startTime := time.Now()

	// Read segment data
	data, err := os.ReadFile(segment.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read segment: %w", err)
	}

	originalSize := int64(len(data))

	// Compress
	compressed, err := a.compress(data)
	if err != nil {
		return fmt.Errorf("failed to compress segment: %w", err)
	}

	compressedSize := int64(len(compressed))

	// Encrypt if configured
	if a.encryptionKey != nil {
		encrypted, err := a.encrypt(compressed)
		if err != nil {
			return fmt.Errorf("failed to encrypt segment: %w", err)
		}
		compressed = encrypted
	}

	// Create metadata
	metadata := backup.WALArchiveMetadata{
		SegmentID:       filepath.Base(segment.FilePath),
		OriginalSize:    originalSize,
		CompressedSize:  compressedSize,
		ArchivedAt:      time.Now(),
		CompressionAlgo: a.compressionAlgo,
		Encrypted:       a.encryptionKey != nil,
		StartLSN:        segment.StartLSN,
		EndLSN:          segment.EndLSN,
	}

	// Upload segment
	segmentKey := fmt.Sprintf("wal/%s", metadata.SegmentID)
	reader := io.NopCloser(bytes.NewReader(compressed))
	if _, err := a.backend.Write(ctx, "incremental", segmentKey, "", "", reader); err != nil {
		return fmt.Errorf("failed to upload segment: %w", err)
	}

	// Upload metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataKey := fmt.Sprintf("wal/%s.metadata", metadata.SegmentID)
	metadataReader := io.NopCloser(bytes.NewReader(metadataJSON))
	if _, err := a.backend.Write(ctx, "incremental", metadataKey, "", "", metadataReader); err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	duration := time.Since(startTime)
	compressionRatio := float64(compressedSize) / float64(originalSize) * 100

	a.logger.Infof("Archived WAL segment %s: %d -> %d bytes (%.1f%%) in %v",
		metadata.SegmentID, originalSize, compressedSize, compressionRatio, duration)

	return nil
}

// compress compresses data using the configured algorithm
func (a *WALArchiver) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	switch a.compressionAlgo {
	case "gzip":
		gw, err := gzip.NewWriterLevel(&buf, a.compressionLevel)
		if err != nil {
			return nil, err
		}
		if _, err := gw.Write(data); err != nil {
			gw.Close()
			return nil, err
		}
		if err := gw.Close(); err != nil {
			return nil, err
		}

	case "zstd":
		zw, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(a.compressionLevel)))
		if err != nil {
			return nil, err
		}
		if _, err := zw.Write(data); err != nil {
			zw.Close()
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", a.compressionAlgo)
	}

	return buf.Bytes(), nil
}

// encrypt encrypts data using AES-256-GCM
func (a *WALArchiver) encrypt(data []byte) ([]byte, error) {
	// Derive a 32-byte key from the encryption key
	hash := sha256.Sum256(a.encryptionKey)
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// calculateChecksum calculates CRC32 checksum of a file
func (a *WALArchiver) calculateChecksum(filePath string) (uint32, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}
	return crc32.ChecksumIEEE(data), nil
}

// GetLastArchivedLSN returns the last archived LSN
func (a *WALArchiver) GetLastArchivedLSN() uint64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastArchivedLSN
}

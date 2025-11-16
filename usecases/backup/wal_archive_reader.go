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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/backup"
	"github.com/weaviate/weaviate/entities/modulecapabilities"
)

// WALArchiveReader reads archived WAL segments for recovery
type WALArchiveReader struct {
	backend       modulecapabilities.BackupBackend
	encryptionKey []byte
	logger        *logrus.Logger
}

// NewWALArchiveReader creates a new WAL archive reader
func NewWALArchiveReader(backend modulecapabilities.BackupBackend, encryptionKey []byte, logger *logrus.Logger) *WALArchiveReader {
	if logger == nil {
		logger = logrus.New()
	}

	return &WALArchiveReader{
		backend:       backend,
		encryptionKey: encryptionKey,
		logger:        logger,
	}
}

// ListSegments lists all archived WAL segments
func (r *WALArchiveReader) ListSegments(ctx context.Context) ([]backup.WALArchiveMetadata, error) {
	// This would need to be implemented based on the backend's listing capabilities
	// For now, we'll return an empty list
	// In a real implementation, this would list all .metadata files in the wal/ prefix
	return []backup.WALArchiveMetadata{}, nil
}

// GetSegments returns WAL segments between startLSN and endLSN
func (r *WALArchiveReader) GetSegments(ctx context.Context, startLSN, endLSN uint64) ([]backup.WALArchiveMetadata, error) {
	// List all segments
	allSegments, err := r.ListSegments(ctx)
	if err != nil {
		return nil, err
	}

	// Filter segments that overlap with the requested range
	var segments []backup.WALArchiveMetadata
	for _, seg := range allSegments {
		// Include segment if it overlaps with [startLSN, endLSN]
		if seg.EndLSN >= startLSN && seg.StartLSN <= endLSN {
			segments = append(segments, seg)
		}
	}

	// Sort by StartLSN
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartLSN < segments[j].StartLSN
	})

	return segments, nil
}

// Download downloads and decompresses a WAL segment
func (r *WALArchiveReader) Download(ctx context.Context, metadata backup.WALArchiveMetadata) ([]byte, error) {
	// Download segment
	segmentKey := fmt.Sprintf("wal/%s", metadata.SegmentID)

	var buf bytes.Buffer
	writer := &nopWriteCloser{Writer: &buf}

	if _, err := r.backend.Read(ctx, "incremental", segmentKey, "", "", writer); err != nil {
		return nil, fmt.Errorf("failed to download segment: %w", err)
	}

	data := buf.Bytes()

	// Decrypt if needed
	if metadata.Encrypted {
		decrypted, err := r.decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt segment: %w", err)
		}
		data = decrypted
	}

	// Decompress
	decompressed, err := r.decompress(data, metadata.CompressionAlgo)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress segment: %w", err)
	}

	r.logger.Infof("Downloaded and decompressed WAL segment %s: %d -> %d bytes",
		metadata.SegmentID, metadata.CompressedSize, metadata.OriginalSize)

	return decompressed, nil
}

// decompress decompresses data using the specified algorithm
func (r *WALArchiveReader) decompress(data []byte, algo string) ([]byte, error) {
	reader := bytes.NewReader(data)

	switch algo {
	case "gzip":
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gr.Close()

		return io.ReadAll(gr)

	case "zstd":
		zr, err := zstd.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer zr.Close()

		return io.ReadAll(zr)

	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", algo)
	}
}

// decrypt decrypts data using AES-256-GCM
func (r *WALArchiveReader) decrypt(data []byte) ([]byte, error) {
	if r.encryptionKey == nil {
		return nil, fmt.Errorf("encryption key not provided")
	}

	// Derive a 32-byte key from the encryption key
	hash := sha256.Sum256(r.encryptionKey)
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// nopWriteCloser wraps an io.Writer to add a no-op Close method
type nopWriteCloser struct {
	io.Writer
}

func (nwc *nopWriteCloser) Close() error {
	return nil
}

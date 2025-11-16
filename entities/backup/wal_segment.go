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
	"time"
)

// WALSegment represents a WAL file segment that can be archived
type WALSegment struct {
	StartLSN  uint64    `json:"startLsn"`
	EndLSN    uint64    `json:"endLsn"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	FilePath  string    `json:"filePath"`
	Size      int64     `json:"size"`
	Checksum  uint32    `json:"checksum"`
}

// WALArchiveMetadata contains metadata about archived WAL segments
type WALArchiveMetadata struct {
	SegmentID       string    `json:"segmentId"`
	OriginalSize    int64     `json:"originalSize"`
	CompressedSize  int64     `json:"compressedSize"`
	ArchivedAt      time.Time `json:"archivedAt"`
	CompressionAlgo string    `json:"compressionAlgo"`
	Encrypted       bool      `json:"encrypted"`
	StartLSN        uint64    `json:"startLsn"`
	EndLSN          uint64    `json:"endLsn"`
}

// IncrementalBackupDescriptor extends BackupDescriptor with incremental backup info
type IncrementalBackupDescriptor struct {
	BackupDescriptor
	Type               string               `json:"type"` // "base" or "incremental"
	BaseBackupID       string               `json:"baseBackupId,omitempty"`
	WALSegments        []WALArchiveMetadata `json:"walSegments,omitempty"`
	LastArchivedLSN    uint64               `json:"lastArchivedLsn"`
	IncrementalEnabled bool                 `json:"incrementalEnabled"`
}

// PITROptions contains options for point-in-time recovery
type PITROptions struct {
	TargetTime   *time.Time `json:"targetTime,omitempty"`
	TargetLSN    *uint64    `json:"targetLsn,omitempty"`
	Mode         string     `json:"mode"` // "complete", "partial", "verify"
	ValidateOnly bool       `json:"validateOnly"`
}

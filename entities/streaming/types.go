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

package streaming

import (
	"time"

	"github.com/google/uuid"
)

// ChangeType represents the type of change event
type ChangeType string

const (
	ChangeTypeInsert ChangeType = "INSERT"
	ChangeTypeUpdate ChangeType = "UPDATE"
	ChangeTypeDelete ChangeType = "DELETE"
	ChangeTypeRead   ChangeType = "READ"
)

// ChangeEvent represents a data change event
type ChangeEvent struct {
	Type      ChangeType             `json:"type"`
	Class     string                 `json:"class"`
	ID        uuid.UUID              `json:"id"`
	Before    map[string]interface{} `json:"before,omitempty"`
	After     map[string]interface{} `json:"after,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	UserID    *uuid.UUID             `json:"userId,omitempty"`

	// Metadata
	TransactionID *string `json:"transactionId,omitempty"`
	SequenceNum   uint64  `json:"sequenceNum"`
}

// ClassConfig represents the configuration for transforming stream messages to objects
type ClassConfig struct {
	ClassName       string
	KeyField        string
	VectorizeFields []string
	TransformFunc   func(map[string]interface{}) (map[string]interface{}, error)
}

// EventType represents the type of trigger event
type EventType string

const (
	EventTypeInsert EventType = "INSERT"
	EventTypeUpdate EventType = "UPDATE"
	EventTypeDelete EventType = "DELETE"
)

// Filter represents a filter for events
type Filter struct {
	Class      string
	Operations []ChangeType
	Where      map[string]interface{}
}

// Matches checks if an event matches the filter
func (f *Filter) Matches(event *ChangeEvent) bool {
	// Check class
	if f.Class != "" && f.Class != event.Class {
		return false
	}

	// Check operations
	if len(f.Operations) > 0 {
		match := false
		for _, op := range f.Operations {
			if op == event.Type {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// TODO: Implement where clause matching
	return true
}

// DebeziumEvent represents a Debezium-style CDC event
type DebeziumEvent struct {
	Before map[string]interface{} `json:"before"`
	After  map[string]interface{} `json:"after"`
	Source SourceMetadata         `json:"source"`
	Op     string                 `json:"op"` // c, u, d, r
	TsMs   int64                  `json:"ts_ms"`
}

// SourceMetadata represents metadata about the source of a change
type SourceMetadata struct {
	Version   string `json:"version"`
	Connector string `json:"connector"`
	Name      string `json:"name"`
	TsMs      int64  `json:"ts_ms"`
	Snapshot  string `json:"snapshot,omitempty"`
	DB        string `json:"db"`
	Sequence  string `json:"sequence,omitempty"`
	Table     string `json:"table"`
	ServerID  int64  `json:"server_id"`
	GTId      string `json:"gtid,omitempty"`
	File      string `json:"file"`
	Pos       int64  `json:"pos"`
	Row       int    `json:"row"`
	Thread    int64  `json:"thread,omitempty"`
	Query     string `json:"query,omitempty"`
}

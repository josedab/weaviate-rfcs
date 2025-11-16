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

package evolution

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/weaviate/weaviate/entities/schema/evolution"
	bolt "go.etcd.io/bbolt"
)

var (
	versionsBucket  = []byte("schema_versions")
	hashIndexBucket = []byte("schema_hash_index")
	metadataBucket  = []byte("schema_metadata")
)

// BoltVersionStore implements VersionStore using BoltDB
type BoltVersionStore struct {
	mu sync.RWMutex
	db *bolt.DB
}

// NewBoltVersionStore creates a new BoltDB-based version store
func NewBoltVersionStore(db *bolt.DB) (*BoltVersionStore, error) {
	store := &BoltVersionStore{
		db: db,
	}

	// Initialize buckets
	err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(versionsBucket); err != nil {
			return fmt.Errorf("failed to create versions bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(hashIndexBucket); err != nil {
			return fmt.Errorf("failed to create hash index bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(metadataBucket); err != nil {
			return fmt.Errorf("failed to create metadata bucket: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return store, nil
}

// Save stores a schema version
func (s *BoltVersionStore) Save(version *evolution.SchemaVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(versionsBucket)
		hashBucket := tx.Bucket(hashIndexBucket)

		// Serialize version
		data, err := json.Marshal(version)
		if err != nil {
			return fmt.Errorf("failed to marshal version: %w", err)
		}

		// Store by ID
		key := s.encodeID(version.ID)
		if err := bucket.Put(key, data); err != nil {
			return fmt.Errorf("failed to store version: %w", err)
		}

		// Store hash index
		if err := hashBucket.Put([]byte(version.Hash), key); err != nil {
			return fmt.Errorf("failed to store hash index: %w", err)
		}

		// Update latest version metadata
		if err := s.updateLatestVersion(tx, version.ID); err != nil {
			return fmt.Errorf("failed to update latest version: %w", err)
		}

		return nil
	})
}

// Get retrieves a schema version by ID
func (s *BoltVersionStore) Get(id uint64) (*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var version *evolution.SchemaVersion

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(versionsBucket)
		key := s.encodeID(id)

		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("version %d not found", id)
		}

		version = &evolution.SchemaVersion{}
		if err := json.Unmarshal(data, version); err != nil {
			return fmt.Errorf("failed to unmarshal version: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return version, nil
}

// GetLatest retrieves the latest schema version
func (s *BoltVersionStore) GetLatest() (*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var version *evolution.SchemaVersion

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(metadataBucket)

		// Get latest version ID
		data := bucket.Get([]byte("latest_version"))
		if data == nil {
			return fmt.Errorf("no versions found")
		}

		var latestID uint64
		if err := json.Unmarshal(data, &latestID); err != nil {
			return fmt.Errorf("failed to unmarshal latest version ID: %w", err)
		}

		// Get the version
		versionBucket := tx.Bucket(versionsBucket)
		versionData := versionBucket.Get(s.encodeID(latestID))
		if versionData == nil {
			return fmt.Errorf("latest version %d not found", latestID)
		}

		version = &evolution.SchemaVersion{}
		if err := json.Unmarshal(versionData, version); err != nil {
			return fmt.Errorf("failed to unmarshal version: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return version, nil
}

// List returns a range of schema versions
func (s *BoltVersionStore) List(offset, limit int) ([]*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := make([]*evolution.SchemaVersion, 0)

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(versionsBucket)

		// Collect all versions
		allVersions := make([]*evolution.SchemaVersion, 0)
		cursor := bucket.Cursor()

		for key, data := cursor.First(); key != nil; key, data = cursor.Next() {
			version := &evolution.SchemaVersion{}
			if err := json.Unmarshal(data, version); err != nil {
				continue // Skip malformed versions
			}
			allVersions = append(allVersions, version)
		}

		// Sort by ID descending (newest first)
		sort.Slice(allVersions, func(i, j int) bool {
			return allVersions[i].ID > allVersions[j].ID
		})

		// Apply offset and limit
		start := offset
		if start >= len(allVersions) {
			return nil
		}

		end := start + limit
		if end > len(allVersions) {
			end = len(allVersions)
		}

		versions = allVersions[start:end]

		return nil
	})

	if err != nil {
		return nil, err
	}

	return versions, nil
}

// GetByHash retrieves a version by its hash
func (s *BoltVersionStore) GetByHash(hash string) (*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var version *evolution.SchemaVersion

	err := s.db.View(func(tx *bolt.Tx) error {
		hashBucket := tx.Bucket(hashIndexBucket)
		versionBucket := tx.Bucket(versionsBucket)

		// Lookup version ID by hash
		versionKey := hashBucket.Get([]byte(hash))
		if versionKey == nil {
			return fmt.Errorf("version with hash %s not found", hash)
		}

		// Get version data
		data := versionBucket.Get(versionKey)
		if data == nil {
			return fmt.Errorf("version data not found")
		}

		version = &evolution.SchemaVersion{}
		if err := json.Unmarshal(data, version); err != nil {
			return fmt.Errorf("failed to unmarshal version: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return version, nil
}

// Delete removes a schema version
func (s *BoltVersionStore) Delete(id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(versionsBucket)
		hashBucket := tx.Bucket(hashIndexBucket)

		// Get version to find its hash
		key := s.encodeID(id)
		data := bucket.Get(key)
		if data == nil {
			return fmt.Errorf("version %d not found", id)
		}

		version := &evolution.SchemaVersion{}
		if err := json.Unmarshal(data, version); err != nil {
			return fmt.Errorf("failed to unmarshal version: %w", err)
		}

		// Delete hash index
		if err := hashBucket.Delete([]byte(version.Hash)); err != nil {
			return fmt.Errorf("failed to delete hash index: %w", err)
		}

		// Delete version
		if err := bucket.Delete(key); err != nil {
			return fmt.Errorf("failed to delete version: %w", err)
		}

		return nil
	})
}

// NextID returns the next available version ID
func (s *BoltVersionStore) NextID() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var nextID uint64

	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(metadataBucket)

		// Get current max ID
		data := bucket.Get([]byte("max_version_id"))
		if data != nil {
			if err := json.Unmarshal(data, &nextID); err != nil {
				return fmt.Errorf("failed to unmarshal max version ID: %w", err)
			}
		}

		// Increment and save
		nextID++
		data, err := json.Marshal(nextID)
		if err != nil {
			return fmt.Errorf("failed to marshal max version ID: %w", err)
		}

		if err := bucket.Put([]byte("max_version_id"), data); err != nil {
			return fmt.Errorf("failed to save max version ID: %w", err)
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return nextID, nil
}

// updateLatestVersion updates the latest version metadata
func (s *BoltVersionStore) updateLatestVersion(tx *bolt.Tx, id uint64) error {
	bucket := tx.Bucket(metadataBucket)

	data, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("failed to marshal latest version ID: %w", err)
	}

	return bucket.Put([]byte("latest_version"), data)
}

// encodeID encodes a version ID as a byte array
func (s *BoltVersionStore) encodeID(id uint64) []byte {
	// Use 8 bytes for uint64
	b := make([]byte, 8)
	b[0] = byte(id >> 56)
	b[1] = byte(id >> 48)
	b[2] = byte(id >> 40)
	b[3] = byte(id >> 32)
	b[4] = byte(id >> 24)
	b[5] = byte(id >> 16)
	b[6] = byte(id >> 8)
	b[7] = byte(id)
	return b
}

// InMemoryVersionStore implements VersionStore using in-memory storage
// Useful for testing
type InMemoryVersionStore struct {
	mu       sync.RWMutex
	versions map[uint64]*evolution.SchemaVersion
	hashes   map[string]uint64
	maxID    uint64
}

// NewInMemoryVersionStore creates a new in-memory version store
func NewInMemoryVersionStore() *InMemoryVersionStore {
	return &InMemoryVersionStore{
		versions: make(map[uint64]*evolution.SchemaVersion),
		hashes:   make(map[string]uint64),
		maxID:    0,
	}
}

// Save stores a schema version
func (s *InMemoryVersionStore) Save(version *evolution.SchemaVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.versions[version.ID] = version
	s.hashes[version.Hash] = version.ID

	if version.ID > s.maxID {
		s.maxID = version.ID
	}

	return nil
}

// Get retrieves a schema version by ID
func (s *InMemoryVersionStore) Get(id uint64) (*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	version, exists := s.versions[id]
	if !exists {
		return nil, fmt.Errorf("version %d not found", id)
	}

	return version, nil
}

// GetLatest retrieves the latest schema version
func (s *InMemoryVersionStore) GetLatest() (*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.maxID == 0 {
		return nil, fmt.Errorf("no versions found")
	}

	return s.versions[s.maxID], nil
}

// List returns a range of schema versions
func (s *InMemoryVersionStore) List(offset, limit int) ([]*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect and sort versions
	versions := make([]*evolution.SchemaVersion, 0, len(s.versions))
	for _, v := range s.versions {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].ID > versions[j].ID
	})

	// Apply offset and limit
	start := offset
	if start >= len(versions) {
		return []*evolution.SchemaVersion{}, nil
	}

	end := start + limit
	if end > len(versions) {
		end = len(versions)
	}

	return versions[start:end], nil
}

// GetByHash retrieves a version by its hash
func (s *InMemoryVersionStore) GetByHash(hash string) (*evolution.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.hashes[hash]
	if !exists {
		return nil, fmt.Errorf("version with hash %s not found", hash)
	}

	return s.versions[id], nil
}

// Delete removes a schema version
func (s *InMemoryVersionStore) Delete(id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	version, exists := s.versions[id]
	if !exists {
		return fmt.Errorf("version %d not found", id)
	}

	delete(s.versions, id)
	delete(s.hashes, version.Hash)

	return nil
}

// NextID returns the next available version ID
func (s *InMemoryVersionStore) NextID() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.maxID++
	return s.maxID, nil
}

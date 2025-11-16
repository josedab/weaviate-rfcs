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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/entities/schema/evolution"
)

func TestInMemoryVersionStore(t *testing.T) {
	store := NewInMemoryVersionStore()

	t.Run("NextID generates sequential IDs", func(t *testing.T) {
		id1, err := store.NextID()
		require.NoError(t, err)
		assert.Equal(t, uint64(1), id1)

		id2, err := store.NextID()
		require.NoError(t, err)
		assert.Equal(t, uint64(2), id2)
	})

	t.Run("Save and Get version", func(t *testing.T) {
		version := &evolution.SchemaVersion{
			ID:              1,
			Timestamp:       time.Now(),
			Author:          "test-user",
			Description:     "Test version",
			Hash:            "abc123",
			Compatibility:   evolution.BackwardCompatible,
			MigrationStatus: evolution.MigrationCompleted,
		}

		err := store.Save(version)
		require.NoError(t, err)

		retrieved, err := store.Get(1)
		require.NoError(t, err)
		assert.Equal(t, version.ID, retrieved.ID)
		assert.Equal(t, version.Author, retrieved.Author)
		assert.Equal(t, version.Hash, retrieved.Hash)
	})

	t.Run("GetByHash retrieves version", func(t *testing.T) {
		version := &evolution.SchemaVersion{
			ID:              2,
			Timestamp:       time.Now(),
			Author:          "test-user",
			Description:     "Test version 2",
			Hash:            "def456",
			Compatibility:   evolution.FullyCompatible,
			MigrationStatus: evolution.MigrationNotRequired,
		}

		err := store.Save(version)
		require.NoError(t, err)

		retrieved, err := store.GetByHash("def456")
		require.NoError(t, err)
		assert.Equal(t, uint64(2), retrieved.ID)
	})

	t.Run("GetLatest returns most recent version", func(t *testing.T) {
		version3 := &evolution.SchemaVersion{
			ID:              3,
			Timestamp:       time.Now(),
			Author:          "test-user",
			Description:     "Latest version",
			Hash:            "ghi789",
			Compatibility:   evolution.BackwardCompatible,
			MigrationStatus: evolution.MigrationCompleted,
		}

		err := store.Save(version3)
		require.NoError(t, err)

		latest, err := store.GetLatest()
		require.NoError(t, err)
		assert.Equal(t, uint64(3), latest.ID)
		assert.Equal(t, "Latest version", latest.Description)
	})

	t.Run("List returns versions with pagination", func(t *testing.T) {
		versions, err := store.List(0, 10)
		require.NoError(t, err)
		assert.Len(t, versions, 3)

		// Should be sorted by ID descending (newest first)
		assert.Equal(t, uint64(3), versions[0].ID)
		assert.Equal(t, uint64(2), versions[1].ID)
		assert.Equal(t, uint64(1), versions[2].ID)
	})

	t.Run("List with offset", func(t *testing.T) {
		versions, err := store.List(1, 2)
		require.NoError(t, err)
		assert.Len(t, versions, 2)
		assert.Equal(t, uint64(2), versions[0].ID)
		assert.Equal(t, uint64(1), versions[1].ID)
	})

	t.Run("Delete removes version", func(t *testing.T) {
		err := store.Delete(2)
		require.NoError(t, err)

		_, err = store.Get(2)
		assert.Error(t, err)

		_, err = store.GetByHash("def456")
		assert.Error(t, err)
	})

	t.Run("Get non-existent version returns error", func(t *testing.T) {
		_, err := store.Get(999)
		assert.Error(t, err)
	})
}

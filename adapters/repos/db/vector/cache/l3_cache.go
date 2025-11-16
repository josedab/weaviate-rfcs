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

package cache

import (
	"context"

	"github.com/weaviate/weaviate/adapters/repos/db/vector/common"
)

// L3Cache represents the cold storage tier
// Currently uses the existing vectorForID mechanism
// Can be enhanced with memory-mapped files in the future
type L3Cache struct {
	vectorForID common.VectorForID[float32]
}

// NewL3Cache creates a new L3 cache
func NewL3Cache(vectorForID common.VectorForID[float32]) *L3Cache {
	return &L3Cache{
		vectorForID: vectorForID,
	}
}

// Get retrieves a vector from L3 (disk/storage)
// Always fetches from the underlying storage
func (c *L3Cache) Get(ctx context.Context, id uint64) ([]float32, error) {
	return c.vectorForID(ctx, id)
}

// Prefetch can be used to hint that a vector will be needed soon
// In a memory-mapped implementation, this would prefetch pages
// Currently a no-op as it delegates to vectorForID
func (c *L3Cache) Prefetch(ctx context.Context, ids []uint64) error {
	// Future: Implement actual prefetching for memory-mapped files
	// For now, this is a no-op since we're using vectorForID
	return nil
}

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

import "time"

// TieredCacheConfig defines the configuration for the multi-tier cache system
type TieredCacheConfig struct {
	// L1Ratio is the fraction of cache budget allocated to L1 (hot, uncompressed)
	// Default: 0.1 (10%)
	L1Ratio float64

	// L2Ratio is the fraction of cache budget allocated to L2 (warm, compressed)
	// Default: 0.3 (30%)
	L2Ratio float64

	// L2Compressed indicates whether L2 should store compressed vectors
	// Default: true
	L2Compressed bool

	// Prefetching configuration (optional)
	Prefetching *PrefetchConfig

	// PromotionThreshold is the number of accesses required to promote from L2 to L1
	// Default: 3
	PromotionThreshold int
}

// PrefetchConfig defines the configuration for query pattern-based prefetching
type PrefetchConfig struct {
	// Enabled indicates whether prefetching is active
	Enabled bool

	// Interval is how often to run prefetching
	// Default: 60s
	Interval time.Duration

	// BatchSize is the number of vectors to prefetch per interval
	// Default: 100
	BatchSize int

	// TrackTemporal enables time-of-day pattern tracking
	// Default: true
	TrackTemporal bool

	// TrackSpatial enables neighbor co-access pattern tracking
	// Default: true
	TrackSpatial bool
}

// DefaultTieredCacheConfig returns the default configuration for tiered cache
func DefaultTieredCacheConfig() *TieredCacheConfig {
	return &TieredCacheConfig{
		L1Ratio:            0.1,
		L2Ratio:            0.3,
		L2Compressed:       true,
		PromotionThreshold: 3,
		Prefetching:        DefaultPrefetchConfig(),
	}
}

// DefaultPrefetchConfig returns the default prefetch configuration
func DefaultPrefetchConfig() *PrefetchConfig {
	return &PrefetchConfig{
		Enabled:       true,
		Interval:      60 * time.Second,
		BatchSize:     100,
		TrackTemporal: true,
		TrackSpatial:  true,
	}
}

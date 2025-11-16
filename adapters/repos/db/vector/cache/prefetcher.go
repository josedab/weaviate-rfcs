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
	"time"

	"github.com/sirupsen/logrus"
	enterrors "github.com/weaviate/weaviate/entities/errors"
)

// Prefetcher handles query pattern-based prefetching
type Prefetcher struct {
	cache    *TieredCache
	detector *QueryPatternDetector
	config   *PrefetchConfig
	logger   logrus.FieldLogger

	ctx    context.Context
	cancel context.CancelFunc
}

// NewPrefetcher creates a new prefetcher
func NewPrefetcher(cache *TieredCache, config *PrefetchConfig, logger logrus.FieldLogger) *Prefetcher {
	ctx, cancel := context.WithCancel(context.Background())

	return &Prefetcher{
		cache:    cache,
		detector: NewQueryPatternDetector(config.TrackTemporal, config.TrackSpatial),
		config:   config,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins the prefetching loop
func (p *Prefetcher) Start(ctx context.Context) {
	if !p.config.Enabled {
		return
	}

	f := func() {
		ticker := time.NewTicker(p.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.prefetchBatch()
			case <-p.ctx.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}

	enterrors.GoWrapper(f, p.logger)
}

// Stop stops the prefetching loop
func (p *Prefetcher) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// prefetchBatch prefetches a batch of predicted vectors
func (p *Prefetcher) prefetchBatch() {
	// Get predictions from pattern detector
	candidates := p.detector.PredictNext(p.config.BatchSize)

	if len(candidates) == 0 {
		return
	}

	// Filter out vectors already in L1 or L2
	toFetch := make([]uint64, 0, len(candidates))
	for _, id := range candidates {
		if !p.cache.l1.Contains(id) && !p.cache.l2.Contains(id) {
			toFetch = append(toFetch, id)
		}
	}

	if len(toFetch) == 0 {
		return
	}

	p.logger.WithFields(logrus.Fields{
		"action":     "prefetch_batch",
		"candidates": len(toFetch),
	}).Debug("prefetching vectors")

	// Prefetch vectors (load into L2)
	for _, id := range toFetch {
		// Use background context for prefetching
		vec, err := p.cache.l3.Get(context.Background(), id)
		if err != nil {
			// Log error but continue with other prefetches
			p.logger.WithFields(logrus.Fields{
				"action":    "prefetch_vector",
				"vector_id": id,
			}).WithError(err).Debug("failed to prefetch vector")
			continue
		}

		// Add to L2 cache
		p.cache.l2.Set(id, vec)
	}
}

// RecordAccess records a vector access for pattern detection
func (p *Prefetcher) RecordAccess(id uint64) {
	if p.detector != nil {
		p.detector.RecordAccess(id)
	}
}

// RecordBatchAccess records multiple vector accesses for pattern detection
func (p *Prefetcher) RecordBatchAccess(ids []uint64) {
	if p.detector != nil {
		p.detector.RecordBatchAccess(ids)
	}
}

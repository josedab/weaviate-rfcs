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
	"sort"
	"sync"
	"time"
)

// QueryPatternDetector analyzes access patterns to predict future accesses
type QueryPatternDetector struct {
	mu sync.RWMutex

	// Temporal patterns
	hourlyAccess map[int]*accessSet     // hour -> vector IDs
	dailyAccess  map[int]*accessSet     // day-of-week -> vector IDs

	// Spatial patterns (neighbor co-access)
	neighborClusters map[uint64]*accessSet // vectorID -> frequently co-accessed neighbors

	// Recent queries tracking
	recentQueries *CircularBuffer

	// Configuration
	trackTemporal bool
	trackSpatial  bool
}

// accessSet is a set of vector IDs with access counts
type accessSet struct {
	mu     sync.RWMutex
	counts map[uint64]int
}

func newAccessSet() *accessSet {
	return &accessSet{
		counts: make(map[uint64]int),
	}
}

func (as *accessSet) add(id uint64) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.counts[id]++
}

func (as *accessSet) getTop(n int) []uint64 {
	as.mu.RLock()
	defer as.mu.RUnlock()

	// Convert to slice for sorting
	type idCount struct {
		id    uint64
		count int
	}

	items := make([]idCount, 0, len(as.counts))
	for id, count := range as.counts {
		items = append(items, idCount{id, count})
	}

	// Sort by count descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	// Return top n
	result := make([]uint64, 0, n)
	for i := 0; i < len(items) && i < n; i++ {
		result = append(result, items[i].id)
	}

	return result
}

// QueryAccess represents a query access pattern
type QueryAccess struct {
	Timestamp   time.Time
	AccessedIDs []uint64
}

// CircularBuffer is a fixed-size buffer for recent queries
type CircularBuffer struct {
	mu     sync.RWMutex
	buffer []QueryAccess
	head   int
	size   int
	maxSize int
}

func newCircularBuffer(maxSize int) *CircularBuffer {
	return &CircularBuffer{
		buffer:  make([]QueryAccess, maxSize),
		head:    0,
		size:    0,
		maxSize: maxSize,
	}
}

func (cb *CircularBuffer) Add(access QueryAccess) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.buffer[cb.head] = access
	cb.head = (cb.head + 1) % cb.maxSize
	if cb.size < cb.maxSize {
		cb.size++
	}
}

func (cb *CircularBuffer) Last(n int) []QueryAccess {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if n > cb.size {
		n = cb.size
	}

	result := make([]QueryAccess, 0, n)
	for i := 0; i < n; i++ {
		idx := (cb.head - 1 - i + cb.maxSize) % cb.maxSize
		if idx < 0 {
			idx += cb.maxSize
		}
		result = append(result, cb.buffer[idx])
	}

	return result
}

// NewQueryPatternDetector creates a new pattern detector
func NewQueryPatternDetector(trackTemporal, trackSpatial bool) *QueryPatternDetector {
	return &QueryPatternDetector{
		hourlyAccess:     make(map[int]*accessSet),
		dailyAccess:      make(map[int]*accessSet),
		neighborClusters: make(map[uint64]*accessSet),
		recentQueries:    newCircularBuffer(1000),
		trackTemporal:    trackTemporal,
		trackSpatial:     trackSpatial,
	}
}

// RecordAccess records a vector access for pattern detection
func (qpd *QueryPatternDetector) RecordAccess(id uint64) {
	now := time.Now()

	if qpd.trackTemporal {
		// Record temporal patterns
		hour := now.Hour()
		dayOfWeek := int(now.Weekday())

		qpd.mu.Lock()
		if qpd.hourlyAccess[hour] == nil {
			qpd.hourlyAccess[hour] = newAccessSet()
		}
		if qpd.dailyAccess[dayOfWeek] == nil {
			qpd.dailyAccess[dayOfWeek] = newAccessSet()
		}
		qpd.mu.Unlock()

		qpd.hourlyAccess[hour].add(id)
		qpd.dailyAccess[dayOfWeek].add(id)
	}

	// Record in recent queries
	qpd.recentQueries.Add(QueryAccess{
		Timestamp:   now,
		AccessedIDs: []uint64{id},
	})
}

// RecordBatchAccess records multiple vector accesses (for spatial patterns)
func (qpd *QueryPatternDetector) RecordBatchAccess(ids []uint64) {
	now := time.Now()

	if qpd.trackSpatial {
		// Build spatial patterns (co-access)
		qpd.mu.Lock()
		for _, id := range ids {
			if qpd.neighborClusters[id] == nil {
				qpd.neighborClusters[id] = newAccessSet()
			}

			// Record all other IDs as neighbors
			for _, otherId := range ids {
				if otherId != id {
					qpd.neighborClusters[id].add(otherId)
				}
			}
		}
		qpd.mu.Unlock()
	}

	if qpd.trackTemporal {
		hour := now.Hour()
		dayOfWeek := int(now.Weekday())

		qpd.mu.Lock()
		if qpd.hourlyAccess[hour] == nil {
			qpd.hourlyAccess[hour] = newAccessSet()
		}
		if qpd.dailyAccess[dayOfWeek] == nil {
			qpd.dailyAccess[dayOfWeek] = newAccessSet()
		}
		qpd.mu.Unlock()

		for _, id := range ids {
			qpd.hourlyAccess[hour].add(id)
			qpd.dailyAccess[dayOfWeek].add(id)
		}
	}

	// Record in recent queries
	qpd.recentQueries.Add(QueryAccess{
		Timestamp:   now,
		AccessedIDs: ids,
	})
}

// PredictNext predicts the next n vector IDs likely to be accessed
func (qpd *QueryPatternDetector) PredictNext(n int) []uint64 {
	scores := make(map[uint64]float64)

	// Temporal prediction (50% weight)
	if qpd.trackTemporal {
		now := time.Now()
		hour := now.Hour()
		dayOfWeek := int(now.Weekday())

		qpd.mu.RLock()
		if hourSet := qpd.hourlyAccess[hour]; hourSet != nil {
			topHourly := hourSet.getTop(n * 2)
			for _, id := range topHourly {
				scores[id] += 0.35 // 35% weight for hourly pattern
			}
		}

		if daySet := qpd.dailyAccess[dayOfWeek]; daySet != nil {
			topDaily := daySet.getTop(n * 2)
			for _, id := range topDaily {
				scores[id] += 0.15 // 15% weight for daily pattern
			}
		}
		qpd.mu.RUnlock()
	}

	// Spatial prediction (50% weight) - neighbors of recent accesses
	if qpd.trackSpatial {
		recent := qpd.recentQueries.Last(10)
		qpd.mu.RLock()
		for _, query := range recent {
			for _, id := range query.AccessedIDs {
				if neighbors := qpd.neighborClusters[id]; neighbors != nil {
					topNeighbors := neighbors.getTop(n)
					for _, neighborID := range topNeighbors {
						scores[neighborID] += 0.5 // 50% weight for spatial
					}
				}
			}
		}
		qpd.mu.RUnlock()
	}

	// Sort by score and return top n
	type idScore struct {
		id    uint64
		score float64
	}

	items := make([]idScore, 0, len(scores))
	for id, score := range scores {
		items = append(items, idScore{id, score})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	result := make([]uint64, 0, n)
	for i := 0; i < len(items) && i < n; i++ {
		result = append(result, items[i].id)
	}

	return result
}

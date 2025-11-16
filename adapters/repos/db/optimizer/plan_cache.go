package optimizer

import (
	"container/list"
	"sync"
	"time"
)

// PlanCache provides an LRU cache for query plans.
type PlanCache struct {
	maxSize  int
	cache    map[string]*cacheEntry
	lruList  *list.List
	mu       sync.RWMutex
	ttl      time.Duration
	hitCount int64
	missCount int64
}

// cacheEntry represents a cached query plan with metadata.
type cacheEntry struct {
	key       string
	plan      *QueryPlan
	element   *list.Element
	timestamp time.Time
	hits      int64
}

// NewPlanCache creates a new plan cache with the specified maximum size.
func NewPlanCache(maxSize int, ttl time.Duration) *PlanCache {
	return &PlanCache{
		maxSize: maxSize,
		cache:   make(map[string]*cacheEntry),
		lruList: list.New(),
		ttl:     ttl,
	}
}

// Get retrieves a cached plan by query hash.
// Returns nil if the plan is not in the cache or has expired.
func (pc *PlanCache) Get(queryHash string) *QueryPlan {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	entry, exists := pc.cache[queryHash]
	if !exists {
		pc.missCount++
		return nil
	}

	// Check if entry has expired
	if pc.ttl > 0 && time.Since(entry.timestamp) > pc.ttl {
		pc.remove(entry)
		pc.missCount++
		return nil
	}

	// Move to front (most recently used)
	pc.lruList.MoveToFront(entry.element)
	entry.hits++
	pc.hitCount++

	return entry.plan
}

// Store adds or updates a query plan in the cache.
func (pc *PlanCache) Store(queryHash string, plan *QueryPlan) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Check if already exists
	if entry, exists := pc.cache[queryHash]; exists {
		entry.plan = plan
		entry.timestamp = time.Now()
		pc.lruList.MoveToFront(entry.element)
		return
	}

	// Evict if at capacity
	if pc.lruList.Len() >= pc.maxSize {
		pc.evictLRU()
	}

	// Add new entry
	entry := &cacheEntry{
		key:       queryHash,
		plan:      plan,
		timestamp: time.Now(),
		hits:      0,
	}

	entry.element = pc.lruList.PushFront(entry)
	pc.cache[queryHash] = entry
}

// Invalidate removes a specific plan from the cache.
func (pc *PlanCache) Invalidate(queryHash string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if entry, exists := pc.cache[queryHash]; exists {
		pc.remove(entry)
	}
}

// Clear removes all entries from the cache.
func (pc *PlanCache) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache = make(map[string]*cacheEntry)
	pc.lruList = list.New()
	pc.hitCount = 0
	pc.missCount = 0
}

// Size returns the current number of cached plans.
func (pc *PlanCache) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.cache)
}

// Stats returns cache statistics.
func (pc *PlanCache) Stats() CacheStats {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return CacheStats{
		Size:      len(pc.cache),
		MaxSize:   pc.maxSize,
		Hits:      pc.hitCount,
		Misses:    pc.missCount,
		HitRate:   pc.calculateHitRate(),
	}
}

// evictLRU removes the least recently used entry.
func (pc *PlanCache) evictLRU() {
	element := pc.lruList.Back()
	if element != nil {
		entry := element.Value.(*cacheEntry)
		pc.remove(entry)
	}
}

// remove removes an entry from the cache.
func (pc *PlanCache) remove(entry *cacheEntry) {
	pc.lruList.Remove(entry.element)
	delete(pc.cache, entry.key)
}

// calculateHitRate calculates the cache hit rate.
func (pc *PlanCache) calculateHitRate() float64 {
	total := pc.hitCount + pc.missCount
	if total == 0 {
		return 0.0
	}
	return float64(pc.hitCount) / float64(total)
}

// EvictExpired removes all expired entries from the cache.
func (pc *PlanCache) EvictExpired() int {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.ttl == 0 {
		return 0
	}

	evicted := 0
	now := time.Now()

	// Iterate through all entries
	for _, entry := range pc.cache {
		if now.Sub(entry.timestamp) > pc.ttl {
			pc.remove(entry)
			evicted++
		}
	}

	return evicted
}

// CacheStats contains statistics about the plan cache.
type CacheStats struct {
	Size    int     // Current number of cached plans
	MaxSize int     // Maximum cache size
	Hits    int64   // Total cache hits
	Misses  int64   // Total cache misses
	HitRate float64 // Hit rate (0.0 to 1.0)
}

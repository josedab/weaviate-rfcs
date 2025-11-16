package optimizer

import (
	"testing"
	"time"
)

func TestNewPlanCache(t *testing.T) {
	cache := NewPlanCache(100, time.Minute)

	if cache == nil {
		t.Fatal("NewPlanCache returned nil")
	}

	if cache.Size() != 0 {
		t.Errorf("Expected empty cache, got size %d", cache.Size())
	}
}

func TestPlanCache_StoreAndGet(t *testing.T) {
	cache := NewPlanCache(100, time.Minute)

	plan := &QueryPlan{
		Type: PlanTypeSeqScan,
		Cost: 100.0,
	}

	// Store plan
	cache.Store("query1", plan)

	// Retrieve plan
	retrieved := cache.Get("query1")
	if retrieved == nil {
		t.Fatal("Expected to retrieve plan, got nil")
	}

	if retrieved.Cost != 100.0 {
		t.Errorf("Expected cost=100.0, got %f", retrieved.Cost)
	}
}

func TestPlanCache_GetMiss(t *testing.T) {
	cache := NewPlanCache(100, time.Minute)

	plan := cache.Get("nonexistent")
	if plan != nil {
		t.Error("Expected nil for cache miss")
	}

	stats := cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestPlanCache_HitRate(t *testing.T) {
	cache := NewPlanCache(100, time.Minute)

	plan := &QueryPlan{Cost: 100.0}
	cache.Store("query1", plan)

	// 2 hits
	cache.Get("query1")
	cache.Get("query1")

	// 1 miss
	cache.Get("nonexistent")

	stats := cache.Stats()
	expectedHitRate := 2.0 / 3.0 // 2 hits out of 3 total

	if stats.HitRate < expectedHitRate-0.01 || stats.HitRate > expectedHitRate+0.01 {
		t.Errorf("Expected hit rate ~%f, got %f", expectedHitRate, stats.HitRate)
	}
}

func TestPlanCache_Eviction(t *testing.T) {
	cache := NewPlanCache(3, time.Minute) // Max 3 items

	// Add 4 plans
	cache.Store("plan1", &QueryPlan{Cost: 1.0})
	cache.Store("plan2", &QueryPlan{Cost: 2.0})
	cache.Store("plan3", &QueryPlan{Cost: 3.0})
	cache.Store("plan4", &QueryPlan{Cost: 4.0}) // This should evict plan1

	// plan1 should be evicted
	if cache.Get("plan1") != nil {
		t.Error("Expected plan1 to be evicted")
	}

	// plan4 should be present
	if cache.Get("plan4") == nil {
		t.Error("Expected plan4 to be present")
	}

	// Size should be at most 3
	if cache.Size() > 3 {
		t.Errorf("Expected size <= 3, got %d", cache.Size())
	}
}

func TestPlanCache_LRU(t *testing.T) {
	cache := NewPlanCache(3, time.Minute)

	// Add 3 plans
	cache.Store("plan1", &QueryPlan{Cost: 1.0})
	cache.Store("plan2", &QueryPlan{Cost: 2.0})
	cache.Store("plan3", &QueryPlan{Cost: 3.0})

	// Access plan1 to make it most recently used
	cache.Get("plan1")

	// Add plan4, which should evict plan2 (least recently used)
	cache.Store("plan4", &QueryPlan{Cost: 4.0})

	// plan2 should be evicted
	if cache.Get("plan2") != nil {
		t.Error("Expected plan2 to be evicted")
	}

	// plan1 should still be present
	if cache.Get("plan1") == nil {
		t.Error("Expected plan1 to still be present")
	}
}

func TestPlanCache_Invalidate(t *testing.T) {
	cache := NewPlanCache(100, time.Minute)

	cache.Store("plan1", &QueryPlan{Cost: 1.0})

	// Verify it's stored
	if cache.Get("plan1") == nil {
		t.Fatal("Expected plan1 to be stored")
	}

	// Invalidate
	cache.Invalidate("plan1")

	// Should be gone
	if cache.Get("plan1") != nil {
		t.Error("Expected plan1 to be invalidated")
	}
}

func TestPlanCache_Clear(t *testing.T) {
	cache := NewPlanCache(100, time.Minute)

	cache.Store("plan1", &QueryPlan{Cost: 1.0})
	cache.Store("plan2", &QueryPlan{Cost: 2.0})

	if cache.Size() != 2 {
		t.Fatalf("Expected size=2, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size=0 after clear, got %d", cache.Size())
	}
}

func TestPlanCache_TTL(t *testing.T) {
	cache := NewPlanCache(100, 100*time.Millisecond)

	cache.Store("plan1", &QueryPlan{Cost: 1.0})

	// Should be retrievable immediately
	if cache.Get("plan1") == nil {
		t.Fatal("Expected plan1 to be retrievable")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	if cache.Get("plan1") != nil {
		t.Error("Expected plan1 to be expired")
	}
}

func TestPlanCache_EvictExpired(t *testing.T) {
	cache := NewPlanCache(100, 100*time.Millisecond)

	cache.Store("plan1", &QueryPlan{Cost: 1.0})
	cache.Store("plan2", &QueryPlan{Cost: 2.0})

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Evict expired entries
	evicted := cache.EvictExpired()

	if evicted != 2 {
		t.Errorf("Expected 2 evicted, got %d", evicted)
	}

	if cache.Size() != 0 {
		t.Errorf("Expected size=0, got %d", cache.Size())
	}
}

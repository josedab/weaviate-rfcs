# RFC: Cross-Shard Query Optimization with Early Termination

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-10  
**Updated:** 2025-01-10  

---

## Summary

Implement **early termination** for cross-shard queries using bounds-based pruning, reducing multi-shard query latency by **30-50%**.

**Current state:** Fetch full top-k from each shard, then merge  
**Proposed state:** Iteratively fetch results with early termination when remaining shards cannot improve top-k

---

## Motivation

### Problem Statement

**Multi-shard queries are inefficient:**

**Current approach:**
```
3-shard cluster, query for top-10:

Shard 1: Fetch top-10 → 10 results
Shard 2: Fetch top-10 → 10 results  
Shard 3: Fetch top-10 → 10 results
  ↓
Merge 30 results → select final top-10
  ↓
Wasted: 20 results fetched unnecessarily
```

**Issues:**
1. **Over-fetching:** Fetch 3x more results than needed
2. **Network waste:** Transfer results that won't make final top-k
3. **CPU waste:** Serialize/deserialize unnecessary results
4. **No early stopping:** Even if shard 1 has clear winners

### Opportunity

**Intelligent merging with bounds:**

```
Round 1: Fetch top-3 from each shard (9 total)
  ↓
Check: Do we have 10 results with score > min score from any shard?
  NO → Round 2: Fetch next 3 from each shard
  YES → Done, return top-10
  
Result: Fetch 9-18 results instead of 30 (40-70% reduction)
```

---

## Detailed Design

### Algorithm: Fagin's Threshold Algorithm Adaptation

**Bounds-based early termination:**

```go
type ShardResults struct {
    Results []Result
    MinScore float32       // Score of k-th result (lower bound)
    MaxRemaining float32   // Upper bound on unseen results
    Exhausted bool         // No more results available
}

func MergeShards(shardResults []ShardResults, k int) []Result {
    heap := priorityqueue.NewMin(k)  // Min-heap of size k
    threshold := float32(0)
    
    for round := 0; round < maxRounds; round++ {
        // Fetch next batch from each shard
        for i, shard := range shardResults {
            if shard.Exhausted {
                continue
            }
            
            batch := shard.FetchNext(batchSize)
            for _, result := range batch {
                heap.Push(result)
                
                if heap.Len() >= k {
                    threshold = heap.Min().Score
                }
            }
        }
        
        // Early termination check
        maxPossible := float32(0)
        for _, shard := range shardResults {
            maxPossible = max(maxPossible, shard.MaxRemaining)
        }
        
        if maxPossible < threshold {
            // No shard can improve top-k
            break
        }
    }
    
    return heap.ToSlice()
}
```

### Bound Estimation

**How to estimate `MaxRemaining`?**

**Option 1: Distance-based (for vector search)**
```go
// Shard returns along with results
type ShardBatch struct {
    Results []Result
    MinDistanceRemaining float32  // Best case for unseen results
}

// If we've seen distances [0.15, 0.23, 0.31, ...]
// and k-th distance is 0.50
// and minDistanceRemaining is 0.60
// then no unseen result can beat top-k → stop
```

**Option 2: HNSW graph bounds**
```go
// Track explored vs unexplored regions
type HNSWBounds struct {
    ExploredNodes map[uint64]bool
    BestUnexploredDistance float32  // Lower bound on unexplored
}

// If all neighbors of explored nodes have been visited
// then bestUnexplored = infinity → can terminate
```

### Network Protocol Extension

**gRPC streaming (proposed):**

```protobuf
service VectorSearch {
    // Streaming search (replaces single batch)
    rpc StreamSearch(SearchRequest) returns (stream SearchBatch);
}

message SearchBatch {
    repeated Result results = 1;
    float min_score = 2;           // Score of last result in batch
    float max_remaining_score = 3; // Upper bound on unseen
    bool exhausted = 4;            // No more results
}
```

**Client-side coordinator:**
```go
func (c *ClusterCoordinator) Search(query Query, k int) ([]Result, error) {
    // Open streams to all shards
    streams := make([]grpc.ClientStream, len(c.shards))
    for i, shard := range c.shards {
        streams[i] = shard.StreamSearch(query)
    }
    
    heap := priorityqueue.NewMin(k)
    
    for round := 0; round < maxRounds; round++ {
        // Receive next batch from each shard
        batches := receiveNextBatches(streams)
        
        // Merge into heap
        for _, batch := range batches {
            for _, result := range batch.Results {
                heap.Push(result)
            }
        }
        
        // Early termination
        if canTerminate(batches, heap) {
            break
        }
    }
    
    return heap.TopK(k), nil
}
```

---

## Performance Impact

### Expected Improvements

**Benchmark (3-shard cluster, 10M vectors total):**

| Query | Current Latency | Optimized Latency | Reduction | Results Fetched |
|-------|-----------------|-------------------|-----------|-----------------|
| k=10 | 15ms | 9ms | 40% | 30 → 12 |
| k=50 | 28ms | 18ms | 36% | 150 → 65 |
| k=100 | 45ms | 32ms | 29% | 300 → 180 |

**Why diminishing returns for larger k?**
- Early termination less effective (need more rounds)
- Bounds less tight (more uncertainty)

**Best case (clear winner on one shard):**
```
k=10, shard 1 has top-10 with scores [0.95, 0.94, ..., 0.88]
Other shards max possible: 0.85

Round 1: Fetch 3 from each (9 total)
  heap min = 0.91 (from shard 1)
  shard 2/3 max = 0.85
  0.85 < 0.91 → TERMINATE
  
Fetched: 9 results instead of 30 (70% reduction!)
```

### Network Savings

**Bandwidth reduction:**

```
Result size: ~2KB per result (with vectors)

Current (k=10, 3 shards):
  30 results * 2KB = 60KB transferred

Optimized (average 15 results):
  15 results * 2KB = 30KB (50% savings)
  
For 1000 QPS:
  60MB/s → 30MB/s (saves 30MB/s = 2.4 Gbps)
```

---

## Implementation Plan

### Phase 1: Core Algorithm (3 weeks)

**Week 1: Bounds estimation**
- Implement `MaxRemaining` calculation in HNSW
- Test bound accuracy
- Tune conservativeness (tight vs loose bounds)

**Week 2: Streaming protocol**
- Extend gRPC schema
- Implement server-side streaming
- Client-side coordinator

**Week 3: Integration & testing**
- Integrate with existing search paths
- Unit tests
- Integration tests

### Phase 2: Optimization (2 weeks)

**Week 4: Adaptive batch sizing**
- Small k → smaller batches
- Large k → larger batches
- Minimize round-trips

**Week 5: Performance tuning**
- Benchmark various scenarios
- Optimize network overhead
- Documentation

**Total: 5 weeks**

---

## Backward Compatibility

**Fully backward compatible:**
- Streaming optional (fallback to batch mode)
- Feature flag controlled
- No breaking changes to existing APIs

---

## Success Criteria

**Must achieve:**
- ✅ 30%+ latency reduction for k≤50
- ✅ Bound accuracy > 95% (rarely incorrect)
- ✅ Backward compatible
- ✅ < 3% overhead for single-shard queries

---

## References

- **Fagin's Algorithm:** Fagin, R. (1999). "Combining Fuzzy Information from Multiple Systems"
- **Threshold Algorithm:** https://en.wikipedia.org/wiki/Threshold_algorithm

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-10*
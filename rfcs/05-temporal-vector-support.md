# RFC: Native Temporal Vector Support for Weaviate

**Status:** Proposed  
**Author:** Jose David Baena (@josedab)  
**Created:** 2025-01-10  
**Updated:** 2025-01-10  

---

## Summary

Add native support for time-decay scoring in vector search, enabling trending content discovery and time-sensitive semantic search.

**Current state:** Must implement time-decay in application code (post-processing)  
**Proposed state:** Native `timeDecay` parameter in vector search queries

---

## Motivation

### Problem Statement

**Many use cases need temporal relevance:**

1. **News/content platforms**
   - Recent articles more relevant than old
   - Example: "AI breakthroughs" → want 2024 results, not 2018

2. **E-commerce**
   - New products boost
   - Seasonal trends

3. **Social media**
   - Trending topics (recency = relevance)

4. **Customer support**
   - Recent documentation more accurate (product changes)

**Current workaround (inefficient):**

```python
# 1. Fetch vectors (ignore time)
results = collection.query.near_text(
    query="AI news",
    limit=100  # Over-fetch
)

# 2. Apply time decay in application
import math
from datetime import datetime

def time_decay(published_at, half_life_days=7):
    age_days = (datetime.now() - published_at).days
    return math.exp(-age_days / half_life_days)

# 3. Re-rank
for result in results:
    result.score *= time_decay(result.properties["publishedAt"])

# 4. Sort and return top-10
sorted_results = sorted(results, key=lambda x: x.score, reverse=True)[:10]
```

**Issues:**
- Must over-fetch (100 for top-10)
- Application-side re-ranking (network overhead)
- Inefficient (scores all 100, only need 10)

---

## Detailed Design

### API Extension

**GraphQL:**

```graphql
{
  Get {
    Article(
      nearText: {concepts: ["AI news"]}
      timeDecay: {
        property: "publishedAt"
        halfLife: "7d"      # Days, hours, etc.
        decayFunction: EXPONENTIAL  # EXPONENTIAL | LINEAR | STEP
      }
      limit: 10
    ) {
      title
      publishedAt
      _additional {
        score          # Combined (semantic + time decay)
        vectorScore    # Pure semantic
        timeDecayScore # Time component
      }
    }
  }
}
```

**Python client:**

```python
results = collection.query.near_text(
    query="AI news",
    limit=10,
    time_decay=TimeDecay(
        property="publishedAt",
        half_life="7d",
        decay_function=DecayFunction.EXPONENTIAL
    )
)
```

### Decay Functions

**1. Exponential Decay (recommended)**

```
score(d) = semantic_similarity(q, d) * exp(-age(d) / halfLife)

where:
  age(d) = now() - d.publishedAt
  halfLife = configurable (e.g., 7 days)
```

**Example:**
```
Article published:
  - Today: decay = exp(-0/7) = 1.0 (full weight)
  - 7 days ago: decay = exp(-7/7) = exp(-1) = 0.37
  - 14 days ago: decay = exp(-14/7) = 0.14
  - 30 days ago: decay = exp(-30/7) = 0.01 (nearly zero)
```

**2. Linear Decay**

```
score(d) = semantic_similarity(q, d) * max(0, 1 - age(d) / maxAge)
```

**Example (maxAge = 30 days):**
```
  - Today: decay = 1.0
  - 15 days ago: decay = 0.5
  - 30 days ago: decay = 0.0 (cutoff)
  - 60 days ago: decay = 0.0
```

**3. Step Decay**

```
score(d) = semantic_similarity(q, d) * stepFunction(age(d))

stepFunction:
  age < 7 days: 1.0
  age < 30 days: 0.5
  age < 90 days: 0.2
  age >= 90 days: 0.0
```

### Implementation

**Storage:**

```go
// Add timestamp to vector index
type vertex struct {
    id uint64
    level int
    connections *PackedConnections
    timestamp time.Time  // NEW: object timestamp
}
```

**Search with time decay:**

```go
func (h *hnsw) SearchWithTimeDecay(
    ctx context.Context,
    vector []float32,
    k int,
    timeDecay *TimeDecayConfig,
) ([]Result, error) {
    // 1. Over-fetch to account for decay re-ranking
    overFetchMultiplier := calculateOverFetch(timeDecay)
    candidates := h.Search(ctx, vector, k * overFetchMultiplier)
    
    // 2. Apply time decay
    now := time.Now()
    for i := range candidates {
        timestamp := h.getTimestamp(candidates[i].ID)
        age := now.Sub(timestamp)
        
        decay := timeDecay.ComputeDecay(age)
        candidates[i].Score *= decay  // Modify score
    }
    
    // 3. Re-rank and return top-k
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].Score > candidates[j].Score
    })
    
    return candidates[:k], nil
}

type TimeDecayConfig struct {
    Property string
    HalfLife time.Duration
    DecayFunction DecayFunc
}

type DecayFunc func(age time.Duration) float32

func ExponentialDecay(halfLife time.Duration) DecayFunc {
    return func(age time.Duration) float32 {
        return float32(math.Exp(-float64(age) / float64(halfLife)))
    }
}
```

---

## Performance Impact

### Over-Fetch Multiplier

**Challenge:** Time decay changes ranking → must fetch more candidates

**Analysis:**

```
Scenario: Want top-10 with time decay (halfLife = 7 days)

Without decay:
  fetch k=10, return 10 ✓

With decay:
  fetch k=10 → after decay re-ranking, top-10 changes
  
  Example:
    Vector rank 5, published today → decay 1.0 → stays rank 5
    Vector rank 2, published 30 days ago → decay 0.01 → drops to rank 50
    Vector rank 50, published today → decay 1.0 → rises to rank 2
    
  Solution: Over-fetch, then select top-k after decay
```

**Optimal multiplier (empirical):**

| Half-Life | Dataset Age Range | Recommended Multiplier |
|-----------|-------------------|------------------------|
| 1 day | 30 days | 5x |
| 7 days | 90 days | 3x |
| 30 days | 1 year | 2x |
| 90 days | 5 years | 1.5x |

**Latency impact:**
- 3x over-fetch: +30% latency (fetch 30 instead of 10)
- Re-ranking: +0.5ms (compute decay for 30 vectors)
- Total: ~+35% latency

**Trade-off:** Temporal relevance worth 35% latency cost for time-sensitive use cases.

---

## Use Cases

### Use Case 1: News Aggregator

**Requirement:** Recent news more relevant

```graphql
{
  Get {
    NewsArticle(
      nearText: {concepts: ["climate change"]}
      timeDecay: {
        property: "publishedAt"
        halfLife: "3d"  # 3-day half-life (fast decay)
        decayFunction: EXPONENTIAL
      }
      limit: 20
    ) {
      title
      publishedAt
      _additional {
        score
        vectorScore
        timeDecayScore
      }
    }
  }
}
```

**Effect:**
- Article from today, similarity 0.80 → score 0.80 * 1.0 = 0.80
- Article from week ago, similarity 0.85 → score 0.85 * 0.37 = 0.31
- Today's article ranks higher despite lower semantic similarity

### Use Case 2: E-Commerce Product Search

**Requirement:** Boost new products

```graphql
{
  Get {
    Product(
      nearText: {concepts: ["wireless headphones"]}
      timeDecay: {
        property: "createdAt"
        halfLife: "30d"  # Monthly refresh
        decayFunction: LINEAR
        maxAge: "180d"   # Ignore products >6 months old
      }
      limit: 20
    ) {
      title
      price
    }
  }
}
```

---

## Implementation Plan

### Phase 1: Core Functionality (3 weeks)

**Week 1: Storage**
- Add timestamp field to vertex structure
- Migrate existing indexes (backfill with object creation time)
- Commit log support

**Week 2: Search algorithm**
- Implement time decay scoring
- Over-fetch logic
- Decay function library

**Week 3: API integration**
- GraphQL schema extensions
- REST API support
- Client library updates

### Phase 2: Optimization (2 weeks)

**Week 4: Adaptive over-fetch**
- Learn optimal multiplier per half-life
- Dynamic adjustment based on results

**Week 5: Testing & benchmarks**
- Integration tests
- Performance benchmarks
- Documentation

**Total: 5 weeks**

---

## Backward Compatibility

**Fully backward compatible:**
- `timeDecay` parameter is optional
- Existing queries work unchanged
- Timestamp storage: Backfilled with object creation time (or current time if unknown)

---

## Success Criteria

**Must achieve:**
- ✅ Temporal relevance improves by user testing
- ✅ Latency overhead < 50% with 3x over-fetch
- ✅ Supports all date/datetime properties
- ✅ Backward compatible
- ✅ Documentation with examples

---

## References

- **Time-Aware Recommendation:** Various papers on temporal dynamics in recommender systems
- **Trending algorithms:** Reddit, Hacker News ranking algorithms

---

*RFC Version: 1.0*  
*Last Updated: 2025-01-10*
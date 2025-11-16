# Temporal Vector Support - User Guide

## Overview

Temporal vector support enables time-decay scoring in vector searches, allowing you to boost recent content in search results. This is particularly useful for:

- **News and content platforms**: Recent articles more relevant than old ones
- **E-commerce**: Boost new products in search results
- **Social media**: Trending topics where recency equals relevance
- **Customer support**: Recent documentation more accurate due to product changes

## How It Works

Time decay combines semantic similarity with temporal relevance:

```
final_score = semantic_similarity × time_decay_factor
```

Where:
- `semantic_similarity` is the standard vector similarity score
- `time_decay_factor` is calculated based on the age of the content

## Decay Functions

### 1. Exponential Decay (Recommended)

Exponential decay provides smooth, natural falloff over time.

**Formula**: `decay = exp(-age / halfLife)`

**When to use**: Most general cases where you want gradual decay.

**GraphQL Example**:
```graphql
{
  Get {
    Article(
      nearText: {concepts: ["AI news"]}
      timeDecay: {
        property: "publishedAt"
        halfLife: "7d"
        decayFunction: EXPONENTIAL
      }
      limit: 10
    ) {
      title
      publishedAt
    }
  }
}
```

**Effects**:
- Article from today: decay = 1.0 (full weight)
- Article from 7 days ago: decay = 0.37 (one half-life)
- Article from 14 days ago: decay = 0.14 (two half-lives)
- Article from 30 days ago: decay = 0.01 (nearly zero)

### 2. Linear Decay

Linear decay provides constant decay rate until a cutoff.

**Formula**: `decay = max(0, 1 - age / maxAge)`

**When to use**: When you want a hard cutoff at a specific age.

**GraphQL Example**:
```graphql
{
  Get {
    Product(
      nearText: {concepts: ["wireless headphones"]}
      timeDecay: {
        property: "createdAt"
        maxAge: "180d"
        decayFunction: LINEAR
      }
      limit: 20
    ) {
      title
      price
      createdAt
    }
  }
}
```

**Effects** (with maxAge = 30 days):
- Today: decay = 1.0
- 15 days ago: decay = 0.5
- 30 days ago: decay = 0.0 (cutoff)
- 60 days ago: decay = 0.0

### 3. Step Decay

Step decay applies discrete weights based on age ranges.

**When to use**: When you have distinct "freshness" categories.

**GraphQL Example**:
```graphql
{
  Get {
    SocialPost(
      nearText: {concepts: ["technology trends"]}
      timeDecay: {
        property: "postedAt"
        decayFunction: STEP
        stepThresholds: [
          {maxAge: "7d", weight: 1.0},
          {maxAge: "30d", weight: 0.5},
          {maxAge: "90d", weight: 0.2}
        ]
      }
      limit: 20
    ) {
      content
      postedAt
    }
  }
}
```

**Effects**:
- 0-7 days: weight = 1.0 (full boost)
- 7-30 days: weight = 0.5 (moderate boost)
- 30-90 days: weight = 0.2 (minimal boost)
- 90+ days: weight = 0.0 (no boost)

## Duration Format

All duration strings use the format: `<number><unit>`

**Supported units**:
- `s` - seconds
- `m` - minutes
- `h` - hours
- `d` - days
- `w` - weeks

**Examples**:
- `"7d"` - 7 days
- `"30d"` - 30 days
- `"12h"` - 12 hours
- `"2w"` - 2 weeks (14 days)

## Python Client Examples

```python
import weaviate
from weaviate.classes.query import Query, TimeDecay, DecayFunction
from datetime import datetime, timedelta

client = weaviate.connect_to_local()

# Example 1: Exponential decay for news articles
results = client.collections.get("Article").query.near_text(
    query="AI breakthroughs",
    limit=10,
    time_decay=TimeDecay(
        property="publishedAt",
        half_life="7d",
        decay_function=DecayFunction.EXPONENTIAL
    )
)

for article in results.objects:
    print(f"{article.properties['title']} - {article.properties['publishedAt']}")

# Example 2: Linear decay for products
results = client.collections.get("Product").query.near_text(
    query="wireless headphones",
    limit=20,
    time_decay=TimeDecay(
        property="createdAt",
        max_age="180d",
        decay_function=DecayFunction.LINEAR
    )
)

# Example 3: Step decay for social posts
results = client.collections.get("SocialPost").query.near_text(
    query="trending topics",
    limit=20,
    time_decay=TimeDecay(
        property="postedAt",
        decay_function=DecayFunction.STEP,
        step_thresholds=[
            {"maxAge": "7d", "weight": 1.0},
            {"maxAge": "30d", "weight": 0.5},
            {"maxAge": "90d", "weight": 0.2}
        ]
    )
)

client.close()
```

## REST API Examples

```bash
# Exponential decay example
curl -X POST http://localhost:8080/v1/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "{
      Get {
        Article(
          nearText: {concepts: [\"AI news\"]}
          timeDecay: {
            property: \"publishedAt\"
            halfLife: \"7d\"
            decayFunction: EXPONENTIAL
          }
          limit: 10
        ) {
          title
          publishedAt
        }
      }
    }"
  }'
```

## Performance Considerations

### Over-Fetch Multiplier

Time decay requires fetching more candidates than the final result limit to ensure accurate ranking after applying temporal weights.

**Default multipliers** (automatically applied):
- Half-life ≤ 1 day: 5x over-fetch
- Half-life ≤ 7 days: 3x over-fetch
- Half-life ≤ 30 days: 2x over-fetch
- Half-life > 30 days: 1.5x over-fetch

**Example**: Requesting `limit: 10` with `halfLife: "7d"` will internally fetch ~30 candidates, apply time decay, and return the top 10.

### Latency Impact

- **3x over-fetch**: ~30% latency increase
- **Re-ranking overhead**: ~0.5ms for 30 vectors
- **Total**: ~35% latency increase

This is generally acceptable for time-sensitive use cases where temporal relevance is critical.

## Property Requirements

The datetime property used for time decay must:

1. Be of type `date` in your schema
2. Exist on all objects (objects without the property get decay factor 1.0)
3. Be in ISO 8601 / RFC3339 format

**Schema Example**:
```json
{
  "class": "Article",
  "properties": [
    {
      "name": "title",
      "dataType": ["text"]
    },
    {
      "name": "publishedAt",
      "dataType": ["date"]
    }
  ]
}
```

## Use Case Examples

### News Aggregator

**Goal**: Recent news more relevant

```graphql
{
  Get {
    NewsArticle(
      nearText: {concepts: ["climate change"]}
      timeDecay: {
        property: "publishedAt"
        halfLife: "3d"  # Fast decay for news
        decayFunction: EXPONENTIAL
      }
      limit: 20
    ) {
      title
      publishedAt
      summary
    }
  }
}
```

### E-Commerce Product Search

**Goal**: Boost new products

```graphql
{
  Get {
    Product(
      nearText: {concepts: ["running shoes"]}
      timeDecay: {
        property: "createdAt"
        halfLife: "30d"  # Monthly product refresh
        decayFunction: LINEAR
        maxAge: "180d"   # Ignore products >6 months old
      }
      limit: 20
    ) {
      name
      price
      createdAt
    }
  }
}
```

### Customer Support Knowledge Base

**Goal**: Recent docs more accurate

```graphql
{
  Get {
    Documentation(
      nearText: {concepts: ["API authentication"]}
      timeDecay: {
        property: "lastUpdated"
        halfLife: "90d"
        decayFunction: EXPONENTIAL
      }
      limit: 5
    ) {
      title
      content
      lastUpdated
    }
  }
}
```

### Social Media Trending

**Goal**: Recent posts are more relevant

```graphql
{
  Get {
    Post(
      nearText: {concepts: ["AI developments"]}
      timeDecay: {
        property: "postedAt"
        decayFunction: STEP
        stepThresholds: [
          {maxAge: "24h", weight: 1.0},
          {maxAge: "7d", weight: 0.3},
          {maxAge: "30d", weight: 0.1}
        ]
      }
      limit: 50
    ) {
      content
      postedAt
      author
    }
  }
}
```

## Best Practices

1. **Choose the right decay function**:
   - **Exponential**: Most use cases (smooth, natural decay)
   - **Linear**: When you need a hard cutoff
   - **Step**: When you have distinct freshness tiers

2. **Set appropriate half-life / max age**:
   - News/social media: 1-7 days
   - E-commerce: 30-90 days
   - Documentation: 90-180 days

3. **Monitor performance**:
   - Time decay adds ~35% latency with default settings
   - Consider using for top-level queries, not nested searches

4. **Test your configuration**:
   - Use different half-life values
   - Compare results with and without time decay
   - Validate that recent content appears first

5. **Combine with filters**:
   - Use `where` filters for hard requirements
   - Use time decay for soft temporal preferences

```graphql
{
  Get {
    Article(
      nearText: {concepts: ["technology"]}
      where: {
        operator: GreaterThan
        path: ["publishedAt"]
        valueDate: "2024-01-01T00:00:00Z"
      }
      timeDecay: {
        property: "publishedAt"
        halfLife: "7d"
        decayFunction: EXPONENTIAL
      }
      limit: 10
    ) {
      title
      publishedAt
    }
  }
}
```

## Troubleshooting

### Results don't reflect temporal order

**Check**:
1. Property name is correct
2. Property is type `date` in schema
3. All objects have the property
4. Half-life is appropriate for your data age range

### Performance is too slow

**Solutions**:
1. Increase half-life (reduces over-fetch multiplier)
2. Reduce result limit
3. Use filters to narrow search space first

### Old results still appearing

**Solutions**:
1. Decrease half-life for faster decay
2. Use LINEAR decay with maxAge for hard cutoff
3. Add a `where` filter to exclude old content

## FAQ

**Q: Can I use time decay with hybrid search?**
A: Yes, time decay works with all vector search types (nearVector, nearText, nearObject, hybrid).

**Q: What happens if an object doesn't have the timestamp property?**
A: It receives a decay factor of 1.0 (no decay penalty).

**Q: Can I use multiple time decay configs in one query?**
A: No, only one time decay configuration per query.

**Q: Does time decay affect BM25/keyword search?**
A: Time decay only affects vector similarity scoring. For hybrid search, it affects the vector component of the fusion.

**Q: What timestamp formats are supported?**
A: ISO 8601 / RFC3339 formats (e.g., "2024-01-15T10:30:00Z")

## Related Documentation

- [Vector Search Guide](https://weaviate.io/developers/weaviate/search/similarity)
- [Schema Configuration](https://weaviate.io/developers/weaviate/configuration/schema)
- [GraphQL API Reference](https://weaviate.io/developers/weaviate/api/graphql)
- [Hybrid Search](https://weaviate.io/developers/weaviate/search/hybrid)

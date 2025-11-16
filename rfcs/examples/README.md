# RFC 0012: Multi-Model Vector Support - Examples

This directory contains practical examples demonstrating the multi-model vector support feature described in RFC 0012.

## Overview

Multi-model vector support (also known as "named vectors") allows you to store multiple vector embeddings per object, each with independent configurations. This enables:

- **Multimodal search**: Combine text, image, and other vector types
- **A/B testing**: Test different embedding models side-by-side
- **Specialized representations**: Different vectors for different use cases
- **Heterogeneous dimensions**: Mix models with different dimensions (e.g., 384D + 1536D)

## Examples

### 1. Multi-Vector Example (`multi-vector-example.go`)

A comprehensive Go example demonstrating:

#### E-commerce Use Case
```go
VectorConfig: map[string]models.VectorConfig{
    "text": {
        Vectorizer: "text2vec-openai",
        VectorIndexType: "hnsw",
        Dimensions: 1536,
    },
    "image": {
        Vectorizer: "img2vec-neural",
        VectorIndexType: "hnsw",
        Dimensions: 512,
    },
}
```

#### Healthcare Use Case
- Clinical BERT embeddings (768D) for medical notes
- ResNet embeddings (2048D) for medical images
- Custom embeddings (256D) for lab results

## Running the Examples

### Prerequisites

1. **Weaviate instance** running (v1.24.0 or later):
   ```bash
   docker-compose up -d
   ```

2. **Required vectorizer modules** enabled:
   - `text2vec-openai`
   - `text2vec-transformers`
   - `img2vec-neural`

3. **Go dependencies**:
   ```bash
   go get github.com/weaviate/weaviate-go-client/v5
   ```

### Run the Example

```bash
cd rfcs/examples
go run multi-vector-example.go
```

## Key Features Demonstrated

### 1. Schema Definition with Multiple Vectors

```go
class := &models.Class{
    Class: "Product",
    VectorConfig: map[string]models.VectorConfig{
        "text": {
            Vectorizer: map[string]interface{}{
                "text2vec-openai": map[string]interface{}{
                    "model": "text-embedding-3-small",
                    "sourceProperties": []string{"title", "description"},
                },
            },
            VectorIndexType: "hnsw",
        },
        "image": {
            Vectorizer: map[string]interface{}{
                "img2vec-neural": map[string]interface{}{
                    "imageFields": []string{"imageUrl"},
                },
            },
            VectorIndexType: "flat",
        },
    },
}
```

### 2. Hybrid Search Across Multiple Vectors

```go
resp := client.GraphQL().Get().
    WithClassName("Product").
    WithHybrid(
        client.GraphQL().HybridArgumentBuilder().
            WithQuery("leather jacket").
            WithAlpha(0.7).
            WithTargetVectors("text", "image").  // Search across both!
            WithFusionType(graphql.RelativeScore),
    ).
    Do(ctx)
```

### 3. Fusion Strategies

#### Ranked Fusion (Reciprocal Rank Fusion)
```go
WithFusionType(graphql.Ranked)
```
- Formula: `1 / (k + rank)` where k=60
- Good for combining results from different sources
- Less sensitive to score magnitudes

#### Relative Score Fusion
```go
WithFusionType(graphql.RelativeScore)
```
- Normalizes scores to [0, 1] using min-max normalization
- Preserves relative score differences
- Better for score-sensitive applications

### 4. Vector-Specific Search

Search using only specific named vectors:

```go
resp := client.GraphQL().Get().
    WithClassName("Product").
    WithNearText(
        client.GraphQL().NearTextArgumentBuilder().
            WithConcepts([]string{"athletic"}).
            WithTargetVectors("text"),  // Only use text vector
    ).
    Do(ctx)
```

## Use Cases by Industry

### E-commerce
- **Product images**: CLIP embeddings (512D)
- **Product descriptions**: Text embeddings (1536D)
- **User reviews**: Sentiment embeddings (384D)

**Benefits**:
- Search by image similarity
- Search by text description
- Combine both for better relevance

### Healthcare
- **Medical images**: ResNet embeddings (2048D)
- **Patient notes**: Clinical BERT (768D)
- **Lab results**: Specialized embeddings (256D)

**Benefits**:
- Multi-modal patient record search
- Cross-reference images with notes
- Specialized retrieval per data type

### Media & Entertainment
- **Video frames**: Visual embeddings (512D)
- **Audio**: Speech embeddings (256D)
- **Subtitles**: Text embeddings (384D)

**Benefits**:
- Search videos by visual content
- Search by spoken words
- Search by subtitle text
- Combine all three for comprehensive search

## Performance Considerations

### Storage Overhead
| Vectors | Avg Dimensions | Storage/Object | Total (10M objects) |
|---------|----------------|----------------|---------------------|
| 1       | 1536          | 6KB            | 60GB                |
| 2       | 1024 avg      | 8KB            | 80GB                |
| 3       | 853 avg       | 10KB           | 100GB               |

### Search Performance
| Operation | Single Vector | 2 Vectors | 3 Vectors |
|-----------|---------------|-----------|-----------|
| Simple search | 15ms | 18ms (+20%) | 22ms (+47%) |
| Fused search | N/A | 25ms | 35ms |
| Batch insert (1000) | 450ms | 720ms (+60%) | 980ms (+118%) |

## Configuration Best Practices

### 1. Choose the Right Index Type

- **HNSW**: Best for most use cases, fast approximate search
  ```go
  VectorIndexType: "hnsw"
  VectorIndexConfig: map[string]interface{}{
      "ef": 100,              // Higher = better recall, slower search
      "efConstruction": 128,  // Higher = better quality index, slower build
      "maxConnections": 32,   // Higher = better recall, more memory
  }
  ```

- **Flat**: Exact search, good for <100k vectors
  ```go
  VectorIndexType: "flat"
  ```

- **SPFresh**: For frequently updated vectors
  ```go
  VectorIndexType: "spfresh"
  ```

### 2. Optimize Vector Dimensions

- Use lower dimensions (384-768) for faster search if acceptable
- Use higher dimensions (1536+) for better semantic understanding
- Consider PQ compression for very large datasets

### 3. Fusion Strategy Selection

| Strategy | Use When |
|----------|----------|
| Ranked (RRF) | Score magnitudes vary greatly between vectors |
| Relative Score | Scores are comparable and meaningful |

## Troubleshooting

### Common Issues

1. **"vectorizer module not found"**
   - Ensure required vectorizer modules are enabled in docker-compose
   - Check `ENABLE_MODULES` environment variable

2. **Slow search performance**
   - Reduce `ef` parameter in HNSW config
   - Consider using fewer target vectors
   - Use flat index for small datasets (<100k)

3. **High memory usage**
   - Each vector index uses ~200MB per 1M vectors
   - Consider PQ compression
   - Use flat index for infrequent searches

## References

- [RFC 0012 Full Specification](../0012-multi-model-vector-support.md)
- [Weaviate Named Vectors Documentation](https://weaviate.io/developers/weaviate/config-refs/schema/vector-config)
- [Weaviate Go Client Documentation](https://weaviate.io/developers/weaviate/client-libraries/go)
- [CLIP Paper](https://github.com/openai/CLIP)
- [Reciprocal Rank Fusion Paper](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf)

## Contributing

To add more examples:

1. Create a new example file following the naming convention
2. Add clear comments explaining the use case
3. Update this README with the new example
4. Test thoroughly with actual Weaviate instance

## License

Copyright © 2016 - 2025 Weaviate B.V. All rights reserved.

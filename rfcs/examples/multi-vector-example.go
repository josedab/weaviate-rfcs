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

package main

import (
	"context"
	"fmt"
	"log"

	wvt "github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
	"github.com/weaviate/weaviate/entities/schema"
)

// MultiVectorExample demonstrates the multi-model vector support feature
// as described in RFC 0012. This example shows:
// 1. Creating a schema with multiple named vectors
// 2. Inserting objects with multiple vector representations
// 3. Performing hybrid search across multiple vectors
// 4. Using fusion strategies to combine results
func main() {
	ctx := context.Background()

	// Connect to Weaviate
	client, err := wvt.NewClient(wvt.Config{
		Scheme: "http",
		Host:   "localhost:8080",
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example 1: Create a multi-vector schema for e-commerce
	createEcommerceSchema(ctx, client)

	// Example 2: Insert products with multiple vector representations
	insertProducts(ctx, client)

	// Example 3: Search across multiple vectors with fusion
	searchProducts(ctx, client)

	// Example 4: Healthcare use case with medical data
	createHealthcareSchema(ctx, client)
}

// createEcommerceSchema creates a Product class with multiple vectors:
// - text: for product descriptions (text embeddings)
// - image: for product images (image embeddings)
// - reviews: for customer reviews (sentiment embeddings)
func createEcommerceSchema(ctx context.Context, client *wvt.Client) {
	fmt.Println("Creating e-commerce schema with multi-vector support...")

	class := &models.Class{
		Class:       "Product",
		Description: "E-commerce product with text, image, and review vectors",
		Properties: []*models.Property{
			{
				Name:     "title",
				DataType: []string{schema.DataTypeText.String()},
			},
			{
				Name:     "description",
				DataType: []string{schema.DataTypeText.String()},
			},
			{
				Name:     "imageUrl",
				DataType: []string{schema.DataTypeString.String()},
			},
			{
				Name:     "price",
				DataType: []string{schema.DataTypeNumber.String()},
			},
		},

		// Multi-vector configuration - the key feature of RFC 0012
		VectorConfig: map[string]models.VectorConfig{
			// Text embeddings for product descriptions
			"text": {
				Vectorizer: map[string]interface{}{
					"text2vec-openai": map[string]interface{}{
						"model":              "text-embedding-3-small",
						"dimensions":         1536,
						"sourceProperties":   []string{"title", "description"},
						"vectorizeClassName": false,
					},
				},
				VectorIndexType: "hnsw",
				VectorIndexConfig: map[string]interface{}{
					"ef":              100,
					"efConstruction":  128,
					"maxConnections":  32,
					"cleanupIntervalSeconds": 300,
				},
			},

			// Image embeddings for product photos
			"image": {
				Vectorizer: map[string]interface{}{
					"img2vec-neural": map[string]interface{}{
						"imageFields": []string{"imageUrl"},
					},
				},
				VectorIndexType: "hnsw",
				VectorIndexConfig: map[string]interface{}{
					"ef":              100,
					"efConstruction":  128,
					"maxConnections":  32,
				},
			},

			// Custom embeddings - bring your own vectors
			"custom": {
				Vectorizer: map[string]interface{}{
					"none": map[string]interface{}{},
				},
				VectorIndexType: "flat", // Use flat index for small datasets
			},
		},

		// Additional configurations
		ReplicationConfig: &models.ReplicationConfig{
			Factor: 3,
		},
		MultiTenancyConfig: &models.MultiTenancyConfig{
			Enabled: false,
		},
	}

	err := client.Schema().ClassCreator().WithClass(class).Do(ctx)
	if err != nil {
		log.Printf("Note: Class may already exist: %v", err)
		return
	}

	fmt.Println("✅ E-commerce schema created successfully")
}

// insertProducts demonstrates inserting objects with multiple vectors
func insertProducts(ctx context.Context, client *wvt.Client) {
	fmt.Println("\nInserting products with multiple vector representations...")

	products := []map[string]interface{}{
		{
			"title":       "Leather Jacket",
			"description": "Premium brown leather jacket with vintage design",
			"imageUrl":    "https://example.com/leather-jacket.jpg",
			"price":       299.99,
		},
		{
			"title":       "Running Shoes",
			"description": "Lightweight athletic shoes perfect for marathon training",
			"imageUrl":    "https://example.com/running-shoes.jpg",
			"price":       129.99,
		},
		{
			"title":       "Wireless Headphones",
			"description": "Noise-cancelling bluetooth headphones with 30-hour battery",
			"imageUrl":    "https://example.com/headphones.jpg",
			"price":       249.99,
		},
	}

	for _, product := range products {
		// The vectorizers will automatically generate vectors for "text" and "image"
		// based on the configured sourceProperties and imageFields
		obj, err := client.Data().Creator().
			WithClassName("Product").
			WithProperties(product).
			Do(ctx)

		if err != nil {
			log.Printf("Failed to insert product: %v", err)
			continue
		}

		fmt.Printf("✅ Inserted product: %s (ID: %s) with %d vectors\n",
			product["title"], obj.Object.ID, len(obj.Object.Vectors))
	}
}

// searchProducts demonstrates multi-vector search with fusion
func searchProducts(ctx context.Context, client *wvt.Client) {
	fmt.Println("\nSearching products with multi-vector fusion...")

	// Example 1: Hybrid search across text and image vectors
	fmt.Println("\n--- Example 1: Hybrid Search with Multiple Vectors ---")

	resp, err := client.GraphQL().Get().
		WithClassName("Product").
		WithHybrid(client.GraphQL().
			HybridArgumentBuilder().
			WithQuery("comfortable leather jacket").
			WithAlpha(0.7). // 0.7 = more vector search, 0.3 = more keyword search
			WithTargetVectors("text", "image"). // Search across both vectors
			WithFusionType(graphql.RelativeScore)). // Use relative score fusion
		WithFields(
			graphql.Field{Name: "title"},
			graphql.Field{Name: "description"},
			graphql.Field{Name: "price"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "score"},
					{Name: "explainScore"},
				},
			},
		).
		WithLimit(3).
		Do(ctx)

	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("Found %d products\n", len(resp.Data))
	}

	// Example 2: Vector search on specific named vector only
	fmt.Println("\n--- Example 2: Search Using Only Text Vector ---")

	resp2, err := client.GraphQL().Get().
		WithClassName("Product").
		WithNearText(client.GraphQL().
			NearTextArgumentBuilder().
			WithConcepts([]string{"athletic", "sports"}).
			WithTargetVectors("text")). // Only search the text vector
		WithFields(
			graphql.Field{Name: "title"},
			graphql.Field{Name: "price"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "distance"},
					{Name: "certainty"},
				},
			},
		).
		WithLimit(3).
		Do(ctx)

	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("Found %d products\n", len(resp2.Data))
	}

	// Example 3: Hybrid search with Ranked Fusion (RRF)
	fmt.Println("\n--- Example 3: Hybrid Search with Ranked Fusion (RRF) ---")

	resp3, err := client.GraphQL().Get().
		WithClassName("Product").
		WithHybrid(client.GraphQL().
			HybridArgumentBuilder().
			WithQuery("premium quality").
			WithAlpha(0.5).
			WithTargetVectors("text").
			WithFusionType(graphql.Ranked)). // Use Reciprocal Rank Fusion
		WithFields(
			graphql.Field{Name: "title"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "score"},
					{Name: "explainScore"},
				},
			},
		).
		WithLimit(5).
		Do(ctx)

	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("Found %d products using RRF\n", len(resp3.Data))
	}
}

// createHealthcareSchema demonstrates multi-vector setup for healthcare
func createHealthcareSchema(ctx context.Context, client *wvt.Client) {
	fmt.Println("\n\nCreating healthcare schema with specialized vectors...")

	class := &models.Class{
		Class:       "MedicalRecord",
		Description: "Medical records with multiple specialized embeddings",
		Properties: []*models.Property{
			{
				Name:     "patientNotes",
				DataType: []string{schema.DataTypeText.String()},
			},
			{
				Name:     "diagnosis",
				DataType: []string{schema.DataTypeText.String()},
			},
			{
				Name:     "imagingUrl",
				DataType: []string{schema.DataTypeString.String()},
			},
		},

		VectorConfig: map[string]models.VectorConfig{
			// Clinical BERT for medical notes
			"clinical": {
				Vectorizer: map[string]interface{}{
					"text2vec-transformers": map[string]interface{}{
						"model":            "emilyalsentzer/Bio_ClinicalBERT",
						"sourceProperties": []string{"patientNotes", "diagnosis"},
					},
				},
				VectorIndexType: "hnsw",
			},

			// Medical image embeddings (e.g., ResNet)
			"imaging": {
				Vectorizer: map[string]interface{}{
					"img2vec-neural": map[string]interface{}{
						"imageFields": []string{"imagingUrl"},
					},
				},
				VectorIndexType: "hnsw",
				VectorIndexConfig: map[string]interface{}{
					"ef":             200, // Higher ef for better recall in medical use case
					"efConstruction": 256,
				},
			},

			// Specialized embeddings for lab results
			"labs": {
				Vectorizer: map[string]interface{}{
					"none": map[string]interface{}{}, // BYOV - bring your own vectors
				},
				VectorIndexType: "flat",
			},
		},
	}

	err := client.Schema().ClassCreator().WithClass(class).Do(ctx)
	if err != nil {
		log.Printf("Note: Class may already exist: %v", err)
		return
	}

	fmt.Println("✅ Healthcare schema created successfully")
	fmt.Println("\nKey Features Demonstrated:")
	fmt.Println("- Multiple named vectors per class (text, image, custom)")
	fmt.Println("- Heterogeneous dimensions and models")
	fmt.Println("- Independent vector index configurations")
	fmt.Println("- Hybrid search with fusion strategies")
	fmt.Println("- Cross-modal retrieval (text + image)")
	fmt.Println("- Backward compatibility with legacy single-vector configs")
}

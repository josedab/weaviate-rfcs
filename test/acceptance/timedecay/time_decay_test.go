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

package timedecay

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/entities/dto"
	"github.com/weaviate/weaviate/entities/filters"
	"github.com/weaviate/weaviate/entities/models"
	"github.com/weaviate/weaviate/entities/schema"
	"github.com/weaviate/weaviate/entities/searchparams"
	"github.com/weaviate/weaviate/test/helper"
)

func TestTimeDecayExponential(t *testing.T) {
	className := "Article"
	defer func() {
		helper.DeleteClass(t, className)
	}()

	// Create a class with a date property
	class := &models.Class{
		Class: className,
		Properties: []*models.Property{
			{
				Name:     "title",
				DataType: []string{string(schema.DataTypeText)},
			},
			{
				Name:     "content",
				DataType: []string{string(schema.DataTypeText)},
			},
			{
				Name:     "publishedAt",
				DataType: []string{string(schema.DataTypeDate)},
			},
		},
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
	}

	helper.CreateClass(t, class)

	// Insert test objects with different timestamps
	now := time.Now()
	objects := []*models.Object{
		{
			Class: className,
			Properties: map[string]interface{}{
				"title":       "Recent AI News",
				"content":     "Latest developments in AI",
				"publishedAt": now.Add(-1 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.1, 0.2, 0.3},
		},
		{
			Class: className,
			Properties: map[string]interface{}{
				"title":       "Week Old AI News",
				"content":     "AI developments from last week",
				"publishedAt": now.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.11, 0.21, 0.31}, // Very similar vector
		},
		{
			Class: className,
			Properties: map[string]interface{}{
				"title":       "Month Old AI News",
				"content":     "AI developments from last month",
				"publishedAt": now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.09, 0.19, 0.29}, // Very similar vector
		},
	}

	for _, obj := range objects {
		helper.CreateObject(t, obj)
	}

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Test vector search WITHOUT time decay
	t.Run("without time decay", func(t *testing.T) {
		params := dto.GetParams{
			ClassName: className,
			NearVector: &searchparams.NearVector{
				Vectors: []models.Vector{[]float32{0.1, 0.2, 0.3}},
			},
			Pagination: &filters.Pagination{
				Limit: 10,
			},
		}

		results, err := helper.QueryClass(t, params)
		require.NoError(t, err)
		require.Len(t, results, 3)

		// Without time decay, the order should be based purely on vector similarity
		// All vectors are very similar, so order might vary
	})

	// Test vector search WITH exponential time decay
	t.Run("with exponential time decay", func(t *testing.T) {
		params := dto.GetParams{
			ClassName: className,
			NearVector: &searchparams.NearVector{
				Vectors: []models.Vector{[]float32{0.1, 0.2, 0.3}},
			},
			TimeDecay: &searchparams.TimeDecay{
				Property:      "publishedAt",
				HalfLife:      "7d",
				DecayFunction: "EXPONENTIAL",
			},
			Pagination: &filters.Pagination{
				Limit: 10,
			},
		}

		results, err := helper.QueryClass(t, params)
		require.NoError(t, err)
		require.Len(t, results, 3)

		// With time decay, recent articles should be ranked higher
		firstTitle := results[0].Properties.(map[string]interface{})["title"].(string)
		assert.Contains(t, firstTitle, "Recent", "Most recent article should be ranked first")
	})
}

func TestTimeDecayLinear(t *testing.T) {
	className := "Product"
	defer func() {
		helper.DeleteClass(t, className)
	}()

	class := &models.Class{
		Class: className,
		Properties: []*models.Property{
			{
				Name:     "name",
				DataType: []string{string(schema.DataTypeText)},
			},
			{
				Name:     "createdAt",
				DataType: []string{string(schema.DataTypeDate)},
			},
		},
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
	}

	helper.CreateClass(t, class)

	now := time.Now()
	objects := []*models.Object{
		{
			Class: className,
			Properties: map[string]interface{}{
				"name":      "New Headphones",
				"createdAt": now.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.5, 0.5, 0.5},
		},
		{
			Class: className,
			Properties: map[string]interface{}{
				"name":      "Old Headphones",
				"createdAt": now.Add(-35 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.51, 0.51, 0.51},
		},
	}

	for _, obj := range objects {
		helper.CreateObject(t, obj)
	}

	time.Sleep(1 * time.Second)

	t.Run("with linear decay", func(t *testing.T) {
		params := dto.GetParams{
			ClassName: className,
			NearVector: &searchparams.NearVector{
				Vectors: []models.Vector{[]float32{0.5, 0.5, 0.5}},
			},
			TimeDecay: &searchparams.TimeDecay{
				Property:      "createdAt",
				MaxAge:        "30d",
				DecayFunction: "LINEAR",
			},
			Pagination: &filters.Pagination{
				Limit: 10,
			},
		}

		results, err := helper.QueryClass(t, params)
		require.NoError(t, err)
		require.Len(t, results, 2)

		// New product should be ranked higher due to time decay
		firstTitle := results[0].Properties.(map[string]interface{})["name"].(string)
		assert.Contains(t, firstTitle, "New", "Newer product should be ranked first")
	})
}

func TestTimeDecayStep(t *testing.T) {
	className := "SocialPost"
	defer func() {
		helper.DeleteClass(t, className)
	}()

	class := &models.Class{
		Class: className,
		Properties: []*models.Property{
			{
				Name:     "content",
				DataType: []string{string(schema.DataTypeText)},
			},
			{
				Name:     "postedAt",
				DataType: []string{string(schema.DataTypeDate)},
			},
		},
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
	}

	helper.CreateClass(t, class)

	now := time.Now()
	objects := []*models.Object{
		{
			Class: className,
			Properties: map[string]interface{}{
				"content":  "Very recent post",
				"postedAt": now.Add(-2 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.7, 0.7, 0.7},
		},
		{
			Class: className,
			Properties: map[string]interface{}{
				"content":  "Recent post",
				"postedAt": now.Add(-15 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.71, 0.71, 0.71},
		},
		{
			Class: className,
			Properties: map[string]interface{}{
				"content":  "Old post",
				"postedAt": now.Add(-100 * 24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.69, 0.69, 0.69},
		},
	}

	for _, obj := range objects {
		helper.CreateObject(t, obj)
	}

	time.Sleep(1 * time.Second)

	t.Run("with step decay", func(t *testing.T) {
		params := dto.GetParams{
			ClassName: className,
			NearVector: &searchparams.NearVector{
				Vectors: []models.Vector{[]float32{0.7, 0.7, 0.7}},
			},
			TimeDecay: &searchparams.TimeDecay{
				Property:      "postedAt",
				DecayFunction: "STEP",
				StepThresholds: []searchparams.TimeDecayStepThreshold{
					{MaxAge: "7d", Weight: 1.0},
					{MaxAge: "30d", Weight: 0.5},
					{MaxAge: "90d", Weight: 0.2},
				},
			},
			Pagination: &filters.Pagination{
				Limit: 10,
			},
		}

		results, err := helper.QueryClass(t, params)
		require.NoError(t, err)
		require.Len(t, results, 3)

		// Very recent post should be ranked first
		firstContent := results[0].Properties.(map[string]interface{})["content"].(string)
		assert.Contains(t, firstContent, "Very recent", "Most recent post should be ranked first")

		// Old post should be ranked last
		lastContent := results[2].Properties.(map[string]interface{})["content"].(string)
		assert.Contains(t, lastContent, "Old", "Old post should be ranked last")
	})
}

func TestTimeDecayWithGraphQL(t *testing.T) {
	className := "NewsArticle"
	defer func() {
		helper.DeleteClass(t, className)
	}()

	class := &models.Class{
		Class: className,
		Properties: []*models.Property{
			{
				Name:     "title",
				DataType: []string{string(schema.DataTypeText)},
			},
			{
				Name:     "publishedAt",
				DataType: []string{string(schema.DataTypeDate)},
			},
		},
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
	}

	helper.CreateClass(t, class)

	now := time.Now()
	objects := []*models.Object{
		{
			Class: className,
			Properties: map[string]interface{}{
				"title":       "Today's News",
				"publishedAt": now.Format(time.RFC3339),
			},
			Vector: []float32{0.3, 0.3, 0.3},
		},
		{
			Class: className,
			Properties: map[string]interface{}{
				"title":       "Yesterday's News",
				"publishedAt": now.Add(-24 * time.Hour).Format(time.RFC3339),
			},
			Vector: []float32{0.31, 0.31, 0.31},
		},
	}

	for _, obj := range objects {
		helper.CreateObject(t, obj)
	}

	time.Sleep(1 * time.Second)

	// Test with GraphQL query
	query := `{
		Get {
			NewsArticle(
				nearVector: {vector: [0.3, 0.3, 0.3]}
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
	}`

	result := helper.QueryGraphQL(t, query)
	require.NotNil(t, result)

	articles := result["Get"].(map[string]interface{})["NewsArticle"].([]interface{})
	require.Len(t, articles, 2)

	// Today's news should be first
	firstArticle := articles[0].(map[string]interface{})
	assert.Contains(t, firstArticle["title"].(string), "Today", "Today's article should be ranked first")
}

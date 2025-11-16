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

package evolution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/entities/models"
	"github.com/weaviate/weaviate/entities/schema/evolution"
)

func TestCompatibilityValidator_AddProperty(t *testing.T) {
	validator := NewCompatibilityValidator(evolution.DefaultCompatibilityConfig())

	oldSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	newSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
					{
						Name:     "author",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	result, err := validator.Validate(oldSchema, newSchema)
	require.NoError(t, err)
	assert.True(t, result.Compatible)
	assert.Equal(t, evolution.BackwardCompatible, result.Level)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, "Article.author", result.Issues[0].Path)
}

func TestCompatibilityValidator_RemoveProperty(t *testing.T) {
	validator := NewCompatibilityValidator(evolution.DefaultCompatibilityConfig())

	oldSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
					{
						Name:     "author",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	newSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	result, err := validator.Validate(oldSchema, newSchema)
	require.NoError(t, err)
	assert.False(t, result.Compatible) // Breaking change
	assert.Equal(t, evolution.Breaking, result.Level)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, evolution.Breaking, result.Issues[0].Level)
	assert.Equal(t, "Article.author", result.Issues[0].Path)
}

func TestCompatibilityValidator_ChangePropertyType(t *testing.T) {
	validator := NewCompatibilityValidator(evolution.DefaultCompatibilityConfig())

	oldSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "count",
						DataType: []string{"int"},
					},
				},
			},
		},
	}

	newSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "count",
						DataType: []string{"number"},
					},
				},
			},
		},
	}

	result, err := validator.Validate(oldSchema, newSchema)
	require.NoError(t, err)
	assert.False(t, result.Compatible)
	assert.Equal(t, evolution.Breaking, result.Level)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, "Article.count", result.Issues[0].Path)
}

func TestCompatibilityValidator_AddClass(t *testing.T) {
	validator := NewCompatibilityValidator(evolution.DefaultCompatibilityConfig())

	oldSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	newSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
				},
			},
			{
				Class: "Author",
				Properties: []*models.Property{
					{
						Name:     "name",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	result, err := validator.Validate(oldSchema, newSchema)
	require.NoError(t, err)
	assert.True(t, result.Compatible)
	// Adding a new class is backward compatible
	// (old code doesn't know about it, new code can use it)
	assert.Equal(t, evolution.BackwardCompatible, result.Level)
}

func TestCompatibilityValidator_RemoveClass(t *testing.T) {
	validator := NewCompatibilityValidator(evolution.DefaultCompatibilityConfig())

	oldSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
				},
			},
			{
				Class: "Author",
				Properties: []*models.Property{
					{
						Name:     "name",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	newSchema := &models.Schema{
		Classes: []*models.Class{
			{
				Class: "Article",
				Properties: []*models.Property{
					{
						Name:     "title",
						DataType: []string{"text"},
					},
				},
			},
		},
	}

	result, err := validator.Validate(oldSchema, newSchema)
	require.NoError(t, err)
	assert.False(t, result.Compatible)
	assert.Equal(t, evolution.Breaking, result.Level)
	assert.Len(t, result.Issues, 1)
	assert.Equal(t, "Author", result.Issues[0].Path)
}

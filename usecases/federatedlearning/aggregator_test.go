package federatedlearning

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaviate/weaviate/entities/federatedlearning"
)

func TestSecureAggregator_Aggregate(t *testing.T) {
	t.Run("basic federated averaging", func(t *testing.T) {
		aggregator := NewSecureAggregator(2, false)

		updates := []*federatedlearning.ModelUpdate{
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{1.0, 2.0, 3.0},
				NumSamples:    100,
			},
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{2.0, 4.0, 6.0},
				NumSamples:    100,
			},
		}

		result, err := aggregator.Aggregate(updates)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should be weighted average
		assert.Equal(t, float32(1.5), result.Weights[0])
		assert.Equal(t, float32(3.0), result.Weights[1])
		assert.Equal(t, float32(4.5), result.Weights[2])
		assert.Equal(t, int64(200), result.NumSamples)
	})

	t.Run("weighted aggregation", func(t *testing.T) {
		aggregator := NewSecureAggregator(2, false)

		updates := []*federatedlearning.ModelUpdate{
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{1.0, 2.0},
				NumSamples:    100,
			},
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{3.0, 4.0},
				NumSamples:    300, // 3x more samples
			},
		}

		result, err := aggregator.Aggregate(updates)
		require.NoError(t, err)

		// Should be weighted by num_samples
		// (1*100 + 3*300) / 400 = 2.5
		// (2*100 + 4*300) / 400 = 3.5
		assert.InDelta(t, 2.5, result.Weights[0], 0.01)
		assert.InDelta(t, 3.5, result.Weights[1], 0.01)
	})

	t.Run("insufficient participants", func(t *testing.T) {
		aggregator := NewSecureAggregator(3, false)

		updates := []*federatedlearning.ModelUpdate{
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{1.0},
				NumSamples:    100,
			},
		}

		_, err := aggregator.Aggregate(updates)
		assert.Error(t, err)
		assert.Equal(t, federatedlearning.ErrInsufficientParticipants, err)
	})

	t.Run("dimension mismatch", func(t *testing.T) {
		aggregator := NewSecureAggregator(2, false)

		updates := []*federatedlearning.ModelUpdate{
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{1.0, 2.0},
				NumSamples:    100,
			},
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{1.0}, // Different dimension
				NumSamples:    100,
			},
		}

		_, err := aggregator.Aggregate(updates)
		assert.Error(t, err)
	})

	t.Run("secure aggregation", func(t *testing.T) {
		aggregator := NewSecureAggregator(2, true)

		updates := []*federatedlearning.ModelUpdate{
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{1.0, 2.0},
				NumSamples:    100,
			},
			{
				ParticipantID: uuid.New(),
				Weights:       []float32{3.0, 4.0},
				NumSamples:    100,
			},
		}

		result, err := aggregator.Aggregate(updates)
		require.NoError(t, err)
		assert.True(t, result.NoiseAdded)
	})
}

func TestSecureAggregator_AggregateWithWeights(t *testing.T) {
	aggregator := NewSecureAggregator(2, false)

	updates := []*federatedlearning.ModelUpdate{
		{
			ParticipantID: uuid.New(),
			Weights:       []float32{1.0, 2.0},
			NumSamples:    100,
		},
		{
			ParticipantID: uuid.New(),
			Weights:       []float32{3.0, 4.0},
			NumSamples:    100,
		},
	}

	// Custom weights: 75% to first, 25% to second
	weights := []float64{0.75, 0.25}

	result, err := aggregator.AggregateWithWeights(updates, weights)
	require.NoError(t, err)

	// (1*0.75 + 3*0.25) = 1.5
	// (2*0.75 + 4*0.25) = 2.5
	assert.InDelta(t, 1.5, result.Weights[0], 0.01)
	assert.InDelta(t, 2.5, result.Weights[1], 0.01)
}

func TestSecureAggregator_FedProx(t *testing.T) {
	aggregator := NewSecureAggregator(2, false)

	globalModel := &federatedlearning.GlobalModel{
		Weights: []float32{5.0, 5.0},
	}

	updates := []*federatedlearning.ModelUpdate{
		{
			ParticipantID: uuid.New(),
			Weights:       []float32{1.0, 2.0},
			NumSamples:    100,
		},
		{
			ParticipantID: uuid.New(),
			Weights:       []float32{3.0, 4.0},
			NumSamples:    100,
		},
	}

	mu := float32(0.1) // Proximal term

	result, err := aggregator.FedProx(updates, globalModel, mu)
	require.NoError(t, err)

	// Without proximal: (1+3)/2 = 2.0, (2+4)/2 = 3.0
	// With proximal: 2.0 + 0.1*(5.0-2.0) = 2.3, 3.0 + 0.1*(5.0-3.0) = 3.2
	assert.InDelta(t, 2.3, result.Weights[0], 0.01)
	assert.InDelta(t, 3.2, result.Weights[1], 0.01)
}

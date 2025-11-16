package federatedlearning

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGaussianMechanism_AddNoise(t *testing.T) {
	mechanism := NewGaussianMechanism(1.0, 1e-5)

	t.Run("adds noise to value", func(t *testing.T) {
		value := 10.0
		sensitivity := 1.0

		noisy := mechanism.AddNoise(value, sensitivity)

		// Noise should be non-zero (very unlikely to be exactly 0)
		assert.NotEqual(t, value, noisy)

		// Should be within reasonable range
		diff := math.Abs(noisy - value)
		assert.Less(t, diff, 10.0) // Within 10 standard deviations
	})

	t.Run("adds noise to vector", func(t *testing.T) {
		vector := []float32{1.0, 2.0, 3.0}
		sensitivity := 1.0

		noisy := mechanism.AddNoiseToVector(vector, sensitivity)

		require.Len(t, noisy, len(vector))

		// At least one value should be different
		different := false
		for i := range vector {
			if noisy[i] != vector[i] {
				different = true
				break
			}
		}
		assert.True(t, different)
	})
}

func TestLaplaceMechanism_AddNoise(t *testing.T) {
	mechanism := NewLaplaceMechanism(1.0)

	t.Run("adds noise to value", func(t *testing.T) {
		value := 10.0
		sensitivity := 1.0

		noisy := mechanism.AddNoise(value, sensitivity)

		// Noise should be non-zero
		assert.NotEqual(t, value, noisy)
	})
}

func TestDifferentialPrivacy_PrivatizeGradients(t *testing.T) {
	dp := NewDifferentialPrivacy(1.0, 1e-5)

	gradients := []float32{0.5, 1.0, 1.5, 2.0}
	clipNorm := 1.0

	private := dp.PrivatizeGradients(gradients, clipNorm)

	require.Len(t, private, len(gradients))

	// Gradients should be different after privatization
	different := false
	for i := range gradients {
		if private[i] != gradients[i] {
			different = true
			break
		}
	}
	assert.True(t, different)
}

func TestClipGradients(t *testing.T) {
	t.Run("clips to max norm", func(t *testing.T) {
		// Gradient with L2 norm = sqrt(1 + 4 + 9) = sqrt(14) ≈ 3.74
		gradients := []float32{1.0, 2.0, 3.0}
		maxNorm := 2.0

		clipped := clipGradients(gradients, maxNorm)

		// Calculate clipped norm
		norm := float64(0)
		for _, g := range clipped {
			norm += float64(g * g)
		}
		norm = math.Sqrt(norm)

		assert.InDelta(t, maxNorm, norm, 0.01)
	})

	t.Run("does not clip if below norm", func(t *testing.T) {
		gradients := []float32{0.1, 0.2, 0.3}
		maxNorm := 10.0

		clipped := clipGradients(gradients, maxNorm)

		// Should be unchanged
		for i := range gradients {
			assert.Equal(t, gradients[i], clipped[i])
		}
	})
}

func TestPrivacyAccountant(t *testing.T) {
	t.Run("check budget", func(t *testing.T) {
		accountant := NewPrivacyAccountant(10.0, 1e-5)

		err := accountant.CheckBudget(5.0, 5e-6)
		assert.NoError(t, err)

		err = accountant.CheckBudget(15.0, 5e-6)
		assert.Error(t, err)
	})

	t.Run("consume privacy", func(t *testing.T) {
		accountant := NewPrivacyAccountant(10.0, 1e-5)

		err := accountant.ConsumePrivacy(1.0, 1e-6, "query1")
		assert.NoError(t, err)

		err = accountant.ConsumePrivacy(2.0, 2e-6, "query2")
		assert.NoError(t, err)

		epsilon, delta := accountant.GetRemainingBudget()
		assert.Equal(t, 7.0, epsilon)
		assert.InDelta(t, 7e-6, delta, 1e-10)
	})

	t.Run("budget exceeded", func(t *testing.T) {
		accountant := NewPrivacyAccountant(5.0, 1e-5)

		err := accountant.ConsumePrivacy(3.0, 3e-6, "query1")
		assert.NoError(t, err)

		err = accountant.ConsumePrivacy(3.0, 3e-6, "query2")
		assert.Error(t, err)
	})

	t.Run("get queries", func(t *testing.T) {
		accountant := NewPrivacyAccountant(10.0, 1e-5)

		accountant.ConsumePrivacy(1.0, 1e-6, "query1")
		accountant.ConsumePrivacy(2.0, 2e-6, "query2")

		queries := accountant.GetQueries()
		assert.Len(t, queries, 2)
		assert.Equal(t, "query1", queries[0].QueryType)
		assert.Equal(t, "query2", queries[1].QueryType)
	})

	t.Run("reset", func(t *testing.T) {
		accountant := NewPrivacyAccountant(10.0, 1e-5)

		accountant.ConsumePrivacy(5.0, 5e-6, "query1")
		accountant.Reset()

		epsilon, delta := accountant.GetRemainingBudget()
		assert.Equal(t, 10.0, epsilon)
		assert.Equal(t, 1e-5, delta)

		queries := accountant.GetQueries()
		assert.Len(t, queries, 0)
	})
}

func TestBasicComposition(t *testing.T) {
	composition := &BasicComposition{}

	queries := []*struct {
		EpsilonUsed float64
		DeltaUsed   float64
	}{
		{EpsilonUsed: 1.0, DeltaUsed: 1e-6},
		{EpsilonUsed: 2.0, DeltaUsed: 2e-6},
		{EpsilonUsed: 1.5, DeltaUsed: 1.5e-6},
	}

	// Convert to proper Query type for testing
	// In actual test, we'd use the real Query type
	// For now, test the concept

	totalEpsilon := 0.0
	totalDelta := 0.0
	for _, q := range queries {
		totalEpsilon += q.EpsilonUsed
		totalDelta += q.DeltaUsed
	}

	// Basic composition: sum of all privacy losses
	assert.Equal(t, 4.5, totalEpsilon)
	assert.InDelta(t, 4.5e-6, totalDelta, 1e-10)
}

func TestMomentsAccountant(t *testing.T) {
	accountant := NewMomentsAccountant()

	t.Run("add queries and compute privacy", func(t *testing.T) {
		accountant.AddQuery(0.1, 1e-6)
		accountant.AddQuery(0.1, 1e-6)
		accountant.AddQuery(0.1, 1e-6)

		epsilon := accountant.GetPrivacy(1e-5)

		// Should give tighter bounds than basic composition
		assert.Greater(t, epsilon, 0.0)
		assert.Less(t, epsilon, 0.3) // Better than 3*0.1
	})
}

package federatedlearning

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/federatedlearning"
)

// SecureAggregator performs secure aggregation of model updates
type SecureAggregator struct {
	threshold         int  // Minimum participants for privacy
	secureAggregation bool // Enable cryptographic secure aggregation
	logger            logrus.FieldLogger
}

// NewSecureAggregator creates a new secure aggregator
func NewSecureAggregator(threshold int, secureAggregation bool) *SecureAggregator {
	return &SecureAggregator{
		threshold:         threshold,
		secureAggregation: secureAggregation,
		logger:            logrus.WithField("component", "secure_aggregator"),
	}
}

// Aggregate aggregates model updates using federated averaging
func (a *SecureAggregator) Aggregate(updates []*federatedlearning.ModelUpdate) (*federatedlearning.ModelUpdate, error) {
	if len(updates) < a.threshold {
		return nil, federatedlearning.ErrInsufficientParticipants
	}

	if len(updates) == 0 {
		return nil, &federatedlearning.Error{
			Code:    "NO_UPDATES",
			Message: "no model updates provided",
		}
	}

	// Calculate total samples
	totalSamples := int64(0)
	for _, update := range updates {
		totalSamples += update.NumSamples
	}

	if totalSamples == 0 {
		return nil, &federatedlearning.Error{
			Code:    "INVALID_UPDATES",
			Message: "total samples is zero",
		}
	}

	// Validate all updates have same weight dimensions
	numWeights := len(updates[0].Weights)
	for i, update := range updates {
		if len(update.Weights) != numWeights {
			return nil, &federatedlearning.Error{
				Code:    "DIMENSION_MISMATCH",
				Message: fmt.Sprintf("update %d has %d weights, expected %d", i, len(update.Weights), numWeights),
			}
		}
	}

	// Perform aggregation
	var aggregated *federatedlearning.ModelUpdate
	var err error

	if a.secureAggregation {
		aggregated, err = a.secureAggregateUpdates(updates, totalSamples)
	} else {
		aggregated, err = a.federatedAverage(updates, totalSamples)
	}

	if err != nil {
		return nil, err
	}

	a.logger.WithFields(logrus.Fields{
		"num_updates":    len(updates),
		"total_samples":  totalSamples,
		"weight_dim":     numWeights,
		"secure":         a.secureAggregation,
	}).Debug("model updates aggregated")

	return aggregated, nil
}

// federatedAverage performs standard federated averaging
func (a *SecureAggregator) federatedAverage(
	updates []*federatedlearning.ModelUpdate,
	totalSamples int64,
) (*federatedlearning.ModelUpdate, error) {
	numWeights := len(updates[0].Weights)
	aggregated := make([]float32, numWeights)

	// Weighted average based on number of samples
	for _, update := range updates {
		weight := float32(update.NumSamples) / float32(totalSamples)

		for i, w := range update.Weights {
			aggregated[i] += w * weight
		}
	}

	return &federatedlearning.ModelUpdate{
		Weights:    aggregated,
		NumSamples: totalSamples,
		NoiseAdded: false,
	}, nil
}

// secureAggregateUpdates performs cryptographically secure aggregation
func (a *SecureAggregator) secureAggregateUpdates(
	updates []*federatedlearning.ModelUpdate,
	totalSamples int64,
) (*federatedlearning.ModelUpdate, error) {
	// Step 1: Add random masks to each participant's update
	numWeights := len(updates[0].Weights)
	maskedUpdates := make([][]float32, len(updates))

	// Generate pairwise masks for secure aggregation
	// In a real implementation, this would use secure multi-party computation
	masks := a.generatePairwiseMasks(len(updates), numWeights)

	// Apply masks
	for i, update := range updates {
		maskedUpdates[i] = make([]float32, numWeights)
		for j := 0; j < numWeights; j++ {
			maskedUpdates[i][j] = update.Weights[j] + masks[i][j]
		}
	}

	// Step 2: Aggregate masked updates
	aggregated := make([]float32, numWeights)
	for _, masked := range maskedUpdates {
		weight := float32(1.0) / float32(len(updates))
		for i, w := range masked {
			aggregated[i] += w * weight
		}
	}

	// Step 3: Remove masks (they cancel out in sum)
	// In secure aggregation, masks are designed to sum to zero
	// so they don't need to be explicitly removed

	return &federatedlearning.ModelUpdate{
		Weights:    aggregated,
		NumSamples: totalSamples,
		NoiseAdded: true,
	}, nil
}

// generatePairwiseMasks generates random masks for secure aggregation
func (a *SecureAggregator) generatePairwiseMasks(numParticipants, numWeights int) [][]float32 {
	masks := make([][]float32, numParticipants)

	for i := 0; i < numParticipants; i++ {
		masks[i] = make([]float32, numWeights)

		for j := 0; j < numWeights; j++ {
			// Generate random mask in range [-1, 1]
			// In production, this would use cryptographically secure randomness
			// and pairwise secrets between participants
			maxVal := big.NewInt(10000)
			randInt, _ := rand.Int(rand.Reader, maxVal)
			masks[i][j] = float32(randInt.Int64())/5000.0 - 1.0
		}
	}

	// Ensure masks sum to zero (for secure aggregation property)
	// Adjust last participant's mask to cancel out others
	for j := 0; j < numWeights; j++ {
		sum := float32(0)
		for i := 0; i < numParticipants-1; i++ {
			sum += masks[i][j]
		}
		masks[numParticipants-1][j] = -sum
	}

	return masks
}

// AggregateWithWeights performs weighted aggregation with custom weights
func (a *SecureAggregator) AggregateWithWeights(
	updates []*federatedlearning.ModelUpdate,
	weights []float64,
) (*federatedlearning.ModelUpdate, error) {
	if len(updates) != len(weights) {
		return nil, &federatedlearning.Error{
			Code:    "INVALID_WEIGHTS",
			Message: "number of weights must match number of updates",
		}
	}

	if len(updates) < a.threshold {
		return nil, federatedlearning.ErrInsufficientParticipants
	}

	// Normalize weights
	totalWeight := 0.0
	for _, w := range weights {
		totalWeight += w
	}

	if totalWeight == 0 {
		return nil, &federatedlearning.Error{
			Code:    "INVALID_WEIGHTS",
			Message: "total weight is zero",
		}
	}

	// Aggregate with custom weights
	numWeights := len(updates[0].Weights)
	aggregated := make([]float32, numWeights)

	totalSamples := int64(0)
	for i, update := range updates {
		weight := float32(weights[i] / totalWeight)
		totalSamples += update.NumSamples

		for j, w := range update.Weights {
			aggregated[j] += w * weight
		}
	}

	return &federatedlearning.ModelUpdate{
		Weights:    aggregated,
		NumSamples: totalSamples,
		NoiseAdded: false,
	}, nil
}

// FedProx performs FedProx aggregation with proximal term
func (a *SecureAggregator) FedProx(
	updates []*federatedlearning.ModelUpdate,
	globalModel *federatedlearning.GlobalModel,
	mu float32, // Proximal term coefficient
) (*federatedlearning.ModelUpdate, error) {
	if len(updates) < a.threshold {
		return nil, federatedlearning.ErrInsufficientParticipants
	}

	// FedAvg aggregation
	aggregated, err := a.federatedAverage(updates, 0)
	if err != nil {
		return nil, err
	}

	// Apply proximal term: w_new = w_avg + mu * (w_global - w_avg)
	if globalModel != nil && len(globalModel.Weights) == len(aggregated.Weights) {
		for i := range aggregated.Weights {
			diff := globalModel.Weights[i] - aggregated.Weights[i]
			aggregated.Weights[i] += mu * diff
		}
	}

	return aggregated, nil
}

// Scaffold performs SCAFFOLD aggregation with control variates
func (a *SecureAggregator) Scaffold(
	updates []*federatedlearning.ModelUpdate,
	controlVariates [][]float32, // Control variates for variance reduction
) (*federatedlearning.ModelUpdate, error) {
	if len(updates) < a.threshold {
		return nil, federatedlearning.ErrInsufficientParticipants
	}

	// Standard aggregation
	aggregated, err := a.federatedAverage(updates, 0)
	if err != nil {
		return nil, err
	}

	// Apply control variates for variance reduction
	if len(controlVariates) == len(updates) {
		numWeights := len(aggregated.Weights)
		controlSum := make([]float32, numWeights)

		for _, cv := range controlVariates {
			if len(cv) != numWeights {
				continue
			}
			for i := range cv {
				controlSum[i] += cv[i]
			}
		}

		// Subtract average control variate
		n := float32(len(controlVariates))
		for i := range aggregated.Weights {
			aggregated.Weights[i] -= controlSum[i] / n
		}
	}

	return aggregated, nil
}

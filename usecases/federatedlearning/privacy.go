package federatedlearning

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/federatedlearning"
)

// DifferentialPrivacy manages differential privacy for federated learning
type DifferentialPrivacy struct {
	epsilon   float64
	delta     float64
	mechanism NoiseMechanism
	logger    logrus.FieldLogger
}

// NoiseMechanism defines the interface for adding noise
type NoiseMechanism interface {
	AddNoise(value float64, sensitivity float64) float64
	AddNoiseToVector(vector []float32, sensitivity float64) []float32
}

// GaussianMechanism implements Gaussian noise for (ε,δ)-differential privacy
type GaussianMechanism struct {
	epsilon float64
	delta   float64
	rng     *rand.Rand
	mu      sync.Mutex
}

// LaplaceMechanism implements Laplace noise for ε-differential privacy
type LaplaceMechanism struct {
	epsilon float64
	rng     *rand.Rand
	mu      sync.Mutex
}

// NewDifferentialPrivacy creates a new differential privacy manager
func NewDifferentialPrivacy(epsilon, delta float64) *DifferentialPrivacy {
	return &DifferentialPrivacy{
		epsilon:   epsilon,
		delta:     delta,
		mechanism: NewGaussianMechanism(epsilon, delta),
		logger:    logrus.WithField("component", "differential_privacy"),
	}
}

// NewGaussianMechanism creates a new Gaussian noise mechanism
func NewGaussianMechanism(epsilon, delta float64) *GaussianMechanism {
	return &GaussianMechanism{
		epsilon: epsilon,
		delta:   delta,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewLaplaceMechanism creates a new Laplace noise mechanism
func NewLaplaceMechanism(epsilon float64) *LaplaceMechanism {
	return &LaplaceMechanism{
		epsilon: epsilon,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddNoise adds Gaussian noise to a single value
func (g *GaussianMechanism) AddNoise(value float64, sensitivity float64) float64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Calculate noise scale: σ = sensitivity * sqrt(2*ln(1.25/δ)) / ε
	sigma := sensitivity * math.Sqrt(2*math.Log(1.25/g.delta)) / g.epsilon

	// Sample from Gaussian distribution N(0, σ²)
	noise := g.rng.NormFloat64() * sigma

	return value + noise
}

// AddNoiseToVector adds Gaussian noise to a vector
func (g *GaussianMechanism) AddNoiseToVector(vector []float32, sensitivity float64) []float32 {
	noisy := make([]float32, len(vector))

	for i, v := range vector {
		noisy[i] = float32(g.AddNoise(float64(v), sensitivity))
	}

	return noisy
}

// AddNoise adds Laplace noise to a single value
func (l *LaplaceMechanism) AddNoise(value float64, sensitivity float64) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Calculate scale: b = sensitivity / ε
	scale := sensitivity / l.epsilon

	// Sample from Laplace distribution
	u := l.rng.Float64() - 0.5
	noise := -scale * math.Copysign(1.0, u) * math.Log(1-2*math.Abs(u))

	return value + noise
}

// AddNoiseToVector adds Laplace noise to a vector
func (l *LaplaceMechanism) AddNoiseToVector(vector []float32, sensitivity float64) []float32 {
	noisy := make([]float32, len(vector))

	for i, v := range vector {
		noisy[i] = float32(l.AddNoise(float64(v), sensitivity))
	}

	return noisy
}

// PrivatizeGradients applies differential privacy to gradient updates
func (dp *DifferentialPrivacy) PrivatizeGradients(gradients []float32, clipNorm float64) []float32 {
	// Step 1: Clip gradients to bound sensitivity
	clipped := clipGradients(gradients, clipNorm)

	// Step 2: Add noise for privacy
	private := dp.mechanism.AddNoiseToVector(clipped, clipNorm)

	dp.logger.WithFields(logrus.Fields{
		"epsilon":   dp.epsilon,
		"delta":     dp.delta,
		"clip_norm": clipNorm,
		"dim":       len(gradients),
	}).Debug("gradients privatized")

	return private
}

// clipGradients clips gradients to a maximum L2 norm
func clipGradients(gradients []float32, maxNorm float64) []float32 {
	// Calculate L2 norm
	norm := float64(0)
	for _, g := range gradients {
		norm += float64(g * g)
	}
	norm = math.Sqrt(norm)

	// If norm exceeds max, scale down
	if norm > maxNorm {
		scale := float32(maxNorm / norm)
		clipped := make([]float32, len(gradients))
		for i, g := range gradients {
			clipped[i] = g * scale
		}
		return clipped
	}

	return gradients
}

// PrivacyAccountant tracks privacy budget consumption
type PrivacyAccountant struct {
	epsilonBudget float64
	deltaBudget   float64
	epsilonUsed   float64
	deltaUsed     float64
	queries       []*federatedlearning.Query
	mu            sync.RWMutex
	logger        logrus.FieldLogger
}

// NewPrivacyAccountant creates a new privacy accountant
func NewPrivacyAccountant(epsilonBudget, deltaBudget float64) *PrivacyAccountant {
	return &PrivacyAccountant{
		epsilonBudget: epsilonBudget,
		deltaBudget:   deltaBudget,
		epsilonUsed:   0,
		deltaUsed:     0,
		queries:       make([]*federatedlearning.Query, 0),
		logger:        logrus.WithField("component", "privacy_accountant"),
	}
}

// CheckBudget checks if sufficient privacy budget is available
func (a *PrivacyAccountant) CheckBudget(epsilon, delta float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.epsilonUsed+epsilon > a.epsilonBudget {
		return &federatedlearning.Error{
			Code:    "PRIVACY_BUDGET_EXCEEDED",
			Message: "epsilon privacy budget exceeded",
		}
	}

	if a.deltaUsed+delta > a.deltaBudget {
		return &federatedlearning.Error{
			Code:    "PRIVACY_BUDGET_EXCEEDED",
			Message: "delta privacy budget exceeded",
		}
	}

	return nil
}

// ConsumePrivacy consumes privacy budget for a query
func (a *PrivacyAccountant) ConsumePrivacy(epsilon, delta float64, queryType string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.epsilonUsed+epsilon > a.epsilonBudget {
		return federatedlearning.ErrPrivacyBudgetExceeded
	}

	if a.deltaUsed+delta > a.deltaBudget {
		return federatedlearning.ErrPrivacyBudgetExceeded
	}

	query := &federatedlearning.Query{
		ID:          uuid.New(),
		Timestamp:   time.Now(),
		QueryType:   queryType,
		EpsilonUsed: epsilon,
		DeltaUsed:   delta,
	}

	a.epsilonUsed += epsilon
	a.deltaUsed += delta
	a.queries = append(a.queries, query)

	a.logger.WithFields(logrus.Fields{
		"epsilon_used":   a.epsilonUsed,
		"epsilon_budget": a.epsilonBudget,
		"delta_used":     a.deltaUsed,
		"delta_budget":   a.deltaBudget,
		"query_type":     queryType,
	}).Debug("privacy budget consumed")

	return nil
}

// GetRemainingBudget returns the remaining privacy budget
func (a *PrivacyAccountant) GetRemainingBudget() (epsilon, delta float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.epsilonBudget - a.epsilonUsed, a.deltaBudget - a.deltaUsed
}

// GetQueries returns the history of privacy-consuming queries
func (a *PrivacyAccountant) GetQueries() []*federatedlearning.Query {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Return a copy
	queries := make([]*federatedlearning.Query, len(a.queries))
	copy(queries, a.queries)
	return queries
}

// Reset resets the privacy accountant
func (a *PrivacyAccountant) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.epsilonUsed = 0
	a.deltaUsed = 0
	a.queries = make([]*federatedlearning.Query, 0)

	a.logger.Info("privacy accountant reset")
}

// CompositionTheorem applies privacy composition for multiple queries
type CompositionTheorem interface {
	Compose(queries []*federatedlearning.Query) (epsilon, delta float64)
}

// BasicComposition implements basic composition theorem
type BasicComposition struct{}

// Compose applies basic composition: ε_total = Σε_i, δ_total = Σδ_i
func (c *BasicComposition) Compose(queries []*federatedlearning.Query) (epsilon, delta float64) {
	for _, q := range queries {
		epsilon += q.EpsilonUsed
		delta += q.DeltaUsed
	}
	return epsilon, delta
}

// AdvancedComposition implements advanced composition theorem
type AdvancedComposition struct{}

// Compose applies advanced composition for tighter bounds
func (c *AdvancedComposition) Compose(queries []*federatedlearning.Query) (epsilon, delta float64) {
	if len(queries) == 0 {
		return 0, 0
	}

	// For simplicity, use the same epsilon/delta for all queries
	// In practice, this would be more sophisticated
	k := float64(len(queries))
	eps := queries[0].EpsilonUsed
	del := queries[0].DeltaUsed

	// Advanced composition: ε' = sqrt(2k ln(1/δ')) * ε + k*ε*(e^ε - 1)
	// Simplified version
	epsilon = math.Sqrt(2*k*math.Log(1/del)) * eps
	delta = k * del

	return epsilon, delta
}

// MomentsAccountant implements moments accountant for tighter privacy bounds
type MomentsAccountant struct {
	moments []float64
	logger  logrus.FieldLogger
}

// NewMomentsAccountant creates a new moments accountant
func NewMomentsAccountant() *MomentsAccountant {
	return &MomentsAccountant{
		moments: make([]float64, 0),
		logger:  logrus.WithField("component", "moments_accountant"),
	}
}

// AddQuery adds a query to the moments accountant
func (m *MomentsAccountant) AddQuery(epsilon, delta float64) {
	// Simplified moments computation
	// In practice, this would track moment generating function
	moment := epsilon * epsilon
	m.moments = append(m.moments, moment)
}

// GetPrivacy computes total privacy from accumulated moments
func (m *MomentsAccountant) GetPrivacy(delta float64) float64 {
	if len(m.moments) == 0 {
		return 0
	}

	// Sum of moments
	totalMoment := float64(0)
	for _, moment := range m.moments {
		totalMoment += moment
	}

	// Compute epsilon from moments
	epsilon := math.Sqrt(2 * totalMoment * math.Log(1/delta))

	return epsilon
}

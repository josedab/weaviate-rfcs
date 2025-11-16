package federatedlearning

// Config represents federated learning configuration
type Config struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Coordinator settings
	Coordinator CoordinatorConfig `json:"coordinator" yaml:"coordinator"`

	// Training configuration
	Training TrainingConfig `json:"training" yaml:"training"`

	// Privacy parameters
	Privacy PrivacyConfig `json:"privacy" yaml:"privacy"`

	// Participants (for coordinator role)
	Participants []ParticipantConfig `json:"participants" yaml:"participants"`
}

// CoordinatorConfig represents coordinator settings
type CoordinatorConfig struct {
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	Role     string `json:"role" yaml:"role"` // coordinator | participant
}

// TrainingConfig represents training parameters
type TrainingConfig struct {
	Rounds       int     `json:"rounds" yaml:"rounds"`
	LocalEpochs  int     `json:"localEpochs" yaml:"localEpochs"`
	BatchSize    int     `json:"batchSize" yaml:"batchSize"`
	LearningRate float64 `json:"learningRate" yaml:"learningRate"`

	// Aggregation settings
	AggregationMethod string `json:"aggregationMethod" yaml:"aggregationMethod"` // fedavg | fedprox | scaffold
	MinParticipants   int    `json:"minParticipants" yaml:"minParticipants"`

	// Convergence criteria
	ConvergenceThreshold float64 `json:"convergenceThreshold" yaml:"convergenceThreshold"`
	MaxRounds            int     `json:"maxRounds" yaml:"maxRounds"`
}

// PrivacyConfig represents privacy parameters
type PrivacyConfig struct {
	DifferentialPrivacy bool    `json:"differentialPrivacy" yaml:"differentialPrivacy"`
	Epsilon             float64 `json:"epsilon" yaml:"epsilon"`
	Delta               float64 `json:"delta" yaml:"delta"`

	// Privacy budget
	Budget BudgetConfig `json:"budget" yaml:"budget"`

	// Secure aggregation
	SecureAggregation bool `json:"secureAggregation" yaml:"secureAggregation"`
	AggregationThreshold int `json:"aggregationThreshold" yaml:"aggregationThreshold"`
}

// BudgetConfig represents privacy budget configuration
type BudgetConfig struct {
	Total    float64 `json:"total" yaml:"total"`
	PerQuery float64 `json:"perQuery" yaml:"perQuery"`
}

// ParticipantConfig represents a participant's configuration
type ParticipantConfig struct {
	ID       string  `json:"id" yaml:"id"`
	Endpoint string  `json:"endpoint" yaml:"endpoint"`
	Weight   float64 `json:"weight" yaml:"weight"`
}

// Validate validates the federated learning configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Validate coordinator config
	if c.Coordinator.Endpoint == "" {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "coordinator endpoint is required",
		}
	}

	if c.Coordinator.Role != "coordinator" && c.Coordinator.Role != "participant" {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "coordinator role must be 'coordinator' or 'participant'",
		}
	}

	// Validate training config
	if c.Training.Rounds <= 0 {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "training rounds must be positive",
		}
	}

	if c.Training.LocalEpochs <= 0 {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "local epochs must be positive",
		}
	}

	if c.Training.BatchSize <= 0 {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "batch size must be positive",
		}
	}

	if c.Training.LearningRate <= 0 {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "learning rate must be positive",
		}
	}

	if c.Training.MinParticipants < 1 {
		return &Error{
			Code:    "INVALID_CONFIG",
			Message: "minimum participants must be at least 1",
		}
	}

	// Validate privacy config
	if c.Privacy.DifferentialPrivacy {
		if c.Privacy.Epsilon <= 0 {
			return &Error{
				Code:    "INVALID_CONFIG",
				Message: "epsilon must be positive when differential privacy is enabled",
			}
		}

		if c.Privacy.Delta < 0 || c.Privacy.Delta > 1 {
			return &Error{
				Code:    "INVALID_CONFIG",
				Message: "delta must be between 0 and 1",
			}
		}

		if c.Privacy.Budget.Total <= 0 {
			return &Error{
				Code:    "INVALID_CONFIG",
				Message: "total privacy budget must be positive",
			}
		}
	}

	// Validate participants (for coordinator role)
	if c.Coordinator.Role == "coordinator" {
		if len(c.Participants) < c.Training.MinParticipants {
			return &Error{
				Code:    "INVALID_CONFIG",
				Message: "number of configured participants is less than minimum required",
			}
		}

		for i, p := range c.Participants {
			if p.ID == "" {
				return &Error{
					Code:    "INVALID_CONFIG",
					Message: "participant ID is required",
				}
			}
			if p.Endpoint == "" {
				return &Error{
					Code:    "INVALID_CONFIG",
					Message: "participant endpoint is required",
				}
			}
			if p.Weight <= 0 {
				c.Participants[i].Weight = 1.0 // Default weight
			}
		}
	}

	return nil
}

// DefaultConfig returns the default federated learning configuration
func DefaultConfig() Config {
	return Config{
		Enabled: false,
		Coordinator: CoordinatorConfig{
			Endpoint: "",
			Role:     "participant",
		},
		Training: TrainingConfig{
			Rounds:               100,
			LocalEpochs:          5,
			BatchSize:            32,
			LearningRate:         0.001,
			AggregationMethod:    "fedavg",
			MinParticipants:      2,
			ConvergenceThreshold: 0.001,
			MaxRounds:            1000,
		},
		Privacy: PrivacyConfig{
			DifferentialPrivacy:  true,
			Epsilon:              1.0,
			Delta:                1e-5,
			Budget: BudgetConfig{
				Total:    10.0,
				PerQuery: 0.1,
			},
			SecureAggregation:    true,
			AggregationThreshold: 2,
		},
		Participants: []ParticipantConfig{},
	}
}

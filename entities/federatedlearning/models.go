package federatedlearning

import (
	"time"

	"github.com/google/uuid"
)

// Participant represents a federated learning participant
type Participant struct {
	ID           uuid.UUID
	Endpoint     string
	DataSize     int64
	LastUpdate   time.Time

	// Privacy budget
	EpsilonBudget float64
	EpsilonUsed   float64

	// Status
	Active bool
	Weight float64 // For weighted aggregation
}

// ModelUpdate represents a local model update from a participant
type ModelUpdate struct {
	ParticipantID  uuid.UUID
	Weights        []float32
	NumSamples     int64
	RoundNumber    int
	Timestamp      time.Time

	// Privacy information
	NoiseAdded     bool
	EpsilonUsed    float64
}

// GlobalModel represents the aggregated global model
type GlobalModel struct {
	ID            uuid.UUID
	Weights       []float32
	Version       int
	RoundNumber   int
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Metrics
	NumParticipants int
	TotalSamples    int64
	Accuracy        float64
}

// TrainingRound represents a single federated training round
type TrainingRound struct {
	RoundNumber        int
	StartTime          time.Time
	EndTime            time.Time
	Status             RoundStatus
	ParticipantsActive int
	ParticipantsTotal  int
	UpdatesReceived    int

	// Model info
	GlobalModelID      uuid.UUID

	// Metrics
	AggregationTime    time.Duration
	CommunicationTime  time.Duration
	ConvergenceMetric  float64

	// Errors
	Errors             []string
}

// RoundStatus represents the status of a training round
type RoundStatus string

const (
	RoundStatusPending    RoundStatus = "pending"
	RoundStatusInProgress RoundStatus = "in_progress"
	RoundStatusAggregating RoundStatus = "aggregating"
	RoundStatusCompleted  RoundStatus = "completed"
	RoundStatusFailed     RoundStatus = "failed"
)

// Query represents a privacy-preserving query for accounting
type Query struct {
	ID          uuid.UUID
	Timestamp   time.Time
	QueryType   string
	EpsilonUsed float64
	DeltaUsed   float64
}

// EncryptedScore represents a score computed on encrypted data
type EncryptedScore struct {
	ID    uuid.UUID
	Score []byte // Encrypted score
}

// Ciphertext represents encrypted data
type Ciphertext struct {
	Data []byte
}

// PublicKey represents a public key for homomorphic encryption
type PublicKey struct {
	N []byte // Modulus
	E int    // Public exponent
}

// PrivateKey represents a private key for homomorphic encryption
type PrivateKey struct {
	D []byte // Private exponent
	P []byte // Prime 1
	Q []byte // Prime 2
}

// Errors
var (
	ErrInsufficientParticipants = &Error{
		Code:    "INSUFFICIENT_PARTICIPANTS",
		Message: "insufficient number of participants for federated training",
	}

	ErrPrivacyBudgetExceeded = &Error{
		Code:    "PRIVACY_BUDGET_EXCEEDED",
		Message: "privacy budget has been exceeded",
	}

	ErrInvalidConfiguration = &Error{
		Code:    "INVALID_CONFIGURATION",
		Message: "invalid federated learning configuration",
	}

	ErrAggregationFailed = &Error{
		Code:    "AGGREGATION_FAILED",
		Message: "model aggregation failed",
	}
)

// Error represents a federated learning error
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

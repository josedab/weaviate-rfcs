package federatedlearning

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/federatedlearning"
)

// FederatedCoordinator manages federated learning training rounds
type FederatedCoordinator struct {
	participants []*federatedlearning.Participant
	aggregator   *SecureAggregator
	model        *federatedlearning.GlobalModel
	config       *federatedlearning.Config
	logger       logrus.FieldLogger

	// State management
	currentRound int
	mu           sync.RWMutex

	// Participant communication
	client ParticipantClient
}

// ParticipantClient defines the interface for communicating with participants
type ParticipantClient interface {
	SendModel(ctx context.Context, participant *federatedlearning.Participant, model *federatedlearning.GlobalModel) error
	ReceiveUpdate(ctx context.Context, participant *federatedlearning.Participant) (*federatedlearning.ModelUpdate, error)
	Ping(ctx context.Context, participant *federatedlearning.Participant) error
}

// NewFederatedCoordinator creates a new federated learning coordinator
func NewFederatedCoordinator(
	config *federatedlearning.Config,
	logger logrus.FieldLogger,
	client ParticipantClient,
) (*FederatedCoordinator, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Initialize participants
	participants := make([]*federatedlearning.Participant, 0, len(config.Participants))
	for _, p := range config.Participants {
		participantID, err := uuid.Parse(p.ID)
		if err != nil {
			// Generate a new UUID if parsing fails
			participantID = uuid.New()
		}

		participants = append(participants, &federatedlearning.Participant{
			ID:            participantID,
			Endpoint:      p.Endpoint,
			Weight:        p.Weight,
			Active:        true,
			EpsilonBudget: config.Privacy.Budget.Total,
			EpsilonUsed:   0,
		})
	}

	// Initialize aggregator
	aggregator := NewSecureAggregator(
		config.Training.MinParticipants,
		config.Privacy.SecureAggregation,
	)

	// Initialize global model
	model := &federatedlearning.GlobalModel{
		ID:          uuid.New(),
		Weights:     []float32{}, // Will be initialized on first training
		Version:     0,
		RoundNumber: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return &FederatedCoordinator{
		participants: participants,
		aggregator:   aggregator,
		model:        model,
		config:       config,
		logger:       logger,
		currentRound: 0,
		client:       client,
	}, nil
}

// TrainRound executes a single federated training round
func (c *FederatedCoordinator) TrainRound(ctx context.Context, round int) (*federatedlearning.TrainingRound, error) {
	c.mu.Lock()
	c.currentRound = round
	c.mu.Unlock()

	roundStart := time.Now()
	c.logger.WithFields(logrus.Fields{
		"round":        round,
		"participants": len(c.participants),
	}).Info("starting federated training round")

	trainingRound := &federatedlearning.TrainingRound{
		RoundNumber:        round,
		StartTime:          roundStart,
		Status:             federatedlearning.RoundStatusInProgress,
		ParticipantsTotal:  len(c.participants),
		ParticipantsActive: 0,
		UpdatesReceived:    0,
		GlobalModelID:      c.model.ID,
		Errors:             []string{},
	}

	// Step 1: Broadcast global model to participants
	updates := make([]*federatedlearning.ModelUpdate, 0, len(c.participants))
	var wg sync.WaitGroup
	var updateMu sync.Mutex
	updateChan := make(chan *federatedlearning.ModelUpdate, len(c.participants))
	errorChan := make(chan error, len(c.participants))

	activeParticipants := 0
	for _, participant := range c.participants {
		if !participant.Active {
			continue
		}
		activeParticipants++

		wg.Add(1)
		go func(p *federatedlearning.Participant) {
			defer wg.Done()

			// Send global model
			if err := c.client.SendModel(ctx, p, c.model); err != nil {
				c.logger.WithFields(logrus.Fields{
					"participant": p.ID,
					"error":       err,
				}).Error("failed to send model to participant")
				errorChan <- fmt.Errorf("send model to %s: %w", p.ID, err)
				return
			}

			// Wait for local training
			update, err := c.client.ReceiveUpdate(ctx, p)
			if err != nil {
				c.logger.WithFields(logrus.Fields{
					"participant": p.ID,
					"error":       err,
				}).Error("failed to receive update from participant")
				errorChan <- fmt.Errorf("receive update from %s: %w", p.ID, err)
				return
			}

			// Update participant state
			updateMu.Lock()
			p.LastUpdate = time.Now()
			if c.config.Privacy.DifferentialPrivacy {
				p.EpsilonUsed += update.EpsilonUsed
			}
			updateMu.Unlock()

			updateChan <- update
		}(participant)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(updateChan)
		close(errorChan)
	}()

	// Collect updates
	for update := range updateChan {
		updates = append(updates, update)
	}

	// Collect errors
	for err := range errorChan {
		trainingRound.Errors = append(trainingRound.Errors, err.Error())
	}

	communicationEnd := time.Now()
	trainingRound.CommunicationTime = communicationEnd.Sub(roundStart)
	trainingRound.UpdatesReceived = len(updates)
	trainingRound.ParticipantsActive = activeParticipants

	// Check minimum participants
	if len(updates) < c.config.Training.MinParticipants {
		trainingRound.Status = federatedlearning.RoundStatusFailed
		trainingRound.EndTime = time.Now()
		return trainingRound, federatedlearning.ErrInsufficientParticipants
	}

	// Step 2: Secure aggregation
	trainingRound.Status = federatedlearning.RoundStatusAggregating
	aggregationStart := time.Now()

	aggregated, err := c.aggregator.Aggregate(updates)
	if err != nil {
		trainingRound.Status = federatedlearning.RoundStatusFailed
		trainingRound.EndTime = time.Now()
		return trainingRound, &federatedlearning.Error{
			Code:    "AGGREGATION_FAILED",
			Message: "failed to aggregate model updates",
			Cause:   err,
		}
	}

	trainingRound.AggregationTime = time.Now().Sub(aggregationStart)

	// Step 3: Update global model
	c.mu.Lock()
	c.model = c.applyUpdate(c.model, aggregated)
	c.model.RoundNumber = round
	c.model.Version++
	c.model.UpdatedAt = time.Now()
	c.model.NumParticipants = len(updates)
	c.mu.Unlock()

	// Step 4: Complete round
	trainingRound.Status = federatedlearning.RoundStatusCompleted
	trainingRound.EndTime = time.Now()

	c.logger.WithFields(logrus.Fields{
		"round":            round,
		"participants":     len(updates),
		"communication_ms": trainingRound.CommunicationTime.Milliseconds(),
		"aggregation_ms":   trainingRound.AggregationTime.Milliseconds(),
	}).Info("federated training round completed")

	return trainingRound, nil
}

// applyUpdate applies aggregated update to the global model
func (c *FederatedCoordinator) applyUpdate(
	model *federatedlearning.GlobalModel,
	update *federatedlearning.ModelUpdate,
) *federatedlearning.GlobalModel {
	// Simple replacement for now
	// In a real implementation, this would apply gradient descent or similar
	updatedModel := &federatedlearning.GlobalModel{
		ID:              model.ID,
		Weights:         update.Weights,
		Version:         model.Version,
		RoundNumber:     model.RoundNumber,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       time.Now(),
		NumParticipants: int(update.NumSamples),
		TotalSamples:    update.NumSamples,
	}

	return updatedModel
}

// Train executes the full federated training process
func (c *FederatedCoordinator) Train(ctx context.Context) ([]*federatedlearning.TrainingRound, error) {
	c.logger.WithFields(logrus.Fields{
		"total_rounds":   c.config.Training.Rounds,
		"participants":   len(c.participants),
		"min_participants": c.config.Training.MinParticipants,
	}).Info("starting federated training")

	rounds := make([]*federatedlearning.TrainingRound, 0, c.config.Training.Rounds)

	for round := 1; round <= c.config.Training.Rounds; round++ {
		select {
		case <-ctx.Done():
			c.logger.Info("federated training cancelled")
			return rounds, ctx.Err()
		default:
		}

		trainingRound, err := c.TrainRound(ctx, round)
		rounds = append(rounds, trainingRound)

		if err != nil {
			c.logger.WithFields(logrus.Fields{
				"round": round,
				"error": err,
			}).Error("training round failed")

			// Continue to next round unless it's a fatal error
			if err == federatedlearning.ErrInsufficientParticipants {
				continue
			}
		}

		// Check convergence
		if c.checkConvergence(rounds) {
			c.logger.WithField("round", round).Info("training converged")
			break
		}
	}

	c.logger.WithFields(logrus.Fields{
		"completed_rounds": len(rounds),
		"model_version":    c.model.Version,
	}).Info("federated training completed")

	return rounds, nil
}

// checkConvergence checks if training has converged
func (c *FederatedCoordinator) checkConvergence(rounds []*federatedlearning.TrainingRound) bool {
	if len(rounds) < 2 {
		return false
	}

	// Simple convergence check based on successful rounds
	// In a real implementation, this would use loss metrics or model accuracy
	threshold := c.config.Training.ConvergenceThreshold
	if threshold <= 0 {
		return false
	}

	// Check if convergence metric is below threshold
	lastRound := rounds[len(rounds)-1]
	return lastRound.ConvergenceMetric < threshold
}

// GetModel returns the current global model
func (c *FederatedCoordinator) GetModel() *federatedlearning.GlobalModel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.model
}

// GetParticipants returns the list of participants
func (c *FederatedCoordinator) GetParticipants() []*federatedlearning.Participant {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.participants
}

// AddParticipant adds a new participant to the federation
func (c *FederatedCoordinator) AddParticipant(participant *federatedlearning.Participant) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if participant already exists
	for _, p := range c.participants {
		if p.ID == participant.ID {
			return &federatedlearning.Error{
				Code:    "DUPLICATE_PARTICIPANT",
				Message: "participant already exists",
			}
		}
	}

	c.participants = append(c.participants, participant)
	c.logger.WithField("participant", participant.ID).Info("participant added")
	return nil
}

// RemoveParticipant removes a participant from the federation
func (c *FederatedCoordinator) RemoveParticipant(participantID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, p := range c.participants {
		if p.ID == participantID {
			c.participants = append(c.participants[:i], c.participants[i+1:]...)
			c.logger.WithField("participant", participantID).Info("participant removed")
			return nil
		}
	}

	return &federatedlearning.Error{
		Code:    "PARTICIPANT_NOT_FOUND",
		Message: "participant not found",
	}
}

package federatedlearning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/federatedlearning"
)

// HTTPParticipantClient implements ParticipantClient using HTTP
type HTTPParticipantClient struct {
	httpClient *http.Client
	timeout    time.Duration
	logger     logrus.FieldLogger
}

// NewHTTPParticipantClient creates a new HTTP participant client
func NewHTTPParticipantClient(timeout time.Duration) *HTTPParticipantClient {
	return &HTTPParticipantClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		logger:  logrus.WithField("component", "participant_client"),
	}
}

// SendModel sends the global model to a participant
func (c *HTTPParticipantClient) SendModel(
	ctx context.Context,
	participant *federatedlearning.Participant,
	model *federatedlearning.GlobalModel,
) error {
	url := fmt.Sprintf("%s/federated/model", participant.Endpoint)

	c.logger.WithFields(logrus.Fields{
		"participant": participant.ID,
		"endpoint":    url,
		"model_version": model.Version,
	}).Debug("sending model to participant")

	// Prepare request body
	body, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("marshal model: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	c.logger.WithField("participant", participant.ID).Debug("model sent successfully")
	return nil
}

// ReceiveUpdate receives a model update from a participant
func (c *HTTPParticipantClient) ReceiveUpdate(
	ctx context.Context,
	participant *federatedlearning.Participant,
) (*federatedlearning.ModelUpdate, error) {
	url := fmt.Sprintf("%s/federated/update", participant.Endpoint)

	c.logger.WithFields(logrus.Fields{
		"participant": participant.ID,
		"endpoint":    url,
	}).Debug("receiving update from participant")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var update federatedlearning.ModelUpdate
	if err := json.NewDecoder(resp.Body).Decode(&update); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"participant": participant.ID,
		"samples":     update.NumSamples,
	}).Debug("update received successfully")

	return &update, nil
}

// Ping checks if a participant is reachable
func (c *HTTPParticipantClient) Ping(
	ctx context.Context,
	participant *federatedlearning.Participant,
) error {
	url := fmt.Sprintf("%s/health", participant.Endpoint)

	c.logger.WithFields(logrus.Fields{
		"participant": participant.ID,
		"endpoint":    url,
	}).Debug("pinging participant")

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	c.logger.WithField("participant", participant.ID).Debug("participant is healthy")
	return nil
}

// ParticipantServer handles incoming requests from the coordinator
type ParticipantServer struct {
	localModel      *federatedlearning.GlobalModel
	localTrainer    LocalTrainer
	privacyManager  *DifferentialPrivacy
	logger          logrus.FieldLogger
}

// LocalTrainer defines the interface for local model training
type LocalTrainer interface {
	Train(ctx context.Context, model *federatedlearning.GlobalModel, epochs int) (*federatedlearning.ModelUpdate, error)
}

// NewParticipantServer creates a new participant server
func NewParticipantServer(
	trainer LocalTrainer,
	privacyManager *DifferentialPrivacy,
	logger logrus.FieldLogger,
) *ParticipantServer {
	return &ParticipantServer{
		localTrainer:   trainer,
		privacyManager: privacyManager,
		logger:         logger,
	}
}

// HandleModelReceive handles receiving the global model from coordinator
func (s *ParticipantServer) HandleModelReceive(w http.ResponseWriter, r *http.Request) {
	var model federatedlearning.GlobalModel

	if err := json.NewDecoder(r.Body).Decode(&model); err != nil {
		s.logger.WithError(err).Error("failed to decode model")
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"model_version": model.Version,
		"round":         model.RoundNumber,
	}).Info("received global model from coordinator")

	// Store the model
	s.localModel = &model

	// Start local training in background
	go func() {
		ctx := context.Background()
		update, err := s.localTrainer.Train(ctx, &model, 5) // 5 local epochs
		if err != nil {
			s.logger.WithError(err).Error("local training failed")
			return
		}

		s.logger.WithFields(logrus.Fields{
			"samples": update.NumSamples,
			"round":   update.RoundNumber,
		}).Info("local training completed")
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "accepted",
		"message": "model received and training started",
	})
}

// HandleUpdateRequest handles sending the local update to coordinator
func (s *ParticipantServer) HandleUpdateRequest(w http.ResponseWriter, r *http.Request) {
	if s.localModel == nil {
		writeError(w, http.StatusNotFound, "no model available")
		return
	}

	// Train local model
	ctx := r.Context()
	update, err := s.localTrainer.Train(ctx, s.localModel, 5)
	if err != nil {
		s.logger.WithError(err).Error("local training failed")
		writeError(w, http.StatusInternalServerError, "training failed")
		return
	}

	// Apply differential privacy
	if s.privacyManager != nil {
		update.Weights = s.privacyManager.PrivatizeGradients(update.Weights, 1.0)
		update.NoiseAdded = true
		update.EpsilonUsed = s.privacyManager.epsilon
	}

	s.logger.WithFields(logrus.Fields{
		"samples":      update.NumSamples,
		"noise_added":  update.NoiseAdded,
		"epsilon_used": update.EpsilonUsed,
	}).Info("sending update to coordinator")

	writeJSON(w, http.StatusOK, update)
}

// HandleHealth handles health check requests
func (s *ParticipantServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
	})
}

// SimpleLocalTrainer is a simple implementation of LocalTrainer for testing
type SimpleLocalTrainer struct {
	dataSize int64
	logger   logrus.FieldLogger
}

// NewSimpleLocalTrainer creates a new simple local trainer
func NewSimpleLocalTrainer(dataSize int64) *SimpleLocalTrainer {
	return &SimpleLocalTrainer{
		dataSize: dataSize,
		logger:   logrus.WithField("component", "simple_trainer"),
	}
}

// Train performs simple local training (placeholder)
func (t *SimpleLocalTrainer) Train(
	ctx context.Context,
	model *federatedlearning.GlobalModel,
	epochs int,
) (*federatedlearning.ModelUpdate, error) {
	t.logger.WithFields(logrus.Fields{
		"epochs":     epochs,
		"data_size":  t.dataSize,
		"model_version": model.Version,
	}).Debug("starting local training")

	// Simulate training by making small random updates to weights
	// In production, this would be actual model training
	updatedWeights := make([]float32, len(model.Weights))
	for i, w := range model.Weights {
		// Small random update
		delta := (float32(i%10) - 5.0) * 0.01
		updatedWeights[i] = w + delta
	}

	update := &federatedlearning.ModelUpdate{
		ParticipantID:  model.ID, // Use model ID as placeholder
		Weights:        updatedWeights,
		NumSamples:     t.dataSize,
		RoundNumber:    model.RoundNumber,
		Timestamp:      time.Now(),
		NoiseAdded:     false,
		EpsilonUsed:    0,
	}

	t.logger.WithField("samples", t.dataSize).Debug("local training completed")
	return update, nil
}

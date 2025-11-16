package federatedlearning

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/weaviate/weaviate/entities/federatedlearning"
)

// Handler handles federated learning HTTP requests
type Handler struct {
	coordinator *FederatedCoordinator
	accountant  *PrivacyAccountant
	logger      logrus.FieldLogger
}

// NewHandler creates a new federated learning handler
func NewHandler(
	coordinator *FederatedCoordinator,
	accountant *PrivacyAccountant,
	logger logrus.FieldLogger,
) *Handler {
	return &Handler{
		coordinator: coordinator,
		accountant:  accountant,
		logger:      logger,
	}
}

// StartTraining starts a federated training session
func (h *Handler) StartTraining(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Info("starting federated training session")

	// Execute training in background
	go func() {
		rounds, err := h.coordinator.Train(context.Background())
		if err != nil {
			h.logger.WithError(err).Error("federated training failed")
			return
		}

		h.logger.WithField("rounds", len(rounds)).Info("federated training completed")
	}()

	// Return immediate response
	response := map[string]interface{}{
		"status":  "started",
		"message": "federated training started",
	}

	writeJSON(w, http.StatusAccepted, response)
}

// GetTrainingStatus returns the current training status
func (h *Handler) GetTrainingStatus(w http.ResponseWriter, r *http.Request) {
	model := h.coordinator.GetModel()
	participants := h.coordinator.GetParticipants()

	activeParticipants := 0
	for _, p := range participants {
		if p.Active {
			activeParticipants++
		}
	}

	response := map[string]interface{}{
		"current_round":       model.RoundNumber,
		"model_version":       model.Version,
		"total_participants":  len(participants),
		"active_participants": activeParticipants,
		"last_updated":        model.UpdatedAt,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetModel returns the current global model
func (h *Handler) GetModel(w http.ResponseWriter, r *http.Request) {
	model := h.coordinator.GetModel()

	response := map[string]interface{}{
		"id":               model.ID,
		"version":          model.Version,
		"round_number":     model.RoundNumber,
		"num_participants": model.NumParticipants,
		"total_samples":    model.TotalSamples,
		"accuracy":         model.Accuracy,
		"created_at":       model.CreatedAt,
		"updated_at":       model.UpdatedAt,
		"weights":          model.Weights,
	}

	writeJSON(w, http.StatusOK, response)
}

// SubmitModelUpdate submits a local model update from a participant
func (h *Handler) SubmitModelUpdate(w http.ResponseWriter, r *http.Request) {
	var update federatedlearning.ModelUpdate

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		h.logger.WithError(err).Error("failed to decode model update")
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate update
	if len(update.Weights) == 0 {
		writeError(w, http.StatusBadRequest, "weights cannot be empty")
		return
	}

	if update.NumSamples <= 0 {
		writeError(w, http.StatusBadRequest, "num_samples must be positive")
		return
	}

	// Store update (in production, this would be stored in a repository)
	h.logger.WithFields(logrus.Fields{
		"participant": update.ParticipantID,
		"round":       update.RoundNumber,
		"samples":     update.NumSamples,
	}).Info("model update received")

	response := map[string]interface{}{
		"status":  "accepted",
		"message": "model update accepted",
	}

	writeJSON(w, http.StatusAccepted, response)
}

// AddParticipant adds a new participant to the federation
func (h *Handler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID            string  `json:"id"`
		Endpoint      string  `json:"endpoint"`
		Weight        float64 `json:"weight"`
		EpsilonBudget float64 `json:"epsilon_budget"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "endpoint is required")
		return
	}

	// Parse or generate participant ID
	var participantID uuid.UUID
	var err error
	if req.ID != "" {
		participantID, err = uuid.Parse(req.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid participant ID")
			return
		}
	} else {
		participantID = uuid.New()
	}

	// Create participant
	participant := &federatedlearning.Participant{
		ID:            participantID,
		Endpoint:      req.Endpoint,
		Weight:        req.Weight,
		Active:        true,
		EpsilonBudget: req.EpsilonBudget,
		EpsilonUsed:   0,
	}

	// Add to coordinator
	if err := h.coordinator.AddParticipant(participant); err != nil {
		h.logger.WithError(err).Error("failed to add participant")
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := map[string]interface{}{
		"id":       participantID,
		"status":   "added",
		"endpoint": req.Endpoint,
	}

	writeJSON(w, http.StatusCreated, response)
}

// RemoveParticipant removes a participant from the federation
func (h *Handler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	// Get participant ID from URL path
	participantIDStr := r.URL.Query().Get("id")
	if participantIDStr == "" {
		writeError(w, http.StatusBadRequest, "participant ID is required")
		return
	}

	participantID, err := uuid.Parse(participantIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid participant ID")
		return
	}

	if err := h.coordinator.RemoveParticipant(participantID); err != nil {
		h.logger.WithError(err).Error("failed to remove participant")
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	response := map[string]interface{}{
		"status":  "removed",
		"message": fmt.Sprintf("participant %s removed", participantID),
	}

	writeJSON(w, http.StatusOK, response)
}

// GetPrivacyBudget returns the current privacy budget status
func (h *Handler) GetPrivacyBudget(w http.ResponseWriter, r *http.Request) {
	epsilonRemaining, deltaRemaining := h.accountant.GetRemainingBudget()
	queries := h.accountant.GetQueries()

	response := map[string]interface{}{
		"epsilon_remaining": epsilonRemaining,
		"delta_remaining":   deltaRemaining,
		"total_queries":     len(queries),
		"queries":           queries,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetParticipants returns the list of participants
func (h *Handler) GetParticipants(w http.ResponseWriter, r *http.Request) {
	participants := h.coordinator.GetParticipants()

	// Convert to response format
	participantsList := make([]map[string]interface{}, len(participants))
	for i, p := range participants {
		participantsList[i] = map[string]interface{}{
			"id":              p.ID,
			"endpoint":        p.Endpoint,
			"data_size":       p.DataSize,
			"last_update":     p.LastUpdate,
			"epsilon_budget":  p.EpsilonBudget,
			"epsilon_used":    p.EpsilonUsed,
			"active":          p.Active,
			"weight":          p.Weight,
		}
	}

	response := map[string]interface{}{
		"total":        len(participants),
		"participants": participantsList,
	}

	writeJSON(w, http.StatusOK, response)
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.WithError(err).Error("failed to encode JSON response")
	}
}

// writeError writes an error response
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
	})
}

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

package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key with enhanced features
type APIKey struct {
	ID        string     `json:"id"`
	Key       string     `json:"-"` // Hashed key, never exposed
	Name      string     `json:"name"`
	Roles     []string   `json:"roles"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LastUsed  time.Time  `json:"last_used"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy string     `json:"created_by"`
	Enabled   bool       `json:"enabled"`
}

// User represents a user with API keys
type User struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Roles     []string   `json:"roles"`
	APIKeys   []*APIKey  `json:"api_keys"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// APIKeyManager manages API keys with enhanced features
type APIKeyManager struct {
	mu      sync.RWMutex
	keys    map[string]*APIKey // map[keyHash]APIKey
	users   map[string]*User   // map[userID]User
	keyUser map[string]string  // map[keyHash]userID
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys:    make(map[string]*APIKey),
		users:   make(map[string]*User),
		keyUser: make(map[string]string),
	}
}

// CreateAPIKey creates a new API key for a user
func (m *APIKeyManager) CreateAPIKey(userID, name, createdBy string, roles []string, expiresAt *time.Time) (*APIKey, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate random key
	rawKey, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the key
	keyHash := hashAPIKey(rawKey)

	// Create API key
	apiKey := &APIKey{
		ID:        uuid.New().String(),
		Key:       keyHash,
		Name:      name,
		Roles:     roles,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		CreatedBy: createdBy,
		Enabled:   true,
	}

	// Store key
	m.keys[keyHash] = apiKey
	m.keyUser[keyHash] = userID

	// Add to user's keys
	if user, exists := m.users[userID]; exists {
		user.APIKeys = append(user.APIKeys, apiKey)
		user.UpdatedAt = time.Now()
	}

	return apiKey, rawKey, nil
}

// ValidateAPIKey validates an API key and returns the associated user
func (m *APIKeyManager) ValidateAPIKey(rawKey string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyHash := hashAPIKey(rawKey)

	// Find API key
	apiKey, exists := m.keys[keyHash]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check if key is enabled
	if !apiKey.Enabled {
		return nil, fmt.Errorf("API key is disabled")
	}

	// Check expiration
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last used time
	apiKey.LastUsed = time.Now()

	// Get user
	userID, exists := m.keyUser[keyHash]
	if !exists {
		return nil, fmt.Errorf("user not found for API key")
	}

	user, exists := m.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// RevokeAPIKey revokes an API key
func (m *APIKeyManager) RevokeAPIKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and disable the key
	for keyHash, apiKey := range m.keys {
		if apiKey.ID == keyID {
			apiKey.Enabled = false

			// Update user's keys
			if userID, exists := m.keyUser[keyHash]; exists {
				if user, ok := m.users[userID]; ok {
					user.UpdatedAt = time.Now()
				}
			}

			return nil
		}
	}

	return fmt.Errorf("API key not found")
}

// RotateAPIKey rotates an API key (revokes old and creates new)
func (m *APIKeyManager) RotateAPIKey(keyID string) (*APIKey, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the old key
	var oldKey *APIKey
	var oldKeyHash string
	for keyHash, apiKey := range m.keys {
		if apiKey.ID == keyID {
			oldKey = apiKey
			oldKeyHash = keyHash
			break
		}
	}

	if oldKey == nil {
		return nil, "", fmt.Errorf("API key not found")
	}

	// Get user ID
	userID, exists := m.keyUser[oldKeyHash]
	if !exists {
		return nil, "", fmt.Errorf("user not found for API key")
	}

	// Disable old key
	oldKey.Enabled = false

	// Generate new key
	rawKey, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}

	keyHash := hashAPIKey(rawKey)

	// Create new API key with same properties
	newKey := &APIKey{
		ID:        uuid.New().String(),
		Key:       keyHash,
		Name:      oldKey.Name + " (rotated)",
		Roles:     oldKey.Roles,
		ExpiresAt: oldKey.ExpiresAt,
		CreatedAt: time.Now(),
		CreatedBy: oldKey.CreatedBy,
		Enabled:   true,
	}

	// Store new key
	m.keys[keyHash] = newKey
	m.keyUser[keyHash] = userID

	// Add to user's keys
	if user, ok := m.users[userID]; ok {
		user.APIKeys = append(user.APIKeys, newKey)
		user.UpdatedAt = time.Now()
	}

	return newKey, rawKey, nil
}

// ListAPIKeys lists all API keys for a user
func (m *APIKeyManager) ListAPIKeys(userID string) ([]*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user.APIKeys, nil
}

// CreateUser creates a new user
func (m *APIKeyManager) CreateUser(email string, roles []string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user := &User{
		ID:        uuid.New().String(),
		Email:     email,
		Roles:     roles,
		APIKeys:   make([]*APIKey, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.users[user.ID] = user

	return user, nil
}

// GetUser retrieves a user by ID
func (m *APIKeyManager) GetUser(userID string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// Helper functions

// generateAPIKey generates a secure random API key
func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// hashAPIKey hashes an API key using SHA-256
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return base64.URLEncoding.EncodeToString(hash[:])
}

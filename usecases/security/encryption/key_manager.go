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

package encryption

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EncryptionKey represents a cryptographic key
type EncryptionKey struct {
	ID        string
	KeyBytes  []byte
	CreatedAt time.Time
	Version   int
}

// Bytes returns the key bytes
func (k *EncryptionKey) Bytes() []byte {
	return k.KeyBytes
}

// KeyManager manages encryption keys
type KeyManager interface {
	GetKey(className, propertyName string) (*EncryptionKey, error)
	GetKeyByID(keyID string) (*EncryptionKey, error)
	RotateKey(className, propertyName string) (*EncryptionKey, error)
}

// LocalKeyManager implements KeyManager using local storage
type LocalKeyManager struct {
	mu   sync.RWMutex
	keys map[string]*EncryptionKey // map[keyID]key
	// fieldKeys maps className.propertyName to current key ID
	fieldKeys map[string]string
}

// NewLocalKeyManager creates a new local key manager
func NewLocalKeyManager() *LocalKeyManager {
	return &LocalKeyManager{
		keys:      make(map[string]*EncryptionKey),
		fieldKeys: make(map[string]string),
	}
}

// GetKey retrieves or creates a key for a field
func (m *LocalKeyManager) GetKey(className, propertyName string) (*EncryptionKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fieldPath := fmt.Sprintf("%s.%s", className, propertyName)

	// Check if key already exists
	if keyID, exists := m.fieldKeys[fieldPath]; exists {
		if key, ok := m.keys[keyID]; ok {
			return key, nil
		}
	}

	// Generate new key
	key, err := m.generateKey()
	if err != nil {
		return nil, err
	}

	// Store key
	m.keys[key.ID] = key
	m.fieldKeys[fieldPath] = key.ID

	return key, nil
}

// GetKeyByID retrieves a key by its ID
func (m *LocalKeyManager) GetKeyByID(keyID string) (*EncryptionKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	return key, nil
}

// RotateKey rotates the encryption key for a field
func (m *LocalKeyManager) RotateKey(className, propertyName string) (*EncryptionKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fieldPath := fmt.Sprintf("%s.%s", className, propertyName)

	// Generate new key
	newKey, err := m.generateKey()
	if err != nil {
		return nil, err
	}

	// Get old key version
	oldVersion := 1
	if oldKeyID, exists := m.fieldKeys[fieldPath]; exists {
		if oldKey, ok := m.keys[oldKeyID]; ok {
			oldVersion = oldKey.Version
		}
	}

	newKey.Version = oldVersion + 1

	// Store new key
	m.keys[newKey.ID] = newKey
	m.fieldKeys[fieldPath] = newKey.ID

	return newKey, nil
}

// generateKey generates a new AES-256 key (32 bytes)
func (m *LocalKeyManager) generateKey() (*EncryptionKey, error) {
	keyBytes := make([]byte, 32) // AES-256 requires 32 bytes
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	return &EncryptionKey{
		ID:        uuid.New().String(),
		KeyBytes:  keyBytes,
		CreatedAt: time.Now(),
		Version:   1,
	}, nil
}

// VaultKeyManager implements KeyManager using HashiCorp Vault
// This is a placeholder for Vault integration
type VaultKeyManager struct {
	vaultAddr  string
	vaultToken string
	namespace  string
}

// NewVaultKeyManager creates a new Vault key manager
func NewVaultKeyManager(addr, token, namespace string) *VaultKeyManager {
	return &VaultKeyManager{
		vaultAddr:  addr,
		vaultToken: token,
		namespace:  namespace,
	}
}

// GetKey retrieves a key from Vault
func (m *VaultKeyManager) GetKey(className, propertyName string) (*EncryptionKey, error) {
	// TODO: Implement Vault integration
	return nil, fmt.Errorf("Vault integration not yet implemented")
}

// GetKeyByID retrieves a key by ID from Vault
func (m *VaultKeyManager) GetKeyByID(keyID string) (*EncryptionKey, error) {
	// TODO: Implement Vault integration
	return nil, fmt.Errorf("Vault integration not yet implemented")
}

// RotateKey rotates a key in Vault
func (m *VaultKeyManager) RotateKey(className, propertyName string) (*EncryptionKey, error) {
	// TODO: Implement Vault integration
	return nil, fmt.Errorf("Vault integration not yet implemented")
}

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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptedField represents an encrypted field with all necessary metadata
type EncryptedField struct {
	Algorithm  string `json:"algorithm"`
	KeyID      string `json:"key_id"`
	IV         []byte `json:"iv"`
	Ciphertext []byte `json:"ciphertext"`
	Tag        []byte `json:"tag"` // For AEAD (Authenticated Encryption with Associated Data)
}

// FieldEncryption handles field-level encryption operations
type FieldEncryption struct {
	keyManager KeyManager
}

// NewFieldEncryption creates a new field encryption instance
func NewFieldEncryption(keyManager KeyManager) *FieldEncryption {
	return &FieldEncryption{
		keyManager: keyManager,
	}
}

// Encrypt encrypts a field value using AES-256-GCM
func (e *FieldEncryption) Encrypt(className, propertyName string, plaintext []byte) (*EncryptedField, error) {
	// Get encryption key for field
	key, err := e.keyManager.GetKey(className, propertyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}

	// Generate IV (12 bytes for GCM)
	iv := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, iv, plaintext, nil)

	// Split ciphertext and tag
	tagSize := gcm.Overhead()
	if len(ciphertext) < tagSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	return &EncryptedField{
		Algorithm:  "AES-256-GCM",
		KeyID:      key.ID,
		IV:         iv,
		Ciphertext: ciphertext[:len(ciphertext)-tagSize],
		Tag:        ciphertext[len(ciphertext)-tagSize:],
	}, nil
}

// Decrypt decrypts an encrypted field
func (e *FieldEncryption) Decrypt(className, propertyName string, encrypted *EncryptedField) ([]byte, error) {
	if encrypted.Algorithm != "AES-256-GCM" {
		return nil, fmt.Errorf("unsupported algorithm: %s", encrypted.Algorithm)
	}

	// Get decryption key
	key, err := e.keyManager.GetKeyByID(encrypted.KeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get decryption key: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(key.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Reconstruct full ciphertext with tag
	fullCiphertext := append(encrypted.Ciphertext, encrypted.Tag...)

	// Decrypt
	plaintext, err := gcm.Open(nil, encrypted.IV, fullCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptionConfig defines encryption configuration for a property
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"` // AES-256-GCM
	KeySource string `json:"keySource"` // vault | kms | local
}

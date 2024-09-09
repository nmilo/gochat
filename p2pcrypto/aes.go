package p2pcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptMessage encrypts the given plaintext using AES-GCM with the derived key.
// The result will be the nonce concatenated with the ciphertext.
func EncryptMessage(key []byte, plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM block cipher: %v", err)
	}

	// Generate a random nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt the message and concatenate nonce and ciphertext
	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)

	// Concatenate nonce and ciphertext
	return append(nonce, ciphertext...), nil
}

// DecryptMessage decrypts the given ciphertext using AES-GCM with the derived key.
// The nonce is extracted from the beginning of the concatenated ciphertext.
func DecryptMessage(key, combinedData []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM block cipher: %v", err)
	}

	// Extract the nonce from the beginning of the combined data
	nonceSize := aesGCM.NonceSize()
	if len(combinedData) < nonceSize {
		return "", fmt.Errorf("invalid ciphertext: too short")
	}

	nonce, ciphertext := combinedData[:nonceSize], combinedData[nonceSize:]

	// Decrypt the message
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message: %v", err)
	}

	return string(plaintext), nil
}

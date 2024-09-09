package p2pcrypto

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
)

// 1536-bit safe prime (RFC 3526 Group 5)
var primeHex = "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFFFFFFFFFF"
var g = big.NewInt(2)

// GenerateKeyPair generates a Diffie-Hellman key pair (private and public keys)
func GenerateKeyPair() (*big.Int, *big.Int, error) {
	p, ok := new(big.Int).SetString(primeHex, 16)
	if !ok || p == nil {
		return nil, nil, fmt.Errorf("failed to initialize the prime number")
	}

	privateKey, err := rand.Int(rand.Reader, p)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	publicKey := new(big.Int).Exp(g, privateKey, p)
	return privateKey, publicKey, nil
}

// ComputeSharedSecret computes the shared secret using the peer's public key
func ComputeSharedSecret(privateKey, peerPublicKey *big.Int) (*big.Int, error) {
	p, ok := new(big.Int).SetString(primeHex, 16)
	if !ok || p == nil {
		return nil, fmt.Errorf("failed to initialize the prime number")
	}

	sharedSecret := new(big.Int).Exp(peerPublicKey, privateKey, p)
	return sharedSecret, nil
}

// DeriveKeyFromSecret derives a 32-byte AES key from the shared secret using SHA-256
func DeriveKeyFromSecret(sharedSecret *big.Int) []byte {
	hash := sha256.Sum256(sharedSecret.Bytes())
	return hash[:]
}

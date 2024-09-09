package p2pcrypto

import "math/big"

// PerformKeyExchange performs the Diffie-Hellman key exchange between two peers
func PerformKeyExchange(privateKey, peerPublicKey *big.Int) ([]byte, error) {
	sharedSecret, err := ComputeSharedSecret(privateKey, peerPublicKey)
	if err != nil {
		return nil, err
	}

	// Derive the AES key from the shared secret
	return DeriveKeyFromSecret(sharedSecret), nil
}

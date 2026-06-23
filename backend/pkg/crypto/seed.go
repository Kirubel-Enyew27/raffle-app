package crypto

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateSeed creates a cryptographically secure random seed and its SHA-256 hash
// The hash is committed BEFORE the draw, the seed is revealed AFTER
func GenerateSeed() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	seed := hex.EncodeToString(bytes)
	hash := SHA256(seed)
	return seed, hash, nil
}

package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

func SHA256(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func VerifyCommit(commit, seed string) bool {
	return SHA256(seed) == commit
}

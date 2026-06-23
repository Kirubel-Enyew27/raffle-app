package crypto

import "fmt"

// GenerateClientSeed creates a deterministic client seed from raffle context
func GenerateClientSeed(raffleID string, drawTime int64) string {
	data := fmt.Sprintf("%s:%d", raffleID, drawTime)
	return SHA256(data)
}

// GenerateCombinedHash creates the final hash used to pick the winner
func GenerateCombinedHash(serverSeed, clientSeed string, nonce int) string {
	data := fmt.Sprintf("%s:%s:%d", serverSeed, clientSeed, nonce)
	return SHA256(data)
}

// IndexFromHash converts a hash string to a ticket index (0-based)
func IndexFromHash(combinedHash string, totalTickets int) int {
	if totalTickets <= 0 {
		return 0
	}
	if len(combinedHash) < 16 {
		return 0
	}
	prefix := combinedHash[:16]
	num := uint64(0)
	for _, c := range prefix {
		var digit uint64
		if c >= '0' && c <= '9' {
			digit = uint64(c - '0')
		} else {
			digit = uint64(c - 'a' + 10)
		}
		num = num*16 + digit
	}
	return int(num % uint64(totalTickets))
}

// GenerateDrawProof creates a complete verifiable proof for a draw
func GenerateDrawProof(serverSeed, serverSeedHash, clientSeed string, nonce, totalTickets int) (int, string, string) {
	combinedHash := GenerateCombinedHash(serverSeed, clientSeed, nonce)
	winningIndex := IndexFromHash(combinedHash, totalTickets)
	return winningIndex, combinedHash, serverSeedHash
}

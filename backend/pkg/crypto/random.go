package crypto

import "fmt"

func GenerateRandom(serverSeed, clientSeed string, nonce int, maxTickets int) int {
	combined := fmt.Sprintf("%s:%s:%d", serverSeed, clientSeed, nonce)
	hash := SHA256(combined)
	return IndexFromHash(hash, maxTickets)
}

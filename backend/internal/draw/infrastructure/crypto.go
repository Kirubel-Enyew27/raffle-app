package infrastructure

import (
	"github.com/raffle-app/backend/pkg/crypto"
)

type CryptoSeedService struct{}

func NewCryptoSeedService() *CryptoSeedService {
	return &CryptoSeedService{}
}

func (s *CryptoSeedService) GenerateSeed() (string, string, error) {
	return crypto.GenerateSeed()
}

func (s *CryptoSeedService) CommitSeed(seed string) (string, error) {
	return crypto.SHA256(seed), nil
}

func (s *CryptoSeedService) VerifyCommit(commit, seed string) bool {
	return crypto.VerifyCommit(commit, seed)
}

type CryptoRandomService struct{}

func NewCryptoRandomService() *CryptoRandomService {
	return &CryptoRandomService{}
}

func (s *CryptoRandomService) GenerateRandom(serverSeed, clientSeed string, nonce, maxTickets int) int {
	return crypto.GenerateRandom(serverSeed, clientSeed, nonce, maxTickets)
}

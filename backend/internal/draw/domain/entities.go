package domain

import (
	"time"
)

type DrawResult struct {
	ID              string
	RaffleID        string
	DrawTimestamp   time.Time
	Status          string
	WinningTicketID string
	WinningTicketNumber int
	Proof           DrawProof
	CreatedAt       time.Time
}

type DrawProof struct {
	CommitHash      string
	RevealedSeed    string
	CombinedHash    string
	WinningNumber   int
	VerificationURL string
}

type DrawCommitment struct {
	ID          string
	RaffleID    string
	ServerSeed  string
	CommitHash  string
	CreatedAt   time.Time
}

type VerificationResult struct {
	Verified        bool
	SeedMatches     bool
	HashMatches     bool
	CommitHash      string
	RevealedSeed    string
	CombinedHash    string
	WinningNumber   int
	VerificationURL string
}

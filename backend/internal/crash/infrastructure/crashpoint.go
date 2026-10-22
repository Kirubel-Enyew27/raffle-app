package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/raffle-app/backend/internal/crash/domain"
	appcrypto "github.com/raffle-app/backend/pkg/crypto"
)

// The multiplier curve helpers (MultiplierAt / DurationForMultiplier) live in
// the domain package because both the service and the engine need them.

// CrashPointSource derives crash points from a committed server seed.
//
// Algorithm (documented so anyone can reproduce it):
//
//	digest      = SHA256(server_seed || ":" || nonce)
//	n           = first 8 bytes of digest, big endian, modulo buckets (1000)
//	crash_bp    = max(minMultiplierBP, (100000 * (10000 - houseEdgeBP) / 10000) / (buckets - n))
//
// Why this shape: for a payout multiplier m >= 1 the probability that a round
// survives to m must be (1 - edge)/m for the game to return (1 - edge) per unit
// staked. Inverting that survival curve gives m = (1-edge)/(1-u) for u uniform
// in [0,1); with 1000 buckets, u = n/1000, so the integer form above is exact
// and gives a realised RTP of exactly (1 - edge) as the bucket count grows.
//
// With the default 300bp edge this makes P(crash == 1.00x) = 31/1000 (~3.1%),
// P(crash >= 2.00x) = 485/1000, and so on. All arithmetic is integer: the game
// never derives money from floating point.
//
// Fairness model: this is commit-reveal, not a trustless scheme. The server
// seed hash is published (and broadcast) when the round is created, before the
// betting window opens, and the seed itself is only exposed after the round has
// crashed. A player can therefore prove that the crash point was fixed before
// they bet and was not changed afterwards. It does *not* remove the need to
// trust that the published commitment was generated before betting opened;
// the app records the publication timestamp for exactly that reason. The game
// is therefore described as "commit-reveal verifiable" and never as
// "provably fair".
type CrashPointSource struct {
	houseEdgeBP     int
	buckets         int
	minMultiplierBP int
	maxMultiplierBP int
}

// Buckets is the resolution of the uniform input used to derive crash points.
const Buckets = 1000

// MinMultiplierBP is the floor: a round never crashes below 1.00x.
const MinMultiplierBP = domain.MinMultiplierBP

func NewCrashPointSource(houseEdgeBP, maxMultiplierBP int) *CrashPointSource {
	if houseEdgeBP < 0 {
		houseEdgeBP = 0
	}
	if houseEdgeBP >= 10000 {
		houseEdgeBP = 9999
	}
	if maxMultiplierBP < MinMultiplierBP {
		maxMultiplierBP = MinMultiplierBP
	}
	return &CrashPointSource{
		houseEdgeBP:     houseEdgeBP,
		buckets:         Buckets,
		minMultiplierBP: MinMultiplierBP,
		maxMultiplierBP: maxMultiplierBP,
	}
}

// Generate produces a fresh random server seed together with its public
// commitment and derived crash point.
func (s *CrashPointSource) Generate(nonce int64) (*domain.SeedCommitment, error) {
	seed, err := RandomSeed()
	if err != nil {
		return nil, err
	}
	return &domain.SeedCommitment{
		ServerSeed:        seed,
		ServerSeedHash:    s.Commit(seed),
		Nonce:             nonce,
		CrashMultiplierBP: s.CrashPoint(seed, nonce),
	}, nil
}

// Commit returns the public commitment for a seed.
func (s *CrashPointSource) Commit(serverSeed string) string {
	return appcrypto.SHA256(serverSeed)
}

// Verify reports whether a revealed seed matches a published commitment.
func (s *CrashPointSource) Verify(serverSeed, serverSeedHash string) bool {
	if serverSeed == "" || serverSeedHash == "" {
		return false
	}
	return s.Commit(serverSeed) == serverSeedHash
}

// CrashPoint derives the crash multiplier (in basis points) for a seed/nonce.
func (s *CrashPointSource) CrashPoint(serverSeed string, nonce int64) int {
	digest := appcrypto.SHA256(fmt.Sprintf("%s:%d", serverSeed, nonce))
	n := bucketFromHex(digest, s.buckets)

	// numerator == 100000 * (1 - edge) computed with integers.
	numerator := 100000 * (10000 - s.houseEdgeBP) / 10000
	denominator := s.buckets - n
	if denominator < 1 {
		denominator = 1
	}

	bp := numerator / denominator
	if bp < s.minMultiplierBP {
		bp = s.minMultiplierBP
	}
	if bp > s.maxMultiplierBP {
		bp = s.maxMultiplierBP
	}
	return bp
}

// SurvivalProbability is the theoretical chance a round reaches multiplierBP.
// It is used by tests to assert the realised return-to-player.
func (s *CrashPointSource) SurvivalProbability(multiplierBP int) float64 {
	if multiplierBP <= 0 {
		return 0
	}
	m := float64(multiplierBP) / 100
	if m < 1 {
		m = 1
	}
	return (1 - float64(s.houseEdgeBP)/10000) / m
}

// bucketFromHex maps the first 8 bytes of a hex digest into [0, buckets).
func bucketFromHex(digest string, buckets int) int {
	if len(digest) < 16 {
		return 0
	}
	var v uint64
	for i := 0; i < 16; i++ {
		c := digest[i]
		var d uint64
		switch {
		case c >= '0' && c <= '9':
			d = uint64(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint64(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			d = uint64(c - 'A' + 10)
		default:
			d = 0
		}
		v = v*16 + d
	}
	return int(v % uint64(buckets))
}

// RandomSeed returns a 256-bit cryptographically secure hex seed.
func RandomSeed() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate crash seed: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

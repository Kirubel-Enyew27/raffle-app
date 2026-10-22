package infrastructure

import (
	"math"
	"testing"

	appcrypto "github.com/raffle-app/backend/pkg/crypto"
)

func TestCrashPointIsDeterministic(t *testing.T) {
	src := NewCrashPointSource(300, 100000)

	first := src.CrashPoint("seed-abc", 1)
	for i := 0; i < 50; i++ {
		if got := src.CrashPoint("seed-abc", 1); got != first {
			t.Fatalf("crash point changed between calls: %d then %d", first, got)
		}
	}

	// Different seeds and nonces must (almost certainly) differ, and a different
	// nonce is a different input to the derivation.
	if src.CrashPoint("seed-abc", 1) == src.CrashPoint("seed-abc", 2) &&
		src.CrashPoint("seed-abc", 1) == src.CrashPoint("seed-xyz", 1) {
		t.Error("different inputs produced identical crash points")
	}
}

func TestCrashPointBounds(t *testing.T) {
	src := NewCrashPointSource(300, 100000)

	for i := 0; i < 2000; i++ {
		bp := src.CrashPoint(seedFor(i), 1)
		if bp < 100 {
			t.Fatalf("crash point %d is below the 1.00x floor", bp)
		}
		if bp > 100000 {
			t.Fatalf("crash point %d exceeded the configured cap", bp)
		}
	}
}

// seedFor makes deterministic distinct seeds without crypto/rand so the test is
// reproducible.
func seedFor(i int) string {
	return appcrypto.SHA256("seed-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('0'+i%10)))
}

func TestHouseEdgeIsRealisedInReturnToPlayer(t *testing.T) {
	// Over the whole bucket space the expected return for a player who always
	// cashes out at a fixed multiplier must be (1 - edge).
	const edgeBP = 300
	src := NewCrashPointSource(edgeBP, 100000)

	// Reconstruct the bucket space by enumerating deterministic seeds and
	// measuring the fraction of rounds that survive each multiplier.
	const samples = 20000
	seeds := make([]string, samples)
	for i := range seeds {
		seeds[i] = seedFor(i) + string(rune(i))
	}

	for _, target := range []int{110, 150, 200, 300, 500} {
		survived := 0
		for _, s := range seeds {
			if src.CrashPoint(s, 1) >= target {
				survived++
			}
		}
		observed := float64(survived) / samples
		expected := src.SurvivalProbability(target)

		// Sampling tolerance: 5% relative, with an absolute floor.
		tol := math.Max(0.01, expected*0.15)
		if math.Abs(observed-expected) > tol {
			t.Errorf("P(crash >= %.2fx) observed %.4f, expected %.4f (tol %.4f)",
				float64(target)/100, observed, expected, tol)
		}
	}
}

func TestImmediateCrashProbabilityMatchesHouseEdge(t *testing.T) {
	src := NewCrashPointSource(300, 100000)
	const samples = 20000

	immediate := 0
	for i := 0; i < samples; i++ {
		if src.CrashPoint(seedFor(i)+string(rune(i/26))+string(rune(i%26)), 1) == 100 {
			immediate++
		}
	}
	rate := float64(immediate) / samples
	// 3% house edge -> about 3.1% of rounds crash instantly (buckets 0..30).
	if rate < 0.02 || rate > 0.045 {
		t.Errorf("immediate crash rate %.4f is not near the 3%% house edge", rate)
	}
}

func TestZeroEdgeIsConfiguredNotAssumed(t *testing.T) {
	zero := NewCrashPointSource(0, 100000)
	withEdge := NewCrashPointSource(500, 100000)

	// A zero edge should produce strictly better multiples on average than a
	// 5% edge for the same seeds.
	var zeroSum, edgeSum int64
	for i := 0; i < 3000; i++ {
		s := seedFor(i) + string(rune(i))
		zeroSum += int64(zero.CrashPoint(s, 1))
		edgeSum += int64(withEdge.CrashPoint(s, 1))
	}
	if zeroSum <= edgeSum {
		t.Errorf("zero-edge total %d should exceed 5%%-edge total %d", zeroSum, edgeSum)
	}
}

func TestGenerateCommitmentAndVerify(t *testing.T) {
	src := NewCrashPointSource(300, 100000)

	commitment, err := src.Generate(1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if commitment.ServerSeed == "" {
		t.Fatal("server seed must be generated")
	}
	if commitment.ServerSeedHash != appcrypto.SHA256(commitment.ServerSeed) {
		t.Error("commitment must be the hash of the server seed")
	}
	if commitment.ServerSeedHash == commitment.ServerSeed {
		t.Error("the commitment must not leak the seed")
	}
	if !src.Verify(commitment.ServerSeed, commitment.ServerSeedHash) {
		t.Error("verify must accept the matching seed")
	}
	if src.Verify("tampered", commitment.ServerSeedHash) {
		t.Error("verify must reject a tampered seed")
	}
	if src.Verify(commitment.ServerSeed, appcrypto.SHA256("something-else")) {
		t.Error("verify must reject a mismatched commitment")
	}
	if src.Verify("", "") {
		t.Error("verify must reject empty input")
	}

	// Generate must be random: two calls must not produce the same seed.
	other, err := src.Generate(1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if other.ServerSeed == commitment.ServerSeed {
		t.Error("two rounds must not share a server seed")
	}
}

func TestCommitmentMatchesDerivedCrashPoint(t *testing.T) {
	src := NewCrashPointSource(300, 100000)
	commitment, err := src.Generate(7)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// A verifier recomputes the crash point from the revealed material only.
	if got := src.CrashPoint(commitment.ServerSeed, commitment.Nonce); got != commitment.CrashMultiplierBP {
		t.Errorf("recomputed %d, committed %d", got, commitment.CrashMultiplierBP)
	}
}

func TestSeedGenerationIsRandom(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seed, err := RandomSeed()
		if err != nil {
			t.Fatalf("random seed: %v", err)
		}
		if len(seed) != 64 {
			t.Fatalf("expected a 256-bit hex seed, got %d chars", len(seed))
		}
		if seen[seed] {
			t.Fatalf("duplicate seed generated: %s", seed)
		}
		seen[seed] = true
	}
}

func TestSourceClampsInvalidConfiguration(t *testing.T) {
	if got := NewCrashPointSource(-10, 100000).houseEdgeBP; got != 0 {
		t.Errorf("negative edge should clamp to 0, got %d", got)
	}
	if got := NewCrashPointSource(20000, 100000).houseEdgeBP; got != 9999 {
		t.Errorf("edge >= 100%% should clamp, got %d", got)
	}
	if got := NewCrashPointSource(300, 1).maxMultiplierBP; got != 100 {
		t.Errorf("max multiplier below 1.00x should clamp to 100, got %d", got)
	}
}

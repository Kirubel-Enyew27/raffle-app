package crypto

import (
	"testing"
)

func TestGenerateSeed_UniqueSeeds(t *testing.T) {
	seed1, hash1, err := GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	seed2, hash2, err := GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	if seed1 == seed2 {
		t.Error("seeds should be unique")
	}
	if hash1 == hash2 {
		t.Error("hashes should be unique")
	}
	if len(seed1) != 64 {
		t.Errorf("seed length should be 64, got %d", len(seed1))
	}
	if len(hash1) != 64 {
		t.Errorf("hash length should be 64, got %d", len(hash1))
	}
}

func TestVerifyCommit_Valid(t *testing.T) {
	seed, hash, err := GenerateSeed()
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyCommit(hash, seed) {
		t.Error("valid commit should verify")
	}
}

func TestVerifyCommit_Invalid(t *testing.T) {
	if VerifyCommit("invalidhash", "someseed") {
		t.Error("invalid commit should not verify")
	}
}

func TestSHA256_Deterministic(t *testing.T) {
	result1 := SHA256("test-data")
	result2 := SHA256("test-data")
	if result1 != result2 {
		t.Error("SHA256 should be deterministic")
	}
	if len(result1) != 64 {
		t.Errorf("SHA256 hex length should be 64, got %d", len(result1))
	}
}

func TestGenerateRandom_Range(t *testing.T) {
	val := GenerateRandom("seed", "client", 1, 100)
	if val < 0 || val >= 100 {
		t.Errorf("random value should be in range [0, 100), got %d", val)
	}
}

func TestGenerateRandom_Deterministic(t *testing.T) {
	val1 := GenerateRandom("seed", "client", 1, 100)
	val2 := GenerateRandom("seed", "client", 1, 100)
	if val1 != val2 {
		t.Error("random value should be deterministic")
	}
}

func TestGenerateRandom_DifferentInputs(t *testing.T) {
	val1 := GenerateRandom("seed1", "client", 1, 100)
	val2 := GenerateRandom("seed2", "client", 1, 100)
	if val1 == val2 {
		t.Error("different inputs should produce different outputs")
	}
}

func TestIndexFromHash_Bounds(t *testing.T) {
	hash := SHA256("test-data")
	for totalTickets := 1; totalTickets <= 1000; totalTickets *= 10 {
		idx := IndexFromHash(hash, totalTickets)
		if idx < 0 || idx >= totalTickets {
			t.Errorf("index %d out of bounds for totalTickets %d", idx, totalTickets)
		}
	}
}

func TestGenerateDrawProof_Deterministic(t *testing.T) {
	serverSeed, serverSeedHash, _ := GenerateSeed()
	clientSeed := GenerateClientSeed("raffle-1", 1234567890)
	idx1, hash1, _ := GenerateDrawProof(serverSeed, serverSeedHash, clientSeed, 1, 100)
	idx2, hash2, _ := GenerateDrawProof(serverSeed, serverSeedHash, clientSeed, 1, 100)
	if idx1 != idx2 {
		t.Error("draw proof should be deterministic")
	}
	if hash1 != hash2 {
		t.Error("combined hash should be deterministic")
	}
	if idx1 < 0 || idx1 >= 100 {
		t.Errorf("winning index %d out of bounds", idx1)
	}
}

func TestGenerateDrawProof_DifferentSeeds(t *testing.T) {
	serverSeed1, hash1, _ := GenerateSeed()
	serverSeed2, hash2, _ := GenerateSeed()
	clientSeed := GenerateClientSeed("raffle-1", 1234567890)
	idx1, _, _ := GenerateDrawProof(serverSeed1, hash1, clientSeed, 1, 100)
	idx2, _, _ := GenerateDrawProof(serverSeed2, hash2, clientSeed, 1, 100)
	if idx1 == idx2 {
		t.Error("different server seeds should produce different winning indices")
	}
}

func TestVerifyCommit_EndToEnd(t *testing.T) {
	seed, hash, _ := GenerateSeed()
	if !VerifyCommit(hash, seed) {
		t.Error("commit verification should pass")
	}
	if VerifyCommit("wrongcommit", seed) {
		t.Error("wrong commit should fail verification")
	}
}

func TestGenerateDrawProof(t *testing.T) {
	serverSeed, serverSeedHash, _ := GenerateSeed()
	clientSeed := GenerateClientSeed("raffle-1", 1234567890)
	_, combinedHash, hash := GenerateDrawProof(serverSeed, serverSeedHash, clientSeed, 1, 1000)

	if len(combinedHash) != 64 {
		t.Errorf("combined hash length should be 64, got %d", len(combinedHash))
	}
	if hash != serverSeedHash {
		t.Error("hash should match server seed hash")
	}
}

func TestGenerateClientSeed(t *testing.T) {
	seed1 := GenerateClientSeed("raffle-1", 1000)
	seed2 := GenerateClientSeed("raffle-1", 1000)
	if seed1 != seed2 {
		t.Error("client seeds should be deterministic")
	}
	if len(seed1) != 64 {
		t.Errorf("client seed length should be 64, got %d", len(seed1))
	}
	seed3 := GenerateClientSeed("raffle-2", 1000)
	if seed1 == seed3 {
		t.Error("client seeds should differ for different raffle IDs")
	}
}

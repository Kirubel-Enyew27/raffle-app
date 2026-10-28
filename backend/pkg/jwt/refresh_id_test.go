package jwt

import (
	"testing"
	"time"
)

// Refresh tokens are revocable by id, so two tokens issued in the same second
// must not be identical — otherwise revoking a session would revoke the other
// one too (for example a fresh login right after a logout).
func TestRefreshTokensAreUniquePerIssue(t *testing.T) {
	secret := []byte("test-secret-1234567890")

	first, err := GenerateRefreshTokenHMAC("user-1", "user@example.com", "user", secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to issue first token: %v", err)
	}
	second, err := GenerateRefreshTokenHMAC("user-1", "user@example.com", "user", secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to issue second token: %v", err)
	}

	if first == second {
		t.Fatal("two refresh tokens issued back to back must differ")
	}

	firstClaims, err := ParseRefreshTokenHMAC(first, secret)
	if err != nil {
		t.Fatalf("failed to parse first token: %v", err)
	}
	secondClaims, err := ParseRefreshTokenHMAC(second, secret)
	if err != nil {
		t.Fatalf("failed to parse second token: %v", err)
	}
	if firstClaims.ID == "" || secondClaims.ID == "" {
		t.Fatal("refresh tokens must carry an id so they can be revoked")
	}
	if firstClaims.ID == secondClaims.ID {
		t.Fatal("refresh token ids must be unique")
	}
}

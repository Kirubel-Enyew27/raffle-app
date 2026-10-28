package jwt

import (
	"errors"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("token-test-secret-0123456789abcdef")

func TestAccessTokenRoundTrip(t *testing.T) {
	token, err := GenerateTokenHMAC("user-1", "a@example.com", "user", testSecret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	claims, err := ParseTokenHMAC(token, testSecret)
	if err != nil {
		t.Fatalf("access token should parse: %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "a@example.com" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Fatalf("expected token_type %q, got %q", TokenTypeAccess, claims.TokenType)
	}

	if _, err := ParseRefreshTokenHMAC(token, testSecret); !errors.Is(err, ErrNotRefreshToken) {
		t.Fatalf("access token must not be accepted as a refresh token, got %v", err)
	}
}

func TestRefreshTokenRoundTripAndSeparation(t *testing.T) {
	token, err := GenerateRefreshTokenHMAC("user-1", "a@example.com", "user", testSecret, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	claims, err := ParseRefreshTokenHMAC(token, testSecret)
	if err != nil {
		t.Fatalf("refresh token should parse: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// The whole security model rests on this: a refresh token can never
	// authorise an API request.
	if _, err := ParseTokenHMAC(token, testSecret); !errors.Is(err, ErrRefreshTokenUsedAsAccess) {
		t.Fatalf("refresh token must be rejected as an access token, got %v", err)
	}
}

func TestLegacyTokensWithoutTypeRemainValidAccessTokens(t *testing.T) {
	// Tokens issued before refresh support carry no token_type claim. They must
	// keep working until they expire naturally, otherwise every logged-in user
	// would be signed out by the deployment.
	legacy := Claims{
		UserID: "user-legacy",
		Email:  "legacy@example.com",
		Role:   "user",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, legacy).SignedString(testSecret)
	if err != nil {
		t.Fatalf("failed to sign legacy token: %v", err)
	}

	claims, err := ParseTokenHMAC(token, testSecret)
	if err != nil {
		t.Fatalf("legacy access token should still parse: %v", err)
	}
	if claims.UserID != "user-legacy" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	if _, err := ParseRefreshTokenHMAC(token, testSecret); !errors.Is(err, ErrNotRefreshToken) {
		t.Fatalf("legacy access token must not refresh a session, got %v", err)
	}
}

func TestExpiredTokensAreRejected(t *testing.T) {
	access, err := GenerateTokenHMAC("user-1", "a@example.com", "user", testSecret, -time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired access token: %v", err)
	}
	if _, err := ParseTokenHMAC(access, testSecret); err == nil {
		t.Fatal("expected expired access token to be rejected")
	}

	refresh, err := GenerateRefreshTokenHMAC("user-1", "a@example.com", "user", testSecret, -time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired refresh token: %v", err)
	}
	if _, err := ParseRefreshTokenHMAC(refresh, testSecret); err == nil {
		t.Fatal("expected expired refresh token to be rejected")
	}
}

func TestTokensFromAnotherSecretAreRejected(t *testing.T) {
	other := []byte("a-completely-different-secret-value")

	access, err := GenerateTokenHMAC("user-1", "a@example.com", "user", other, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if _, err := ParseTokenHMAC(access, testSecret); err == nil {
		t.Fatal("expected access token signed with another secret to be rejected")
	}

	refresh, err := GenerateRefreshTokenHMAC("user-1", "a@example.com", "user", other, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}
	if _, err := ParseRefreshTokenHMAC(refresh, testSecret); err == nil {
		t.Fatal("expected refresh token signed with another secret to be rejected")
	}
}

func TestRefreshTokenOutlivesAccessToken(t *testing.T) {
	access, err := GenerateTokenHMAC("user-1", "a@example.com", "user", testSecret, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	refresh, err := GenerateRefreshTokenHMAC("user-1", "a@example.com", "user", testSecret, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	accessClaims, err := ParseTokenHMAC(access, testSecret)
	if err != nil {
		t.Fatalf("access token should parse: %v", err)
	}
	refreshClaims, err := ParseRefreshTokenHMAC(refresh, testSecret)
	if err != nil {
		t.Fatalf("refresh token should parse: %v", err)
	}

	if !refreshClaims.ExpiresAt.After(accessClaims.ExpiresAt.Time) {
		t.Fatal("refresh token must expire after the access token, otherwise a long match still logs the player out")
	}
}

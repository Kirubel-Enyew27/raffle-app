package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token types. Access tokens authorise API calls and expire quickly; refresh
// tokens are only accepted by /auth/refresh and live much longer.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	ErrRefreshTokenUsedAsAccess = errors.New("refresh token cannot be used as an access token")
	ErrNotRefreshToken          = errors.New("not a refresh token")
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	// TokenType is empty on tokens issued before refresh support existed. Those
	// legacy tokens are treated as access tokens so existing sessions keep
	// working until they expire on their own.
	TokenType string `json:"token_type,omitempty"`
	jwt.RegisteredClaims
}

// ─── RSA (legacy, kept for test backward compat) ───

func ParseToken(tokenString string, publicKey []byte) (*Claims, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM(publicKey)
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return key, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token claims")
}

func GenerateToken(userID, email, role string, privateKey []byte, expiry time.Duration) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	if err != nil {
		return "", err
	}
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}

// ─── HMAC (cloud-friendly, uses a single secret) ───

func GenerateTokenHMAC(userID, email, role string, secret []byte, expiry time.Duration) (string, error) {
	return generateHMAC(Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: TokenTypeAccess,
	}, secret, expiry, "")
}

// GenerateRefreshTokenHMAC issues a token that is only accepted by
// ParseRefreshTokenHMAC. It is signed with the same secret but carries a
// distinct token_type claim so it can never authorise API requests.
//
// Every refresh token gets a unique jti. Without one, two tokens minted for the
// same user in the same second would be byte-identical, which breaks
// single-use rotation: revoking one would revoke the other.
func GenerateRefreshTokenHMAC(userID, email, role string, secret []byte, expiry time.Duration) (string, error) {
	id, err := newTokenID()
	if err != nil {
		return "", err
	}
	return generateHMAC(Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: TokenTypeRefresh,
	}, secret, expiry, id)
}

func newTokenID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateHMAC(claims Claims, secret []byte, expiry time.Duration, id string) (string, error) {
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ID:        id,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ParseTokenHMAC(tokenString string, secret []byte) (*Claims, error) {
	claims, err := parseHMAC(tokenString, secret)
	if err != nil {
		return nil, err
	}
	if claims.TokenType == TokenTypeRefresh {
		return nil, ErrRefreshTokenUsedAsAccess
	}
	return claims, nil
}

// ParseRefreshTokenHMAC validates a refresh token. Access tokens (including
// legacy tokens without a type) are rejected.
func ParseRefreshTokenHMAC(tokenString string, secret []byte) (*Claims, error) {
	claims, err := parseHMAC(tokenString, secret)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrNotRefreshToken
	}
	return claims, nil
}

func parseHMAC(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token claims")
}

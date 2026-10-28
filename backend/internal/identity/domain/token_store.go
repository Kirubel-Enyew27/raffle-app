package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// RotatedSession is the credential pair a refresh token was exchanged for. It is
// kept for a short grace window so that two concurrent refreshes (two tabs, or a
// retried request) receive the same pair instead of one of them being rejected
// for presenting an already-rotated token.
type RotatedSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshTokenRecord is the recorded fate of a single refresh token: signed out,
// or already exchanged for another pair. Both states are kept in one record so a
// refresh is decided from a single read — splitting them into separate flags
// leaves a window where a concurrent rotation looks like a revocation and the
// player is thrown out mid-match.
type RefreshTokenRecord struct {
	// Revoked means the token was signed out and must never be exchanged again.
	Revoked bool `json:"revoked,omitempty"`
	// Session is the pair this token was exchanged for, if it was rotated.
	Session *RotatedSession `json:"session,omitempty"`
	// GraceUntil bounds how long a duplicate request may still be handed that
	// session. It is not an expiry for the record: past it the token is simply
	// spent.
	GraceUntil time.Time `json:"grace_until,omitempty"`
}

// Reusable returns the session this token was exchanged for while the grace
// window is still open, otherwise nil. A token that was never exchanged also
// returns nil: there is nothing to hand back, it is simply unused.
func (r *RefreshTokenRecord) Reusable(now time.Time) *RotatedSession {
	if r == nil || r.Revoked || r.Session == nil || !r.GraceUntil.After(now) {
		return nil
	}
	return r.Session
}

// SessionTokenID derives a stable, non-reversible identifier for a refresh
// token. The token's own jti is preferred; tokens issued before refresh support
// existed have none and fall back to a hash of the credential, so they can still
// be revoked. Only this identifier is stored: revoking a token never requires
// keeping the credential itself at rest, and a log line cannot leak it.
func SessionTokenID(rawToken, jwtID string) string {
	if jwtID != "" {
		return jwtID
	}
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// RefreshTokenStore records which refresh tokens can no longer be used, and
// remembers for a moment what a just-rotated token was exchanged for.
//
// Implementations must treat an unreachable backend as an error, never as "not
// revoked": callers translate that into a transient failure so a client keeps
// its session instead of being logged out for an infrastructure blip.
type RefreshTokenStore interface {
	// Get returns the record for a token, or nil when it has never been used or
	// revoked. It must resolve from a single consistent read.
	Get(ctx context.Context, tokenID string) (*RefreshTokenRecord, error)

	// Revoke marks a token as signed out until it expires.
	Revoke(ctx context.Context, tokenID string, expiresAt time.Time) error

	// RecordExchange revokes a token and stores the pair it was exchanged for,
	// together with the end of the grace window, in a single write.
	RecordExchange(ctx context.Context, tokenID string, expiresAt time.Time, session RotatedSession, graceUntil time.Time) error
}

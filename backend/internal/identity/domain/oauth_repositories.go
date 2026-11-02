package domain

import (
	"context"
	"time"
)

// OAuthIdentityRepository stores provider links. Creation of the user and its
// link must be atomic: a half-created account (a user row with no identity)
// would be unusable and invisible to Google sign-in.
type OAuthIdentityRepository interface {
	// FindByProviderUserID returns the link for a provider subject, or
	// apperrors.ErrNotFound when the account has never signed in that way.
	FindByProviderUserID(ctx context.Context, provider, providerUserID string) (*OAuthIdentity, error)

	// FindByUserID returns the provider link owned by an account, or
	// apperrors.ErrNotFound when the account has not connected that provider.
	FindByUserID(ctx context.Context, provider, userID string) (*OAuthIdentity, error)

	// CreateUserWithIdentity inserts the account and its provider link in one
	// transaction. apperrors.ErrConflict means the email or the provider
	// subject was taken concurrently.
	CreateUserWithIdentity(ctx context.Context, user *User, identity *OAuthIdentity) error

	// Create attaches a provider identity to an existing account.
	// apperrors.ErrConflict means the subject belongs to another account, or
	// this account already has an identity for that provider.
	Create(ctx context.Context, identity *OAuthIdentity) error
}

// OAuthStateStore holds pending authorization attempts, keyed by the `state`
// parameter. Consume must be atomic and single use, so a replayed callback can
// never complete twice.
type OAuthStateStore interface {
	Save(ctx context.Context, state string, attempt OAuthAttempt, ttl time.Duration) error
	// Consume deletes and returns the attempt. It returns (nil, nil) when the
	// state is unknown or already used. An error means the store is
	// unreachable and the flow must fail closed.
	Consume(ctx context.Context, state string) (*OAuthAttempt, error)
}

// LoginHandoffStore holds minted sessions for a few seconds so the browser can
// collect them without the credentials appearing in a redirect URL.
type LoginHandoffStore interface {
	Save(ctx context.Context, code string, handoff LoginHandoff, ttl time.Duration) error
	Consume(ctx context.Context, code string) (*LoginHandoff, error)
}

// GoogleProvider performs the OAuth exchange with Google. Implementations must
// validate the ID token (signature, issuer, audience, expiry, nonce) and never
// expose Google's access or refresh tokens to callers.
type GoogleProvider interface {
	AuthorizationURL(state, nonce, codeChallenge string) string
	Exchange(ctx context.Context, code, codeVerifier, nonce string) (*GoogleProfile, error)
}

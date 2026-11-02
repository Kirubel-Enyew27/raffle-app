package domain

import "time"

// ProviderGoogle is the provider key stored in oauth_identities.provider.
const ProviderGoogle = "google"

// OAuthIdentity links a local account to an external provider identity. The
// provider's immutable subject identifier (Google's `sub`) is the key; email is
// kept for support and conflict reporting only.
type OAuthIdentity struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	ProviderEmail  string    `json:"provider_email,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GoogleProfile is the verified identity extracted from Google's response. It
// is built only from a validated ID token; nothing here comes from the browser.
type GoogleProfile struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// OAuthAttempt is the server-side half of an in-flight OAuth authorization.
// It never leaves Redis: the browser only ever sees the opaque state value.
type OAuthAttempt struct {
	CodeVerifier string `json:"code_verifier"`
	Nonce        string `json:"nonce"`
	// RedirectTo is the internal path to return to once signed in.
	RedirectTo string `json:"redirect_to,omitempty"`
	// LinkUserID is set when the attempt was started from an authenticated
	// session to connect Google to that existing account. It is empty for a
	// normal sign-in. Because the state is created server-side from an already
	// authenticated request, the callback can trust this binding.
	LinkUserID string `json:"link_user_id,omitempty"`
}

// LoginHandoff is a freshly minted session waiting to be collected by the SPA.
// It exists so the callback redirect never carries the tokens themselves.
type LoginHandoff struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         *User  `json:"user"`
	RedirectTo   string `json:"redirect_to,omitempty"`
}

/**
 * Helpers for the "Sign in with Google" flow.
 *
 * The browser never receives a token from the callback redirect. The backend
 * parks the freshly minted session behind a one-time code (valid for 60
 * seconds) and hands that code back in the URL fragment, which browsers do not
 * send to servers. These helpers build the start URL and read that fragment.
 */
import { sanitizeRedirect } from './landing.ts'

export interface GoogleCallbackResult {
  code: string | null
  redirect: string | null
  error: string | null
}

// Redirect sanitising lives with the other post-sign-in navigation rules.
export { sanitizeRedirect }

/** Builds the backend URL that starts the Google authorization-code flow. */
export function buildGoogleStartUrl(apiBase: string, redirect?: string | null): string {
  const base = apiBase.replace(/\/+$/, '')
  const safe = sanitizeRedirect(redirect)
  return safe
    ? `${base}/auth/google?redirect=${encodeURIComponent(safe)}`
    : `${base}/auth/google`
}

/** Reads `#code=...&redirect=...` from the callback redirect. */
export function parseGoogleCallbackHash(hash: string): GoogleCallbackResult {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash
  const params = new URLSearchParams(raw)
  return {
    code: params.get('code'),
    redirect: sanitizeRedirect(params.get('redirect')),
    error: params.get('error'),
  }
}

/**
 * Maps the backend's short reason codes onto translation keys. Everything
 * unknown collapses to a generic failure so a new server-side reason can never
 * leak implementation detail into the UI.
 */
const GOOGLE_ERROR_KEYS: Record<string, string> = {
  access_denied: 'auth.googleCancelled',
  invalid_state: 'auth.googleExpired',
  account_exists: 'auth.googleAccountExists',
  email_unverified: 'auth.googleEmailUnverified',
  google_unavailable: 'auth.googleUnavailable',
  temporarily_unavailable: 'auth.googleUnavailable',
  account_disabled: 'auth.googleAccountDisabled',
  google_failed: 'auth.googleSignInFailed',
}

export function googleErrorKey(reason: string | null | undefined): string {
  if (reason && GOOGLE_ERROR_KEYS[reason]) return GOOGLE_ERROR_KEYS[reason]
  return 'auth.googleSignInFailed'
}

/** Outcomes of the authenticated account-linking flow (`/profile?google=…`). */
const GOOGLE_LINK_ERROR_KEYS: Record<string, string> = {
  already_linked: 'profile.googleLinkAlreadyLinked',
  link_conflict: 'profile.googleLinkConflict',
  link_email_mismatch: 'profile.googleLinkEmailMismatch',
  email_required: 'profile.googleLinkEmailRequired',
  email_unverified: 'profile.googleLinkEmailUnverified',
  access_denied: 'profile.googleLinkCancelled',
  invalid_state: 'profile.googleLinkExpired',
  account_disabled: 'profile.googleLinkDisabled',
  google_unavailable: 'profile.googleUnavailable',
  temporarily_unavailable: 'profile.googleUnavailable',
  google_failed: 'profile.googleLinkFailed',
}

/** Maps a link callback reason onto a translation key. */
export function googleLinkErrorKey(reason: string | null | undefined): string {
  if (reason && GOOGLE_LINK_ERROR_KEYS[reason]) return GOOGLE_LINK_ERROR_KEYS[reason]
  return 'profile.googleLinkFailed'
}

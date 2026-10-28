/**
 * Session storage for the auth token + current user.
 *
 * Production keeps the session in `localStorage` so it survives reloads, new
 * tabs and browser restarts. In development we use `sessionStorage` instead,
 * which is scoped to a single tab: that lets several accounts stay logged in
 * side by side (one per tab) when testing multiplayer features locally.
 *
 * The refresh token lives alongside the access token. It is what keeps an
 * actively playing user signed in when the short-lived access token expires;
 * losing it would force a new login mid-match.
 */
export interface AuthStorage {
  getToken(): string | null
  getRefreshToken(): string | null
  getUser(): string | null
  /**
   * Persist a session. When `refreshToken` is omitted the stored refresh token
   * is left untouched (profile updates re-save the same session).
   */
  save(token: string, user: unknown, refreshToken?: string): void
  /** Replace the access token (and optionally the refresh token) after a refresh. */
  saveTokens(token: string, refreshToken?: string): void
  clear(): void
}

export const TOKEN_KEY = 'token'
export const REFRESH_TOKEN_KEY = 'refresh_token'
export const USER_KEY = 'user'

/** Fired on window after a background token refresh so in-memory state catches up. */
export const AUTH_TOKEN_REFRESHED_EVENT = 'auth:token-refreshed'

export function createAuthStorage(store: Storage): AuthStorage {
  return {
    getToken: () => store.getItem(TOKEN_KEY),
    getRefreshToken: () => store.getItem(REFRESH_TOKEN_KEY),
    getUser: () => store.getItem(USER_KEY),
    save: (token, user, refreshToken) => {
      store.setItem(TOKEN_KEY, token)
      store.setItem(USER_KEY, JSON.stringify(user))
      if (refreshToken) store.setItem(REFRESH_TOKEN_KEY, refreshToken)
    },
    saveTokens: (token, refreshToken) => {
      store.setItem(TOKEN_KEY, token)
      if (refreshToken) store.setItem(REFRESH_TOKEN_KEY, refreshToken)
    },
    clear: () => {
      store.removeItem(TOKEN_KEY)
      store.removeItem(REFRESH_TOKEN_KEY)
      store.removeItem(USER_KEY)
    },
  }
}

/** sessionStorage in dev (per tab), localStorage otherwise (per browser). */
export function pickStore(isDev: boolean, local: Storage, session: Storage): Storage {
  return isDev ? session : local
}

let cachedStore: Storage | null = null

function store(): Storage {
  if (!cachedStore) {
    cachedStore = pickStore(import.meta.env.DEV, window.localStorage, window.sessionStorage)
  }
  return cachedStore
}

export const authStorage: AuthStorage = {
  getToken: () => store().getItem(TOKEN_KEY),
  getRefreshToken: () => store().getItem(REFRESH_TOKEN_KEY),
  getUser: () => store().getItem(USER_KEY),
  save: (token, user, refreshToken) => createAuthStorage(store()).save(token, user, refreshToken),
  saveTokens: (token, refreshToken) => createAuthStorage(store()).saveTokens(token, refreshToken),
  clear: () => {
    const current = store()
    createAuthStorage(current).clear()
    // Also drop a session left behind by the other store (a localStorage token
    // from before the dev switch, or a stale per-tab entry in production).
    createAuthStorage(current === window.sessionStorage ? window.localStorage : window.sessionStorage).clear()
  },
}

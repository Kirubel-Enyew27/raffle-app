/**
 * Session storage for the auth token + current user.
 *
 * Production keeps the session in `localStorage` so it survives reloads, new
 * tabs and browser restarts. In development we use `sessionStorage` instead,
 * which is scoped to a single tab: that lets several accounts stay logged in
 * side by side (one per tab) when testing multiplayer features locally.
 */
export interface AuthStorage {
  getToken(): string | null
  getUser(): string | null
  save(token: string, user: unknown): void
  clear(): void
}

export const TOKEN_KEY = 'token'
export const USER_KEY = 'user'

export function createAuthStorage(store: Storage): AuthStorage {
  return {
    getToken: () => store.getItem(TOKEN_KEY),
    getUser: () => store.getItem(USER_KEY),
    save: (token, user) => {
      store.setItem(TOKEN_KEY, token)
      store.setItem(USER_KEY, JSON.stringify(user))
    },
    clear: () => {
      store.removeItem(TOKEN_KEY)
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
  getUser: () => store().getItem(USER_KEY),
  save: (token, user) => createAuthStorage(store()).save(token, user),
  clear: () => {
    const current = store()
    createAuthStorage(current).clear()
    // Also drop a session left behind by the other store (a localStorage token
    // from before the dev switch, or a stale per-tab entry in production).
    createAuthStorage(current === window.sessionStorage ? window.localStorage : window.sessionStorage).clear()
  },
}

/**
 * Signs a user out on both ends.
 *
 * The refresh token is a long-lived credential, so clearing it from the browser
 * alone is not enough: it would stay replayable until it expires. The server
 * revokes it before the client forgets it.
 *
 * Local credentials are always dropped, immediately and synchronously, whether
 * or not the revocation call succeeds. A network failure must never leave
 * someone half signed out, and an explicit logout must never be blocked by the
 * server being unreachable.
 */
export interface EndSessionDeps {
  storage: {
    getRefreshToken(): string | null
    clear(): void
  }
  /** Best-effort server-side revocation of a refresh token. */
  revoke(refreshToken: string): Promise<unknown>
}

export type EndSession = () => Promise<void>

export function createEndSession({ storage, revoke }: EndSessionDeps): EndSession {
  return async () => {
    const refreshToken = storage.getRefreshToken()
    // Clear first: the UI is signed out even if the request below hangs.
    storage.clear()
    if (!refreshToken) return

    try {
      await revoke(refreshToken)
    } catch {
      // The token expires on its own; being unable to revoke it right now is
      // not a reason to keep the user signed in locally or to surface an error.
    }
  }
}

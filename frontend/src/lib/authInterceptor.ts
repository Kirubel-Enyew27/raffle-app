import type { AxiosError, AxiosInstance, InternalAxiosRequestConfig } from 'axios'

/**
 * Keeps an authenticated session alive across short-lived access tokens.
 *
 * The access token is deliberately short lived, so a 401 usually just means
 * "mint a new one from the refresh token". Only a refresh token that the server
 * rejects (expired/revoked) ends the session. Network errors, timeouts and 5xx
 * responses keep the session intact so a brief blip during a match does not
 * log the player out.
 *
 * All 401s share a single refresh call: concurrent requests never race each
 * other into duplicate refreshes or duplicate logouts.
 */

export interface RefreshedTokens {
  accessToken: string
  refreshToken?: string
}

export interface AuthInterceptorDeps {
  getToken(): string | null
  getRefreshToken(): string | null
  saveTokens(tokens: RefreshedTokens): void
  clearSession(): void
  /** Resolves with a new pair, or rejects. A rejection carrying a 4xx `status` means "session is gone". */
  requestRefresh(refreshToken: string): Promise<RefreshedTokens>
  onSessionExpired?(): void
  onTokenRefreshed?(tokens: RefreshedTokens): void
  /** Paths that must never trigger a refresh (login, register, refresh itself). */
  isAuthEndpoint?(url: string): boolean
}

export class SessionExpiredError extends Error {
  status = 401
  code = 'SESSION_EXPIRED'

  constructor(message = 'Your session has expired. Please sign in again.') {
    super(message)
    this.name = 'SessionExpiredError'
  }
}

type RetriableConfig = InternalAxiosRequestConfig & {
  __authRetried?: boolean
  __authToken?: string | null
}

type StatusError = Error & { status?: number; response?: { status?: number } }

function errorStatus(err: unknown): number | undefined {
  if (typeof err !== 'object' || err === null) return undefined
  const candidate = err as StatusError
  return candidate.status ?? candidate.response?.status
}

/**
 * A 4xx from the refresh endpoint means the refresh token itself is no longer
 * valid. Anything else (no response, timeout, 5xx, malformed success body) is
 * transient and must not end the session.
 */
function isRejectedSession(err: unknown): boolean {
  const status = errorStatus(err)
  return status !== undefined && status >= 400 && status < 500
}

export interface SessionRefresher {
  /** Share one refresh across every caller. Resolves false when the session is gone. */
  refresh(): Promise<boolean>
}

export function createSessionRefresher(deps: AuthInterceptorDeps): SessionRefresher {
  let inFlight: Promise<boolean> | null = null

  const run = async (): Promise<boolean> => {
    const refreshToken = deps.getRefreshToken()
    if (!refreshToken) {
      deps.clearSession()
      deps.onSessionExpired?.()
      return false
    }

    try {
      const tokens = await deps.requestRefresh(refreshToken)
      if (!tokens?.accessToken) {
        // A 200 without a token is a server bug, not proof the session is dead.
        throw new Error('refresh response did not contain an access token')
      }
      deps.saveTokens(tokens)
      deps.onTokenRefreshed?.(tokens)
      return true
    } catch (err) {
      if (isRejectedSession(err)) {
        deps.clearSession()
        deps.onSessionExpired?.()
        return false
      }
      throw err
    }
  }

  return {
    refresh() {
      if (inFlight) return inFlight
      const promise = run().finally(() => {
        if (inFlight === promise) inFlight = null
      })
      inFlight = promise
      return promise
    },
  }
}

function retryWith(instance: AxiosInstance, config: RetriableConfig, token: string) {
  config.__authRetried = true
  config.__authToken = token
  config.headers = config.headers ?? {}
  config.headers.Authorization = `Bearer ${token}`
  return instance.request(config)
}

/**
 * Installs the token header, the transparent refresh-and-retry on 401, and
 * nothing else. Error shaping stays in api.ts.
 */
export function installAuthInterceptors(instance: AxiosInstance, deps: AuthInterceptorDeps): SessionRefresher {
  const refresher = createSessionRefresher(deps)

  instance.interceptors.request.use((config) => {
    const token = deps.getToken()
    const retriable = config as RetriableConfig
    retriable.__authToken = token
    config.headers = config.headers ?? {}
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  })

  instance.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
      const config = error.config as RetriableConfig | undefined
      if (error.response?.status !== 401 || !config) throw error

      const url = config.url ?? ''
      if (deps.isAuthEndpoint?.(url)) throw error

      if (config.__authRetried) {
        // A token minted seconds ago was rejected: the session really is invalid.
        deps.clearSession()
        deps.onSessionExpired?.()
        throw new SessionExpiredError()
      }

      const currentToken = deps.getToken()
      const usedToken = config.__authToken ?? null
      if (currentToken && usedToken && currentToken !== usedToken) {
        // Another request already refreshed; just retry with the newer token.
        return retryWith(instance, config, currentToken)
      }

      try {
        const refreshed = await refresher.refresh()
        if (!refreshed) throw new SessionExpiredError()
      } catch (refreshError) {
        if (refreshError instanceof SessionExpiredError) throw refreshError
        // Transient refresh failure: keep the session and surface the original
        // 401 so callers can retry after connectivity returns.
        throw error
      }

      const token = deps.getToken()
      if (!token) throw new SessionExpiredError()
      return retryWith(instance, config, token)
    },
  )

  return refresher
}

import axios, { type AxiosError } from 'axios'
import { AUTH_TOKEN_REFRESHED_EVENT, authStorage } from '@/lib/authStorage'
import { installAuthInterceptors, type RefreshedTokens } from '@/lib/authInterceptor'

// In production (Vercel), VITE_API_URL points to the Render backend.
// In development, Vite's proxy handles /api requests to localhost:8080.
const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'

export const api = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

export class ApiError extends Error {
  status?: number
  code?: string
  response?: AxiosError['response']

  constructor(message: string, status?: number, code?: string, response?: AxiosError['response']) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.response = response
  }
}

// A raw client for the refresh call itself: it must never pass through the auth
// interceptor, or a failing refresh would recurse forever.
const refreshClient = axios.create({
  baseURL: BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

async function requestRefresh(refreshToken: string): Promise<RefreshedTokens> {
  try {
    const res = await refreshClient.post('/auth/refresh', { refresh_token: refreshToken })
    const data = res.data?.data
    return {
      accessToken: data?.token,
      refreshToken: data?.refresh_token,
    }
  } catch (err) {
    if (axios.isAxiosError(err)) {
      const failure = new Error('failed to refresh the session') as Error & { status?: number }
      const status = err.response?.status
      // No status means a network/timeout failure: transient, not a dead session.
      if (status) failure.status = status
      throw failure
    }
    throw err
  }
}

// One navigation per expired session, even if many requests fail at once.
let sessionExpiryHandled = false

function handleSessionExpiry() {
  if (sessionExpiryHandled || typeof window === 'undefined') return
  sessionExpiryHandled = true

  const { pathname, search, hash } = window.location
  if (pathname === '/login' || pathname.startsWith('/login/')) return

  // Keep the player's place (an active match URL) so signing back in returns
  // them to the same board. The match itself lives on the server.
  const redirect = encodeURIComponent(pathname + search + hash)
  window.location.assign(`/login?reason=session_expired&redirect=${redirect}`)
}

export const sessionRefresher = installAuthInterceptors(api, {
  getToken: () => authStorage.getToken(),
  getRefreshToken: () => authStorage.getRefreshToken(),
  saveTokens: ({ accessToken, refreshToken }) => authStorage.saveTokens(accessToken, refreshToken),
  clearSession: () => authStorage.clear(),
  requestRefresh,
  onSessionExpired: handleSessionExpiry,
  onTokenRefreshed: ({ accessToken }) => {
    sessionExpiryHandled = false
    if (typeof window !== 'undefined') {
      // Let AuthContext (and the realtime stream) pick up the new token.
      window.dispatchEvent(new CustomEvent(AUTH_TOKEN_REFRESHED_EVENT, { detail: { token: accessToken } }))
    }
  },
  isAuthEndpoint: (url) =>
    url.includes('/auth/login') ||
    url.includes('/auth/register') ||
    url.includes('/auth/refresh') ||
    url.includes('/auth/logout'),
})

// Normalise error shape. Authentication failures are handled by the interceptor
// above; this only turns what is left into ApiError.
api.interceptors.response.use(
  (res) => res,
  (err: unknown) => {
    // Refresh/invalid-session errors already carry a clear message; pass them
    // through instead of replacing it with a generic server message.
    if (!axios.isAxiosError(err)) return Promise.reject(err)

    const status = err.response?.status
    const rawData = err.response?.data as
      | { error?: { message?: string; code?: string }; message?: string; code?: string }
      | undefined
    const serverMessage =
      typeof rawData === 'object' && rawData !== null
        ? rawData.message || rawData.error?.message
        : undefined
    const serverCode =
      typeof rawData === 'object' && rawData !== null
        ? rawData.code || rawData.error?.code
        : undefined

    let message = serverMessage
    if (!message) {
      if (status === 404) {
        message = 'The requested resource was not found'
      } else if (err.code === 'ERR_NETWORK' || !err.response) {
        message = 'Unable to connect to the server. Please check your internet connection.'
      } else {
        message = err.message || 'An unexpected error occurred'
      }
    }

    const apiError = new ApiError(message, status, serverCode, err.response)
    return Promise.reject(apiError)
  },
)

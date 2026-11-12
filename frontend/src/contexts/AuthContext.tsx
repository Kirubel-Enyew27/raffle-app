import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { authApi } from '@/features/auth/api'
import { AUTH_TOKEN_REFRESHED_EVENT, authStorage } from '@/lib/authStorage'
import { createEndSession } from '@/lib/endSession'
import { hasTelegramLaunchData, telegramLoginOnce } from '@/lib/telegramAuth'
import type { LoginResponse, User } from '@/types/api'

// Revokes the refresh token on the server, then forgets it locally.
const endSession = createEndSession({
  storage: authStorage,
  revoke: (refreshToken) => authApi.logout(refreshToken),
})

interface AuthState {
  user: User | null
  token: string | null
  isLoading: boolean
}

interface AuthContextValue extends AuthState {
  login: (token: string, user: User, refreshToken?: string) => void
  /**
   * Exchanges the raw Telegram launch payload for a KWIK session. Used for the
   * automatic sign-in and as an explicit retry from the sign-in screen.
   */
  loginWithTelegram: () => Promise<void>
  /** Why automatic Telegram sign-in failed, if it did. */
  telegramError: string | null
  logout: () => void
  isAuthenticated: boolean
  isAdmin: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

/** Save a session exactly like a normal login. */
function persistSession(session: LoginResponse): AuthState {
  authStorage.save(session.token, session.user, session.refresh_token)
  return { token: session.token, user: session.user, isLoading: false }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    token: authStorage.getToken(),
    isLoading: true,
  })
  const [telegramError, setTelegramError] = useState<string | null>(null)

  // Hydrate the session on mount. A stored KWIK session wins; otherwise, when
  // the page was launched by Telegram, the signed initData is exchanged for a
  // normal KWIK session through the existing backend. Browser visitors who are
  // signed out stay signed out and see the login form.
  useEffect(() => {
    let cancelled = false

    const hydrate = async () => {
      const token = authStorage.getToken()
      const raw = authStorage.getUser()

      if (token && raw) {
        try {
          const user = JSON.parse(raw) as User
          if (user?.id) {
            setState({ token, user, isLoading: false })
            return
          }
        } catch {
          // Corrupt stored session: fall through and drop it.
        }
        authStorage.clear()
      }

      if (!hasTelegramLaunchData()) {
        setState({ token: null, user: null, isLoading: false })
        return
      }

      try {
        const session = await telegramLoginOnce()
        if (cancelled) return
        if (!session) {
          setState({ token: null, user: null, isLoading: false })
          return
        }
        setState(persistSession(session))
        setTelegramError(null)
      } catch (error) {
        if (cancelled) return
        setTelegramError(
          error instanceof Error ? error.message : 'Telegram sign-in failed',
        )
        setState({ token: null, user: null, isLoading: false })
      }
    }

    void hydrate()
    return () => {
      cancelled = true
    }
  }, [])

  // The API client renews the access token in the background when it expires.
  // Reflect the new token here so token-dependent consumers (the realtime event
  // stream) reconnect instead of staying stuck on an expired token.
  useEffect(() => {
    if (typeof window === 'undefined') return
    const onRefreshed = (event: Event) => {
      const detail = (event as CustomEvent<{ token?: string }>).detail
      const token = detail?.token || authStorage.getToken()
      if (!token) return
      setState(prev => ({ ...prev, token, isLoading: false }))
    }
    window.addEventListener(AUTH_TOKEN_REFRESHED_EVENT, onRefreshed)
    return () => window.removeEventListener(AUTH_TOKEN_REFRESHED_EVENT, onRefreshed)
  }, [])

  const login = (token: string, user: User, refreshToken?: string) => {
    // Profile edits re-save the existing session without a new refresh token;
    // keep the stored one so the session can still renew itself.
    authStorage.save(token, user, refreshToken)
    setTelegramError(null)
    setState({ token, user, isLoading: false })
  }

  const loginWithTelegram = async () => {
    setTelegramError(null)
    setState(prev => ({ ...prev, isLoading: true }))
    try {
      const session = await telegramLoginOnce()
      if (!session) {
        setState({ token: null, user: null, isLoading: false })
        return
      }
      setState(persistSession(session))
    } catch (error) {
      setTelegramError(error instanceof Error ? error.message : 'Telegram sign-in failed')
      setState({ token: null, user: null, isLoading: false })
    }
  }

  const logout = () => {
    // Clears local credentials synchronously; the server-side revocation is
    // best-effort so signing out works offline or during an outage.
    void endSession()
    setState({ token: null, user: null, isLoading: false })
  }

  return (
    <AuthContext.Provider
      value={{
        ...state,
        login,
        loginWithTelegram,
        telegramError,
        logout,
        isAuthenticated: !!state.token && !!state.user,
        isAdmin: state.user?.role === 'admin',
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { authApi } from '@/features/auth/api'
import { AUTH_TOKEN_REFRESHED_EVENT, authStorage } from '@/lib/authStorage'
import { createEndSession } from '@/lib/endSession'
import type { User } from '@/types/api'

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
  logout: () => void
  isAuthenticated: boolean
  isAdmin: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    token: authStorage.getToken(),
    isLoading: true,
  })

  // Hydrate user from storage on mount
  useEffect(() => {
    const token = authStorage.getToken()
    const raw = authStorage.getUser()
    const user: User | null = raw ? (JSON.parse(raw) as User) : null
    setState({ token, user, isLoading: false })
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
    setState({ token, user, isLoading: false })
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

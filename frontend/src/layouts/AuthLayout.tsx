import { Navigate, Outlet, useSearchParams } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { resolvePostAuthPath } from '@/features/auth/landing'

/** Redirects authenticated users away from login/register. */
export function AuthLayout() {
  const { isAuthenticated, user, isLoading } = useAuth()
  const [searchParams] = useSearchParams()
  if (isLoading) return null
  if (isAuthenticated) {
    // Signing in flips the auth state, so this layout can redirect before the
    // form's own navigation runs. Follow the `?redirect=` the visitor arrived
    // with (for example an invite link bounced here by the route guard) instead
    // of dropping them on the role's landing page and losing the invitation.
    const redirect = resolvePostAuthPath({
      redirect: searchParams.get('redirect'),
      role: user?.role,
    })
    return <Navigate to={redirect} replace />
  }
  return (
    <div className="kwik-auth-screen flex min-h-screen items-center justify-center bg-muted/40">
      <Outlet />
    </div>
  )
}

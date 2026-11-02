import { Navigate } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { landingPathForRole } from '@/features/auth/landing'

/**
 * The root URL is not a page of its own: signed-in users continue to their
 * role's landing page (games for players, the admin dashboard for admins) and
 * visitors who are not signed in go to the sign-in form. The session is read
 * from the authentication context, which hydrates from storage before the first
 * decision, so nobody flashes the wrong screen.
 */
export function Component() {
  const { isAuthenticated, user, isLoading } = useAuth()
  if (isLoading) return null
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <Navigate to={landingPathForRole(user?.role)} replace />
}

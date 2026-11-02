import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'
import { landingPathForRole } from '@/features/auth/landing'

/** Redirects authenticated users away from login/register. */
export function AuthLayout() {
  const { isAuthenticated, user, isLoading } = useAuth()
  if (isLoading) return null
  if (isAuthenticated) return <Navigate to={landingPathForRole(user?.role)} replace />
  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/40">
      <Outlet />
    </div>
  )
}

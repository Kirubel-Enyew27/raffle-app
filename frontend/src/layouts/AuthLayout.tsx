import { Navigate, Outlet } from 'react-router-dom'
import { useAuth } from '@/contexts/AuthContext'

/** Redirects authenticated users away from login/register. */
export function AuthLayout() {
  const { isAuthenticated, isAdmin, isLoading } = useAuth()
  if (isLoading) return null
  if (isAuthenticated) return <Navigate to={isAdmin ? '/admin/dashboard' : '/dashboard'} replace />
  return (
    <div className="flex min-h-screen items-center justify-center bg-muted/40">
      <Outlet />
    </div>
  )
}

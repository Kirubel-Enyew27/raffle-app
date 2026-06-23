import { Navigate, Outlet, Link, useNavigate } from 'react-router-dom'
import { LogOut, Bell } from 'lucide-react'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/button'

function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => { logout(); navigate('/login') }

  return (
    <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        <div className="flex items-center gap-6">
          <Link to="/dashboard" className="font-bold tracking-tight">🎟 RaffleApp</Link>
          <nav className="hidden gap-4 text-sm sm:flex">
            <Link to="/raffles" className="text-muted-foreground hover:text-foreground transition-colors">Raffles</Link>
            <Link to="/tickets" className="text-muted-foreground hover:text-foreground transition-colors">My Tickets</Link>
            <Link to="/wallet" className="text-muted-foreground hover:text-foreground transition-colors">Wallet</Link>
            <Link to="/winners" className="text-muted-foreground hover:text-foreground transition-colors">Winners</Link>
          </nav>
        </div>
        <div className="flex items-center gap-2">
          <Link to="/notifications">
            <Button variant="ghost" size="icon" aria-label="Notifications">
              <Bell className="h-4 w-4" />
            </Button>
          </Link>
          <span className="hidden text-sm text-muted-foreground sm:block">{user?.email}</span>
          <Button variant="ghost" size="icon" onClick={handleLogout} aria-label="Sign out">
            <LogOut className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </header>
  )
}

export function AppLayout() {
  const { isAuthenticated, isLoading } = useAuth()
  if (isLoading) return null
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return (
    <div className="flex min-h-screen flex-col">
      <Navbar />
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  )
}

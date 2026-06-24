import { useState } from 'react'
import { Navigate, Outlet, Link, useNavigate } from 'react-router-dom'
import { LogOut, Bell, Menu, X } from 'lucide-react'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/button'

function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)

  const handleLogout = () => { logout(); navigate('/login') }

  const navLinks = [
    { to: '/raffles', label: 'Raffles' },
    { to: '/tickets', label: 'My Tickets' },
    { to: '/wallet', label: 'Wallet' },
    { to: '/winners', label: 'Winners' },
  ]

  return (
    <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        <div className="flex items-center gap-6">
          <Link to="/dashboard" className="font-bold tracking-tight">🎟 RaffleApp</Link>
          <button
            onClick={() => setMenuOpen(!menuOpen)}
            className="sm:hidden p-2 -ml-2 rounded-md hover:bg-accent transition-colors"
            aria-label={menuOpen ? 'Close navigation' : 'Open navigation'}
          >
            {menuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
          <nav className="hidden gap-4 text-sm sm:flex" aria-label="Main navigation">
            {navLinks.map(link => (
              <Link
                key={link.to}
                to={link.to}
                className="text-muted-foreground hover:text-foreground transition-colors"
              >
                {link.label}
              </Link>
            ))}
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
      {/* Mobile drawer */}
      {menuOpen && (
        <div className="border-t sm:hidden">
          <nav className="space-y-1 px-4 py-3" aria-label="Mobile navigation">
            {navLinks.map(link => (
              <Link
                key={link.to}
                to={link.to}
                onClick={() => setMenuOpen(false)}
                className="block rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
      )}
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

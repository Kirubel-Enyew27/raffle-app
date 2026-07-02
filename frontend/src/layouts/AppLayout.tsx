import { useState } from 'react'
import { Navigate, Outlet, Link, useNavigate } from 'react-router-dom'
import { LogOut, Bell, Menu, X, User as UserIcon, Wallet } from 'lucide-react'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/button'
import { useQuery } from '@tanstack/react-query'
import { walletApi } from '@/features/wallet/api'
import { notificationsApi } from '@/features/notifications/api'

function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const { data: wallet } = useQuery({ queryKey: ['wallet'], queryFn: walletApi.balance })
  const { data: unreadData } = useQuery({
    queryKey: ['notifications-unread'],
    queryFn: notificationsApi.unread,
    enabled: !!user,
  })
  const unreadCount = unreadData?.unread || 0

  const handleLogout = () => { logout(); navigate('/login') }

  const navLinks = [
    { to: '/raffles', label: 'Raffles' },
    { to: '/tickets', label: 'My Tickets' },
    { to: '/wallet', label: 'Wallet' },
    { to: '/winners', label: 'Winners' },
    { to: '/profile', label: 'Profile' },
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
          {wallet && (
            <div className="mr-2 hidden items-center gap-1.5 rounded-lg border border-border/80 bg-accent/30 px-3 py-1 text-xs font-semibold text-foreground sm:flex transition-all hover:bg-accent/55">
              <Wallet className="h-3.5 w-3.5 text-primary animate-pulse" />
              <span>{wallet.balance.toLocaleString()} {wallet.currency || 'ETB'}</span>
            </div>
          )}
          <Link to="/notifications" className="relative mr-1">
            <Button variant="ghost" size="icon" aria-label="Notifications" className="relative">
              <Bell className="h-4 w-4" />
            </Button>
            {unreadCount > 0 && (
              <span className="absolute top-1.5 right-1.5 flex h-4.5 w-4.5 items-center justify-center rounded-full bg-destructive text-[9px] font-bold text-destructive-foreground ring-2 ring-background animate-pulse">
                {unreadCount}
              </span>
            )}
          </Link>
          <div className="flex items-center gap-2">
            <div className="hidden h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-muted sm:flex">
              {user?.avatar_url ? (
                <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
              ) : (
                <UserIcon className="h-3.5 w-3.5 text-muted-foreground" />
              )}
            </div>
            <span className="hidden text-sm text-muted-foreground sm:block">{user?.full_name || user?.email}</span>
          </div>
          <Button variant="ghost" size="icon" onClick={handleLogout} aria-label="Sign out">
            <LogOut className="h-4 w-4" />
          </Button>
        </div>
      </div>
      {/* Mobile drawer */}
      {menuOpen && (
        <div className="border-t sm:hidden">
          {wallet && (
            <div className="mx-4 mt-3 flex items-center gap-1.5 rounded-lg border border-border/80 bg-accent/30 px-3 py-2 text-sm font-semibold text-foreground">
              <Wallet className="h-4 w-4 text-primary animate-pulse" />
              <span>{wallet.balance.toLocaleString()} {wallet.currency || 'ETB'}</span>
            </div>
          )}
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

import { useEffect, useState } from 'react'
import { Navigate, Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import {
  Activity, BarChart2, Coins, LayoutDashboard, LogOut, Menu, ScrollText,
  Settings2, Swords, Ticket, Trophy, Users, WalletCards, Gamepad2, X, User as UserIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/contexts/AuthContext'
import { LanguageSelector } from '@/components/ui/LanguageSelector'
import { ThemeSelector } from '@/components/ui/ThemeSelector'
import { cn } from '@/lib/utils'
import { KwikWordmark } from '@/components/brand/KwikWordmark'

type NavItem = { to: string; icon: React.ElementType; label: string }
type NavGroup = { label?: string; items: NavItem[] }

function useAdminNav(): NavGroup[] {
  const { t } = useTranslation()
  return [
    {
      items: [
        { to: '/admin/dashboard', icon: LayoutDashboard, label: t('admin.dashboard', 'Overview') },
        { to: '/admin/users', icon: Users, label: t('admin.users', 'Users') },
        { to: '/admin/live', icon: Activity, label: 'Live Activity' },
        { to: '/admin/players', icon: Trophy, label: 'Top Players' },
      ],
    },
    {
      label: 'Competition',
      items: [
        { to: '/admin/matches', icon: Swords, label: 'Matches' },
        { to: '/admin/games', icon: Gamepad2, label: 'Games Analytics' },
      ],
    },
    {
      label: 'Finance',
      items: [
        { to: '/admin/deposits', icon: WalletCards, label: 'Deposits' },
        { to: '/admin/withdrawals', icon: Coins, label: t('admin.withdrawals', 'Withdrawals') },
      ],
    },
    {
      label: 'Platform',
      items: [
        { to: '/admin/raffles', icon: Ticket, label: t('admin.raffles', 'Raffles') },
        { to: '/admin/winners', icon: Trophy, label: t('admin.winners', 'Winners') },
        { to: '/admin/reports', icon: BarChart2, label: t('admin.reports', 'Reports') },
        { to: '/admin/audit', icon: ScrollText, label: t('admin.auditLog', 'Audit Log') },
      ],
    },
    {
      label: 'Configuration',
      items: [
        { to: '/admin/crash-settings', icon: Settings2, label: 'Crash Settings' },
        { to: '/admin/p2p-settings', icon: Coins, label: 'P2P Earnings' },
      ],
    },
  ]
}

function SidebarContent({ onNavigate }: { onNavigate?: () => void }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const nav = useAdminNav()

  return (
    <div className="flex h-full flex-col bg-card">
      <div className="flex h-16 items-center gap-2.5 border-b border-border/70 px-4">
        <KwikWordmark variant="compact" className="h-9 w-[112px] rounded-md" />
        <span className="rounded-md bg-primary px-1.5 py-0.5 text-[9px] font-black uppercase tracking-[0.18em] text-primary-foreground">
          Admin
        </span>
      </div>

      <nav className="min-h-0 flex-1 overflow-y-auto px-2.5 py-3">
        {nav.map((group, index) => (
          <div key={group.label ?? index} className={cn(index > 0 && 'mt-4')}>
            {group.label && (
              <p className="mb-1 px-2.5 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/70">
                {group.label}
              </p>
            )}
            <div className="space-y-0.5">
              {group.items.map(({ to, icon: Icon, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  onClick={onNavigate}
                  className={({ isActive }) => cn(
                    'group flex items-center gap-3 rounded-lg px-2.5 py-2 text-[13px] font-medium transition-colors',
                    isActive
                      ? 'bg-primary/10 font-semibold text-foreground'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground',
                  )}
                >
                  {({ isActive }) => (
                    <>
                      <span className={cn(
                        'flex h-7 w-7 shrink-0 items-center justify-center rounded-md transition-colors',
                        isActive ? 'bg-primary text-primary-foreground' : 'bg-muted/70 text-muted-foreground group-hover:text-foreground',
                      )}>
                        <Icon className="h-3.5 w-3.5" />
                      </span>
                      <span className="truncate">{label}</span>
                    </>
                  )}
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>

      <div className="border-t border-border/70 p-2.5">
        <div className="mb-2 flex items-center justify-between gap-2 rounded-lg bg-muted/40 px-2.5 py-2">
          <ThemeSelector />
          <LanguageSelector />
        </div>
        <div className="flex items-center gap-2.5 px-2 py-2">
          <span className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-muted">
            {user?.avatar_url
              ? <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
              : <UserIcon className="h-4 w-4 text-muted-foreground" />}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-semibold">{user?.full_name || user?.email || 'Admin'}</p>
            <p className="truncate text-[10px] text-muted-foreground">{user?.email}</p>
          </div>
        </div>
        <button
          onClick={() => {
            logout()
            navigate('/login')
          }}
          className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        >
          <LogOut className="h-4 w-4" />
          {t('nav.signOut', 'Sign out')}
        </button>
      </div>
    </div>
  )
}

function Breadcrumb() {
  const { pathname } = useLocation()
  const nav = useAdminNav()
  const item = nav.flatMap(group => group.items)
    .filter(entry => pathname === entry.to || pathname.startsWith(`${entry.to}/`))
    .sort((a, b) => b.to.length - a.to.length)[0]
  return (
    <div className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
      <span className="hidden sm:inline">KWIK Admin</span>
      <span className="hidden sm:inline">/</span>
      <span className="truncate font-semibold text-foreground">{item?.label ?? 'Overview'}</span>
    </div>
  )
}

export function AdminLayout() {
  const { isAuthenticated, isAdmin, isLoading } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    if (!menuOpen) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = previous }
  }, [menuOpen])

  if (isLoading) return null
  if (!isAuthenticated) return <Navigate to="/login" replace />
  if (!isAdmin) return <Navigate to="/dashboard" replace />

  return (
    <div className="min-h-screen bg-muted/25">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 border-r border-border/70 lg:block">
        <SidebarContent />
      </aside>

      {menuOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            aria-label="Close admin menu"
            className="absolute inset-0 bg-black/55 backdrop-blur-[2px]"
            onClick={() => setMenuOpen(false)}
          />
          <aside className="absolute inset-y-0 left-0 w-[min(84vw,288px)] border-r border-border shadow-2xl">
            <SidebarContent onNavigate={() => setMenuOpen(false)} />
            <button
              onClick={() => setMenuOpen(false)}
              aria-label="Close menu"
              className="absolute right-2 top-2 flex h-9 w-9 items-center justify-center rounded-lg bg-background/80 text-muted-foreground"
            >
              <X className="h-4 w-4" />
            </button>
          </aside>
        </div>
      )}

      <div className="lg:pl-60">
        <header className="sticky top-0 z-20 flex h-14 items-center justify-between gap-3 border-b border-border/70 bg-background/90 px-3 backdrop-blur-md sm:px-5 lg:px-7">
          <div className="flex min-w-0 items-center gap-2">
            <button
              onClick={() => setMenuOpen(true)}
              aria-label="Open admin menu"
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-border bg-card text-foreground lg:hidden"
            >
              <Menu className="h-4 w-4" />
            </button>
            <Breadcrumb />
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden items-center gap-1.5 rounded-full bg-success/10 px-2.5 py-1 text-[10px] font-bold uppercase tracking-wide text-success sm:inline-flex">
              <span className="h-1.5 w-1.5 rounded-full bg-success" />
              Admin access
            </span>
            <span className="hidden text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground sm:inline">
              Play. Compete. Earn.
            </span>
          </div>
        </header>
        <main className="min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

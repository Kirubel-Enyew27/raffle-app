import { useEffect, useState } from 'react'
import { Navigate, Outlet, NavLink, useLocation } from 'react-router-dom'
import {
  Activity, BarChart2, Coins, LayoutDashboard, Menu, ScrollText,
  Settings2, Swords, Ticket, Trophy, Users, WalletCards, Gamepad2, X,
  PanelLeftClose, PanelLeftOpen,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/contexts/AuthContext'
import { UserMenu } from '@/components/ui/UserMenu'
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

function SidebarContent({
  onNavigate,
  isCollapsed = false,
  onToggleCollapse,
}: {
  onNavigate?: () => void
  isCollapsed?: boolean
  onToggleCollapse?: () => void
}) {
  const nav = useAdminNav()
  const { t } = useTranslation()

  return (
    <div className="flex h-full flex-col bg-card">
      <div className={cn(
        'flex h-16 items-center border-b border-border/70 transition-all duration-300',
        isCollapsed ? 'justify-center px-2' : 'justify-between px-3.5',
      )}>
        {!isCollapsed && (
          <div className="flex items-center gap-2 min-w-0 overflow-hidden">
            <KwikWordmark variant="compact" className="h-9 w-[112px] rounded-md shrink-0" />
          </div>
        )}
        {onToggleCollapse && (
          <button
            type="button"
            onClick={onToggleCollapse}
            aria-label={isCollapsed ? t('admin.expandSidebar', 'Expand sidebar') : t('admin.collapseSidebar', 'Collapse sidebar')}
            title={isCollapsed ? t('admin.expandSidebar', 'Expand sidebar') : t('admin.collapseSidebar', 'Collapse sidebar')}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-border/70 bg-card text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {isCollapsed ? (
              <PanelLeftOpen className="h-4 w-4" />
            ) : (
              <PanelLeftClose className="h-4 w-4" />
            )}
          </button>
        )}
      </div>

      <nav className={cn(
        'min-h-0 flex-1 overflow-y-auto py-3 transition-all duration-300',
        isCollapsed ? 'px-2 overflow-x-visible' : 'px-2.5',
      )}>
        {nav.map((group, index) => (
          <div key={group.label ?? index} className={cn(index > 0 && (isCollapsed ? 'mt-2' : 'mt-4'))}>
            {group.label && (
              isCollapsed ? (
                <div className="my-2 border-t border-border/40 mx-1" aria-hidden="true" />
              ) : (
                <p className="mb-1 px-2.5 text-[9px] font-black uppercase tracking-[0.2em] text-muted-foreground/70">
                  {group.label}
                </p>
              )
            )}
            <div className="space-y-0.5">
              {group.items.map(({ to, icon: Icon, label }) => (
                <NavLink
                  key={to}
                  to={to}
                  onClick={onNavigate}
                  title={isCollapsed ? label : undefined}
                  aria-label={label}
                  className={({ isActive }) => cn(
                    'group relative flex items-center rounded-lg text-[13px] font-medium transition-colors',
                    isCollapsed ? 'justify-center p-2' : 'gap-3 px-2.5 py-2',
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
                      {!isCollapsed && <span className="truncate">{label}</span>}
                      {isCollapsed && (
                        <div
                          role="tooltip"
                          className="pointer-events-none absolute left-full top-1/2 ml-2 -translate-y-1/2 z-50 hidden rounded-md border border-border bg-popover px-2.5 py-1 text-xs font-semibold text-popover-foreground shadow-md group-hover:block whitespace-nowrap animate-in fade-in-0 zoom-in-95"
                        >
                          {label}
                        </div>
                      )}
                    </>
                  )}
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>
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
  const [isCollapsed, setIsCollapsed] = useState<boolean>(() => {
    try {
      return localStorage.getItem('kwik_admin_sidebar_collapsed') === 'true'
    } catch {
      return false
    }
  })

  const toggleCollapse = () => {
    setIsCollapsed(prev => {
      const next = !prev
      try {
        localStorage.setItem('kwik_admin_sidebar_collapsed', String(next))
      } catch {
        // ignore storage errors
      }
      return next
    })
  }

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
      <aside className={cn(
        'fixed inset-y-0 left-0 z-30 hidden border-r border-border/70 bg-card lg:block transition-all duration-300 ease-in-out',
        isCollapsed ? 'w-16 overflow-visible' : 'w-60 overflow-hidden',
      )}>
        <SidebarContent
          isCollapsed={isCollapsed}
          onToggleCollapse={toggleCollapse}
        />
      </aside>

      {menuOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            aria-label="Close admin menu"
            className="absolute inset-0 bg-black/55 backdrop-blur-[2px]"
            onClick={() => setMenuOpen(false)}
          />
          <aside className="absolute inset-y-0 left-0 w-[min(84vw,288px)] border-r border-border shadow-2xl">
            <SidebarContent
              isCollapsed={false}
              onNavigate={() => setMenuOpen(false)}
              onToggleCollapse={() => setMenuOpen(false)}
            />
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

      <div className={cn(
        'transition-all duration-300 ease-in-out',
        isCollapsed ? 'lg:pl-16' : 'lg:pl-60',
      )}>
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
            <span className="hidden text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground md:inline">
              Play. Compete. Earn.
            </span>
            <UserMenu />
          </div>
        </header>
        <main className="min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

import { useState, useRef, useEffect } from 'react'
import { Navigate, Outlet, Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { Bell, Menu, X, User as UserIcon, Wallet, ChevronDown, Gamepad2, Trophy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/contexts/AuthContext'
import { Button } from '@/components/ui/button'
import { useQuery } from '@tanstack/react-query'
import { walletApi } from '@/features/wallet/api'
import { notificationsApi } from '@/features/notifications/api'
import { extractStartInviteCode } from '@/lib/telegram'
import { KwikWordmark } from '@/components/brand/KwikWordmark'
import { UserMenu } from '@/components/ui/UserMenu'

/**
 * Section groups for the navbar. Pages that only make sense as part of a
 * section hang off its menu instead of sitting as separate top-level entries:
 * the raffle section owns the raffle list, the tickets a user holds and the
 * prizes a user has won; the games section owns the match lobby, crash and the
 * leaderboard.
 */
function useSectionNav() {
  const { t } = useTranslation()
  const raffleItems = [
    { to: '/raffles', label: t('raffles.allRaffles') },
    { to: '/tickets', label: t('nav.myTickets') },
    { to: '/winners', label: t('nav.winners') },
  ]
  const gameItems = [
    { to: '/games', label: t('nav.allGames') },
    // Crash is hidden from the Games menu for now; the /crash route is still live.
    // { to: '/crash', label: t('nav.crash') },
    { to: '/leaderboard', label: t('nav.leaderboard') },
  ]
  return { raffleItems, gameItems }
}

/** Desktop dropdown for a nav section (click to open; Escape / outside click closes) */
function NavGroup({ label, items }: { label: string; items: { to: string; label: string }[] }) {
  const location = useLocation()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const inSection = items.some(
    i => location.pathname === i.to || location.pathname.startsWith(`${i.to}/`),
  )

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        className={[
          'flex items-center gap-1.5 transition-colors',
          inSection ? 'text-foreground font-bold' : 'text-muted-foreground hover:text-foreground',
        ].join(' ')}
      >
        {inSection && <span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-primary" />}
        {label}
        <ChevronDown className={`h-3.5 w-3.5 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div
          role="menu"
          aria-label={label}
          className="absolute left-0 top-full z-50 w-44 rounded-lg border border-border bg-popover py-1 text-popover-foreground shadow-lg"
        >
          {items.map(item => (
            <NavLink
              key={item.to}
              to={item.to}
              role="menuitem"
              onClick={() => setOpen(false)}
              className={({ isActive }) => [
                'block px-3 py-1.5 text-sm transition-colors',
                isActive
                  ? 'bg-primary/10 font-semibold text-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground',
              ].join(' ')}
            >
              {item.label}
            </NavLink>
          ))}
        </div>
      )}
    </div>
  )
}

function Navbar() {
  const { user } = useAuth()
  const { t } = useTranslation()
  const [menuOpen, setMenuOpen] = useState(false)
  const { raffleItems, gameItems } = useSectionNav()
  const { data: wallet } = useQuery({ queryKey: ['wallet'], queryFn: walletApi.balance, staleTime: 0 })
  const { data: unreadData } = useQuery({
    queryKey: ['notifications-unread'],
    queryFn: notificationsApi.unread,
    enabled: !!user,
  })
  const unreadCount = unreadData?.unread || 0

  const sections = [
    { key: 'games',   label: t('nav.games'),   to: '/games',   items: gameItems   },
    { key: 'raffles', label: t('nav.raffles'), to: '/raffles', items: raffleItems },
  ]

  const navLinks = [
    { to: '/wallet',  label: t('nav.wallet')  },
    { to: '/profile', label: t('nav.profile') },
  ]

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95 text-foreground backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">

        {/* ── Left: logo + desktop nav ── */}
        <div className="flex items-center gap-6">
          <Link to="/dashboard" className="shrink-0" aria-label="KWIK">
            <KwikWordmark
              variant="compact"
              className="rounded-md border border-border/60"
            />
          </Link>

          {/* Hamburger – mobile only */}
          <button
            onClick={() => setMenuOpen(v => !v)}
            className="sm:hidden -ml-2 rounded-md p-2 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            aria-label={menuOpen ? t('nav.closeMenu') : t('nav.openMenu')}
            aria-expanded={menuOpen}
          >
            {menuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>

          <nav className="hidden gap-4 text-sm sm:flex" aria-label="Main navigation">
            {sections.map(section => (
              <NavGroup key={section.key} label={section.label} items={section.items} />
            ))}
            {navLinks.map(link => (
              <NavLink
                key={link.to}
                to={link.to}
                className={({ isActive }) =>
                  isActive
                    ? 'font-bold text-foreground transition-colors'
                    : 'text-muted-foreground transition-colors hover:text-foreground'
                }
              >
                {link.label}
              </NavLink>
            ))}
          </nav>
        </div>

        {/* ── Right: wallet · bell · user menu ── */}
        <div className="flex items-center gap-1">
          {/* Wallet balance badge */}
          {wallet && (
            <div className="mr-1 hidden items-center gap-1.5 rounded-lg border border-border bg-muted/50 px-2.5 py-1 text-xs font-semibold text-foreground sm:flex">
              <Wallet className="h-3.5 w-3.5 text-primary" />
              <span>{wallet.balance.toLocaleString()} {wallet.currency || t('common.currency')}</span>
            </div>
          )}

          {/* Notification bell */}
          <Link to="/notifications" className="relative">
            <Button
              variant="ghost"
              size="icon"
              aria-label={t('nav.notifications')}
              className="text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              <Bell className="h-4 w-4" />
            </Button>
            {unreadCount > 0 && (
              <span className="absolute top-1.5 right-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-primary text-[9px] font-bold text-primary-foreground ring-2 ring-background animate-pulse pointer-events-none">
                {unreadCount}
              </span>
            )}
          </Link>

          {/* User dropdown (avatar + name + chevron) */}
          <UserMenu />
        </div>
      </div>

      {/* ── Mobile drawer ── */}
      {menuOpen && (
        <div className="border-t border-border sm:hidden">
          {wallet && (
            <div className="mx-4 mt-3 flex items-center gap-1.5 rounded-lg border border-border bg-muted/50 px-3 py-2 text-sm font-semibold">
              <Wallet className="h-4 w-4 text-primary" />
              <span>{wallet.balance.toLocaleString()} {wallet.currency || t('common.currency')}</span>
            </div>
          )}

          <nav className="space-y-1 px-4 py-3" aria-label="Mobile navigation">
            {sections.map(section => (
              <div key={section.key} className="space-y-1">
                <NavLink
                  to={section.to}
                  onClick={() => setMenuOpen(false)}
                  className={({ isActive }) =>
                    isActive
                      ? 'block rounded-md px-3 py-2 text-sm bg-primary/10 text-foreground font-bold'
                      : 'block rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-foreground'
                  }
                >
                  {section.label}
                </NavLink>
                <div className="ml-3 space-y-1 border-l border-border pl-2">
                  {section.items
                    .filter(item => item.to !== section.to)
                    .map(item => (
                      <NavLink
                        key={item.to}
                        to={item.to}
                        onClick={() => setMenuOpen(false)}
                        className={({ isActive }) =>
                          isActive
                            ? 'block rounded-md px-3 py-1.5 text-sm bg-primary/10 text-foreground font-semibold'
                            : 'block rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-foreground'
                        }
                      >
                        {item.label}
                      </NavLink>
                    ))}
                </div>
              </div>
            ))}

            {navLinks.map(link => (
              <NavLink
                key={link.to}
                to={link.to}
                onClick={() => setMenuOpen(false)}
                className={({ isActive }) =>
                  isActive
                    ? 'block rounded-md px-3 py-2 text-sm bg-accent text-foreground font-bold'
                    : 'block rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-foreground'
                }
              >
                {link.label}
              </NavLink>
            ))}
          </nav>
        </div>
      )}
    </header>
  )
}

/**
 * Mobile-first primary navigation. Telegram Mini App users get four
 * thumb-reachable destinations pinned to the bottom of the screen; desktop
 * keeps the header navigation only.
 */
function MobileTabBar() {
  const { t } = useTranslation()
  const tabs = [
    { to: '/games', label: t('nav.play'), icon: Gamepad2 },
    { to: '/leaderboard', label: t('nav.ranks'), icon: Trophy },
    { to: '/wallet', label: t('nav.wallet'), icon: Wallet },
    { to: '/profile', label: t('nav.profile'), icon: UserIcon },
  ]

  return (
    <nav
      aria-label="Primary"
      className="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur sm:hidden"
    >
      <div className="mx-auto flex max-w-md items-stretch justify-between px-2">
        {tabs.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              [
                'flex flex-1 flex-col items-center gap-1 py-2.5 text-[10px] font-semibold transition-colors',
                isActive ? 'text-primary' : 'text-muted-foreground hover:text-foreground',
              ].join(' ')
            }
          >
            {({ isActive }) => (
              <>
                <Icon className="h-5 w-5" strokeWidth={isActive ? 2.4 : 1.8} />
                <span className="truncate">{label}</span>
              </>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  )
}

export function AppLayout() {
  const { isAuthenticated, isLoading } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    if (!isAuthenticated) return
    const code = extractStartInviteCode()
    if (code && !location.pathname.startsWith('/games')) {
      navigate(`/games?join=${code}`, { replace: true })
    }
  }, [isAuthenticated, location.pathname, navigate])

  if (isLoading) return null
  if (!isAuthenticated) {
    const code = extractStartInviteCode()
    const redirect = code ? `/games?join=${code}` : location.pathname + location.search
    return <Navigate to={`/login?redirect=${encodeURIComponent(redirect)}`} replace />
  }
  return (
    <div className="flex min-h-screen flex-col">
      <Navbar />
      <main className="flex-1 pb-20 sm:pb-0">
        <Outlet />
      </main>
      <MobileTabBar />
    </div>
  )
}

import { useState, useRef, useEffect } from 'react'
import { Navigate, Outlet, Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { LogOut, Bell, Menu, X, User as UserIcon, Wallet, Sun, Moon, Monitor, Globe, ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/contexts/AuthContext'
import { useTheme } from '@/contexts/ThemeContext'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { useQuery } from '@tanstack/react-query'
import { walletApi } from '@/features/wallet/api'
import { notificationsApi } from '@/features/notifications/api'

type Theme = 'light' | 'dark' | 'system'

const languages = [
  { code: 'en', nativeName: 'English' },
  { code: 'am', nativeName: 'አማርኛ' },
]

/** Inline theme toggle – three icon buttons, no text labels, compact */
function ThemeToggle() {
  const { theme, setTheme } = useTheme()
  const { t } = useTranslation()

  const options: { value: Theme; icon: React.ReactNode; label: string }[] = [
    { value: 'light',  icon: <Sun     className="h-3.5 w-3.5" />, label: t('theme.light')  },
    { value: 'dark',   icon: <Moon    className="h-3.5 w-3.5" />, label: t('theme.dark')   },
    { value: 'system', icon: <Monitor className="h-3.5 w-3.5" />, label: t('theme.system') },
  ]

  return (
    <div
      role="group"
      aria-label={t('theme.title')}
      className="flex items-center rounded-md border border-border bg-muted/50 p-0.5 gap-0.5"
    >
      {options.map(({ value, icon, label }) => (
        <button
          key={value}
          type="button"
          onClick={() => setTheme(value)}
          aria-label={label}
          aria-pressed={theme === value}
          title={label}
          className={[
            'flex h-6 w-6 items-center justify-center rounded transition-all',
            theme === value
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground',
          ].join(' ')}
        >
          {icon}
        </button>
      ))}
    </div>
  )
}

/** User dropdown: avatar + name → panel with theme, language, logout */
function UserMenu() {
  const { user, logout } = useAuth()
  const { i18n, t } = useTranslation()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  // Close on Escape
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open])

  const handleLogout = () => { setOpen(false); logout(); navigate('/login') }
  const handleLangChange = (e: React.ChangeEvent<HTMLSelectElement>) => i18n.changeLanguage(e.target.value)

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        aria-haspopup="true"
        aria-expanded={open}
        className="flex items-center gap-1.5 rounded-md px-2 py-1 text-sm text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
      >
        <div className="flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-muted">
          {user?.avatar_url
            ? <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
            : <UserIcon className="h-3 w-3 text-muted-foreground" />}
        </div>
        <span className="hidden max-w-[120px] truncate sm:block">{user?.full_name || user?.email}</span>
        <ChevronDown className={`h-3.5 w-3.5 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1.5 w-60 rounded-lg border border-border bg-popover shadow-lg z-50">
          {/* User info header */}
          <div className="px-3 py-2.5 border-b border-border">
            <p className="text-sm font-medium truncate">{user?.full_name || user?.email}</p>
            {user?.full_name && user?.email && (
              <p className="text-xs text-muted-foreground truncate">{user.email}</p>
            )}
          </div>

          {/* Theme */}
          <div className="px-3 py-2.5 border-b border-border space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{t('theme.title')}</p>
            <ThemeToggle />
          </div>

          {/* Language */}
          <div className="px-3 py-2.5 border-b border-border space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{t('nav.language')}</p>
            <div className="flex items-center gap-2">
              <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <Select
                value={i18n.language.split('-')[0]}
                onChange={handleLangChange}
                className="flex-1"
              >
                {languages.map(l => (
                  <option key={l.code} value={l.code}>{l.nativeName}</option>
                ))}
              </Select>
            </div>
          </div>

          {/* Sign out */}
          <div className="p-1">
            <button
              type="button"
              onClick={handleLogout}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
            >
              <LogOut className="h-3.5 w-3.5" />
              {t('nav.signOut')}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

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
          'flex items-center gap-1 transition-colors',
          inSection ? 'text-foreground font-bold' : 'text-muted-foreground hover:text-foreground',
        ].join(' ')}
      >
        {label}
        <ChevronDown className={`h-3.5 w-3.5 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div
          role="menu"
          aria-label={label}
          className="absolute left-0 top-full z-50 w-44 rounded-lg border border-border bg-popover py-1 shadow-lg"
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
                  ? 'bg-accent font-semibold text-foreground'
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
    { key: 'raffles', label: t('nav.raffles'), to: '/raffles', items: raffleItems },
    { key: 'games',   label: t('nav.games'),   to: '/games',   items: gameItems   },
  ]

  const navLinks = [
    { to: '/wallet',  label: t('nav.wallet')  },
    { to: '/profile', label: t('nav.profile') },
  ]

  return (
    <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">

        {/* ── Left: logo + desktop nav ── */}
        <div className="flex items-center gap-6">
          <Link to="/dashboard" className="font-bold tracking-tight shrink-0">{t('app.title')}</Link>

          {/* Hamburger – mobile only */}
          <button
            onClick={() => setMenuOpen(v => !v)}
            className="sm:hidden p-2 -ml-2 rounded-md hover:bg-accent transition-colors"
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
                    ? 'text-foreground font-bold transition-colors'
                    : 'text-muted-foreground hover:text-foreground transition-colors'
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
            <div className="hidden items-center gap-1.5 rounded-lg border border-border/80 bg-accent/30 px-2.5 py-1 text-xs font-semibold text-foreground sm:flex mr-1">
              <Wallet className="h-3.5 w-3.5 text-primary" />
              <span>{wallet.balance.toLocaleString()} {wallet.currency || t('common.currency')}</span>
            </div>
          )}

          {/* Notification bell */}
          <Link to="/notifications" className="relative">
            <Button variant="ghost" size="icon" aria-label={t('nav.notifications')}>
              <Bell className="h-4 w-4" />
            </Button>
            {unreadCount > 0 && (
              <span className="absolute top-1.5 right-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-destructive text-[9px] font-bold text-destructive-foreground ring-2 ring-background animate-pulse pointer-events-none">
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
        <div className="border-t sm:hidden">
          {wallet && (
            <div className="mx-4 mt-3 flex items-center gap-1.5 rounded-lg border border-border/80 bg-accent/30 px-3 py-2 text-sm font-semibold">
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
                      ? 'block rounded-md px-3 py-2 text-sm bg-accent text-foreground font-bold'
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
                            ? 'block rounded-md px-3 py-1.5 text-sm bg-accent text-foreground font-semibold'
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

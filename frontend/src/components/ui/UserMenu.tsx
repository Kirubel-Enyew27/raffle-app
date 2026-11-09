import { useState, useRef, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { LogOut, User as UserIcon, Sun, Moon, Monitor, Globe, ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/contexts/AuthContext'
import { useTheme } from '@/contexts/ThemeContext'
import { Select } from '@/components/ui/select'

export type Theme = 'light' | 'dark' | 'system'

const languages = [
  { code: 'en', nativeName: 'English' },
  { code: 'am', nativeName: 'አማርኛ' },
]

/** Inline theme toggle – three icon buttons, segmented control appearance */
export function ThemeToggle() {
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
      className="flex items-center rounded-md border border-border bg-muted/60 p-0.5 gap-0.5"
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

/** Account dropdown: avatar + name/email trigger → panel with profile, theme, language, logout */
export function UserMenu() {
  const { user, logout } = useAuth()
  const { i18n, t } = useTranslation()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  // Close on Escape and return focus to trigger
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
        triggerRef.current?.focus()
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open])

  const handleLogout = () => {
    setOpen(false)
    logout()
    navigate('/login')
  }

  const handleLangChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    i18n.changeLanguage(e.target.value)
  }

  const displayName = user?.full_name || user?.email || 'Admin'

  return (
    <div ref={ref} className="relative">
      <button
        ref={triggerRef}
        type="button"
        onClick={() => setOpen(v => !v)}
        aria-haspopup="true"
        aria-expanded={open}
        aria-label={t('nav.accountMenu', 'Account menu')}
        className="flex items-center gap-1.5 rounded-md px-2 py-1 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-muted">
          {user?.avatar_url ? (
            <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
          ) : (
            <UserIcon className="h-3 w-3 text-muted-foreground" />
          )}
        </div>
        <span className="hidden max-w-[120px] truncate sm:block">{displayName}</span>
        <ChevronDown className={`h-3.5 w-3.5 transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && (
        <div
          role="menu"
          aria-label={t('nav.accountMenu', 'Account menu')}
          className="absolute right-0 top-full mt-1.5 z-50 w-60 max-w-[calc(100vw-1.5rem)] rounded-lg border border-border bg-popover text-popover-foreground shadow-lg"
        >
          {/* Profile */}
          <div className="border-b border-border px-3 py-2.5">
            {/* <Link
              to="/profile"
              onClick={() => setOpen(false)}
              className="group block rounded-md transition-colors hover:opacity-85"
            > */}
              <p className="text-sm font-medium truncate group-hover:text-primary">
                {displayName}
              </p>
              {user?.full_name && user?.email && (
                <p className="text-xs text-muted-foreground truncate">{user.email}</p>
              )}
              {/* <div className="mt-2 flex items-center gap-1.5 text-xs font-medium text-muted-foreground group-hover:text-primary">
                <UserIcon className="h-3.5 w-3.5" />
                <span>{t('nav.profile', 'Profile')}</span>
              </div> */}
            {/* </Link> */}
          </div>

          {/* Theme */}
          <div className="border-b border-border px-3 py-2.5 space-y-1.5">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t('theme.title')}
            </p>
            <ThemeToggle />
          </div>

          {/* Language */}
          <div className="border-b border-border px-3 py-2.5 space-y-1.5">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t('nav.language')}
            </p>
            <div className="flex items-center gap-2">
              <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <Select
                value={i18n.language.split('-')[0]}
                onChange={handleLangChange}
                className="flex-1"
              >
                {languages.map(l => (
                  <option key={l.code} value={l.code}>
                    {l.nativeName}
                  </option>
                ))}
              </Select>
            </div>
          </div>

          {/* Sign out */}
          <div className="p-1">
            <button
              type="button"
              onClick={handleLogout}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
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

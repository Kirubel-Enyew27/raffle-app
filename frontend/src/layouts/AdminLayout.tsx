import { Navigate, Outlet, NavLink, useNavigate } from 'react-router-dom'
import { LayoutDashboard, Ticket, Trophy, Users, BarChart2, ScrollText, LogOut, User as UserIcon, Clock, Settings2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/contexts/AuthContext'
import { LanguageSelector } from '@/components/ui/LanguageSelector'
import { ThemeSelector } from '@/components/ui/ThemeSelector'
import { cn } from '@/lib/utils'
import { KwikWordmark } from '@/components/brand/KwikWordmark'

function Sidebar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const nav = [
    { to: '/admin/dashboard',   icon: LayoutDashboard, label: t('admin.dashboard')    },
    { to: '/admin/raffles',     icon: Ticket,          label: t('admin.raffles')      },
    { to: '/admin/winners',     icon: Trophy,          label: t('admin.winners')      },
    { to: '/admin/withdrawals', icon: Clock,           label: t('admin.withdrawals')  },
    { to: '/admin/users',       icon: Users,           label: t('admin.users')        },
    { to: '/admin/reports',     icon: BarChart2,       label: t('admin.reports')      },
    { to: '/admin/audit',          icon: ScrollText,      label: t('admin.auditLog')       },
    { to: '/admin/crash-settings', icon: Settings2,       label: 'Crash Settings'           },
  ]

  return (
    <aside className="dark flex w-56 shrink-0 flex-col border-r border-[#1f2126] bg-[#0a0a0b] text-white">
      <div className="flex h-14 items-center gap-2 border-b border-[#1f2126] px-4">
        <KwikWordmark className="h-6 w-auto" />
        <span className="rounded bg-[#ff5b55]/15 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.16em] text-[#ff6d64]">
          Admin
        </span>
      </div>

      <nav className="flex-1 space-y-0.5 p-2">
        {nav.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                isActive
                  ? 'bg-[#ff5b55]/12 font-semibold text-white'
                  : 'text-[#9ca1a8] hover:bg-white/5 hover:text-white',
              )
            }
          >
            <Icon className="h-4 w-4 shrink-0" />
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="space-y-2 border-t border-[#1f2126] p-2">
        <div className="px-3 py-2">
          <ThemeSelector />
        </div>
        <div className="px-3 py-2">
          <LanguageSelector />
        </div>

        <div className="flex items-center gap-3 px-3 py-2">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-[#2a2c33] bg-[#15161a]">
            {user?.avatar_url
              ? <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
              : <UserIcon className="h-4 w-4 text-[#9ca1a8]" />}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{user?.full_name || user?.email || 'User'}</p>
            {user?.email && user?.full_name && (
              <p className="truncate text-xs text-[#8b9098]">{user.email}</p>
            )}
          </div>
        </div>

        <button
          onClick={() => {
            logout()
            navigate('/login')
          }}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-[#9ca1a8] transition-colors hover:bg-white/5 hover:text-white"
        >
          <LogOut className="h-4 w-4" />
          {t('nav.signOut')}
        </button>
      </div>
    </aside>
  )
}

export function AdminLayout() {
  const { isAuthenticated, isAdmin, isLoading } = useAuth()
  if (isLoading) return null
  if (!isAuthenticated) return <Navigate to="/login" replace />
  if (!isAdmin) return <Navigate to="/dashboard" replace />
  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="flex-1 overflow-auto bg-muted/30">
        <Outlet />
      </main>
    </div>
  )
}

import type { ComponentType } from 'react'
import { createBrowserRouter } from 'react-router-dom'
import { RootLayout } from '@/layouts/RootLayout'
import { AuthLayout } from '@/layouts/AuthLayout'
import { AppLayout } from '@/layouts/AppLayout'
import { AdminLayout } from '@/layouts/AdminLayout'
import { RouteErrorPage } from '@/components/errors/RouteErrorPage'
import { lazyRoute } from './lazyRoute'

/**
 * Route tree:
 *
 * /                     RootLayout → HomeRedirectPage (role-based landing)
 * ├── /login            AuthLayout  → LoginPage   (stub)
 * ├── /register         AuthLayout  → RegisterPage (stub)
 * ├── /dashboard        AppLayout   → DashboardPage
 * ├── /raffles          AppLayout   → RaffleListPage
 * │   └── /:id          AppLayout   → RaffleDetailPage
 * ├── /tickets          AppLayout   → MyTicketsPage
 * ├── /wallet           AppLayout   → WalletPage
 * ├── /notifications    AppLayout   → NotificationsPage
 * ├── /winners          AppLayout   → WinnersPage
 * └── /admin            AdminLayout
 *     ├── /dashboard    AdminLayout → AdminDashboardPage
 *     ├── /raffles      AdminLayout → AdminRafflesPage
 *     ├── /users        AdminLayout → AdminUsersPage
 *     ├── /live         AdminLayout → AdminLivePage
 *     ├── /players      AdminLayout → AdminPlayersPage
 *     ├── /matches      AdminLayout → AdminMatchesPage
 *     ├── /games        AdminLayout → AdminGamesPage
 *     ├── /deposits     AdminLayout → AdminDepositsPage
 *     ├── /reports      AdminLayout → AdminReportsPage
 *     └── /audit        AdminLayout → AdminAuditPage
 *
 * Every lazy page owns its error element. React Router then renders the
 * branded fallback inside that route's layout, so a failed AdminLivePage
 * import does not take down the admin sidebar or unrelated pages.
 */
const page = (
  routeKey: string,
  loader: () => Promise<{ Component: ComponentType }>,
) => ({
  lazy: lazyRoute(routeKey, loader),
  errorElement: <RouteErrorPage />,
})

export const router = createBrowserRouter([
  {
    element: <RootLayout />,
    errorElement: <RouteErrorPage />,
    children: [
      {
        element: <AuthLayout />,
        children: [
          { path: '/login', ...page('/login', () => import('@/pages/auth/LoginPage')) },
          { path: '/register', ...page('/register', () => import('@/pages/auth/RegisterPage')) },
          {
            path: '/forgot-password',
            ...page('/forgot-password', () => import('@/pages/auth/ForgotPasswordPage')),
          },
          {
            path: '/reset-password',
            ...page('/reset-password', () => import('@/pages/auth/ResetPasswordPage')),
          },
        ],
      },
      {
        element: <AppLayout />,
        children: [
          { path: '/dashboard', ...page('/dashboard', () => import('@/pages/app/DashboardPage')) },
          { path: '/raffles', ...page('/raffles', () => import('@/pages/app/RaffleListPage')) },
          {
            path: '/raffles/:id',
            ...page('/raffles/:id', () => import('@/pages/app/RaffleDetailPage')),
          },
          { path: '/games', ...page('/games', () => import('@/pages/app/P2PGamesPage')) },
          {
            path: '/games/match/:id',
            ...page('/games/match/:id', () => import('@/pages/app/P2PMatchPage')),
          },
          {
            path: '/leaderboard',
            ...page('/leaderboard', () => import('@/pages/app/LeaderboardPage')),
          },
          { path: '/tickets', ...page('/tickets', () => import('@/pages/app/MyTicketsPage')) },
          { path: '/profile', ...page('/profile', () => import('@/pages/app/ProfilePage')) },
          { path: '/wallet', ...page('/wallet', () => import('@/pages/app/WalletPage')) },
          {
            path: '/notifications',
            ...page('/notifications', () => import('@/pages/app/NotificationsPage')),
          },
          { path: '/winners', ...page('/winners', () => import('@/pages/app/WinnersPage')) },
        ],
      },
      {
        element: <AdminLayout />,
        children: [
          {
            path: '/admin/dashboard',
            ...page('/admin/dashboard', () => import('@/pages/admin/AdminDashboardPage')),
          },
          {
            path: '/admin/raffles',
            ...page('/admin/raffles', () => import('@/pages/admin/AdminRafflesPage')),
          },
          {
            path: '/admin/winners',
            ...page('/admin/winners', () => import('@/pages/admin/AdminWinnersPage')),
          },
          {
            path: '/admin/live',
            ...page('/admin/live', () => import('@/pages/admin/AdminLivePage')),
          },
          {
            path: '/admin/players',
            ...page('/admin/players', () => import('@/pages/admin/AdminPlayersPage')),
          },
          {
            path: '/admin/matches',
            ...page('/admin/matches', () => import('@/pages/admin/AdminMatchesPage')),
          },
          {
            path: '/admin/games',
            ...page('/admin/games', () => import('@/pages/admin/AdminGamesPage')),
          },
          {
            path: '/admin/deposits',
            ...page('/admin/deposits', () => import('@/pages/admin/AdminDepositsPage')),
          },
          {
            path: '/admin/withdrawals',
            ...page('/admin/withdrawals', () => import('@/pages/admin/AdminWithdrawalsPage')),
          },
          {
            path: '/admin/users',
            ...page('/admin/users', () => import('@/pages/admin/AdminUsersPage')),
          },
          {
            path: '/admin/users/:id',
            ...page('/admin/users/:id', () => import('@/pages/admin/AdminUserDetailPage')),
          },
          {
            path: '/admin/reports',
            ...page('/admin/reports', () => import('@/pages/admin/AdminReportsPage')),
          },
          {
            path: '/admin/audit',
            ...page('/admin/audit', () => import('@/pages/admin/AdminAuditPage')),
          },
          {
            path: '/admin/p2p-settings',
            ...page('/admin/p2p-settings', () => import('@/pages/admin/AdminP2PSettingsPage')),
          },
        ],
      },
      // Lives outside AuthLayout: the callback signs the user in itself and
      // must choose the destination before the layout redirects it away.
      {
        path: '/auth/google/callback',
        ...page('/auth/google/callback', () => import('@/pages/auth/GoogleCallbackPage')),
      },
      // `/` resolves to the role's landing page (games for players, the admin
      // dashboard for admins) or to the sign-in form for visitors.
      { path: '/', ...page('/', () => import('@/pages/HomeRedirectPage')) },
      { path: '*', ...page('*', () => import('@/pages/NotFoundPage')) },
    ],
  },
])

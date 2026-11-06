import { createBrowserRouter } from 'react-router-dom'
import { RootLayout } from '@/layouts/RootLayout'
import { AuthLayout } from '@/layouts/AuthLayout'
import { AppLayout } from '@/layouts/AppLayout'
import { AdminLayout } from '@/layouts/AdminLayout'

/**
 * Route tree:
 *
 * /                     RootLayout → HomeRedirectPage (role-based landing)
 * ├── /login            AuthLayout  → LoginPage   (stub)
 * ├── /register         AuthLayout  → RegisterPage (stub)
 * ├── /dashboard        AppLayout   → DashboardPage
 * ├── /raffles          AppLayout   → RaffleListPage
 * │   └── /:id          AppLayout   → RaffleDetailPage
 * ├── /crash            AppLayout   → CrashGamePage
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
 */
export const router = createBrowserRouter([
  {
    element: <RootLayout />,
    children: [
      {
        element: <AuthLayout />,
        children: [
          { path: '/login', lazy: () => import('@/pages/auth/LoginPage') },
          { path: '/register', lazy: () => import('@/pages/auth/RegisterPage') },
          { path: '/forgot-password', lazy: () => import('@/pages/auth/ForgotPasswordPage') },
          { path: '/reset-password', lazy: () => import('@/pages/auth/ResetPasswordPage') },
        ],
      },
      {
        element: <AppLayout />,
        children: [
          { path: '/dashboard', lazy: () => import('@/pages/app/DashboardPage') },
          { path: '/raffles', lazy: () => import('@/pages/app/RaffleListPage') },
          { path: '/raffles/:id', lazy: () => import('@/pages/app/RaffleDetailPage') },
          { path: '/games', lazy: () => import('@/pages/app/P2PGamesPage') },
          { path: '/games/match/:id', lazy: () => import('@/pages/app/P2PMatchPage') },
          { path: '/leaderboard', lazy: () => import('@/pages/app/LeaderboardPage') },
          { path: '/crash', lazy: () => import('@/pages/app/CrashGamePage') },
          { path: '/tickets', lazy: () => import('@/pages/app/MyTicketsPage') },
          { path: '/profile', lazy: () => import('@/pages/app/ProfilePage') },
          { path: '/wallet', lazy: () => import('@/pages/app/WalletPage') },
          { path: '/notifications', lazy: () => import('@/pages/app/NotificationsPage') },
          { path: '/winners', lazy: () => import('@/pages/app/WinnersPage') },
        ],
      },
      {
        element: <AdminLayout />,
        children: [
          { path: '/admin/dashboard', lazy: () => import('@/pages/admin/AdminDashboardPage') },
          { path: '/admin/raffles', lazy: () => import('@/pages/admin/AdminRafflesPage') },
          { path: '/admin/winners', lazy: () => import('@/pages/admin/AdminWinnersPage') },
          { path: '/admin/live', lazy: () => import('@/pages/admin/AdminLivePage') },
          { path: '/admin/players', lazy: () => import('@/pages/admin/AdminPlayersPage') },
          { path: '/admin/matches', lazy: () => import('@/pages/admin/AdminMatchesPage') },
          { path: '/admin/games', lazy: () => import('@/pages/admin/AdminGamesPage') },
          { path: '/admin/deposits', lazy: () => import('@/pages/admin/AdminDepositsPage') },
          { path: '/admin/withdrawals', lazy: () => import('@/pages/admin/AdminWithdrawalsPage') },
          { path: '/admin/users', lazy: () => import('@/pages/admin/AdminUsersPage') },
          { path: '/admin/users/:id', lazy: () => import('@/pages/admin/AdminUserDetailPage') },
          { path: '/admin/reports', lazy: () => import('@/pages/admin/AdminReportsPage') },
          { path: '/admin/audit', lazy: () => import('@/pages/admin/AdminAuditPage') },
          { path: '/admin/crash-settings', lazy: () => import('@/pages/admin/AdminCrashSettingsPage') },
          { path: '/admin/p2p-settings', lazy: () => import('@/pages/admin/AdminP2PSettingsPage') },
        ],
      },
      // Lives outside AuthLayout: the callback signs the user in itself and
      // must choose the destination before the layout redirects it away.
      { path: '/auth/google/callback', lazy: () => import('@/pages/auth/GoogleCallbackPage') },
      // `/` resolves to the role's landing page (games for players, the admin
      // dashboard for admins) or to the sign-in form for visitors.
      { path: '/', lazy: () => import('@/pages/HomeRedirectPage') },
      { path: '*', lazy: () => import('@/pages/NotFoundPage') },
    ],
  },
])

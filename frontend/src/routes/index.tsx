import { createBrowserRouter } from 'react-router-dom'
import { RootLayout } from '@/layouts/RootLayout'
import { AuthLayout } from '@/layouts/AuthLayout'
import { AppLayout } from '@/layouts/AppLayout'
import { AdminLayout } from '@/layouts/AdminLayout'

/**
 * Route tree:
 *
 * /                     RootLayout
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
          { path: '/tickets', lazy: () => import('@/pages/app/MyTicketsPage') },
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
          { path: '/admin/users', lazy: () => import('@/pages/admin/AdminUsersPage') },
          { path: '/admin/reports', lazy: () => import('@/pages/admin/AdminReportsPage') },
          { path: '/admin/audit', lazy: () => import('@/pages/admin/AdminAuditPage') },
        ],
      },
      { path: '/', lazy: () => import('@/pages/app/DashboardPage') },
      { path: '*', lazy: () => import('@/pages/NotFoundPage') },
    ],
  },
])

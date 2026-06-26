import { usePageTitle } from '@/hooks/usePageTitle'
import { useAuth } from '@/contexts/AuthContext'
import { WalletCard } from '@/features/dashboard/WalletCard'
import { DrawCountdown } from '@/features/dashboard/DrawCountdown'
import { PreviousWinnings } from '@/features/dashboard/PreviousWinnings'
import { ActiveRaffles } from '@/features/dashboard/ActiveRaffles'
import { RecentTransactions } from '@/features/dashboard/RecentTransactions'
import { MyTickets } from '@/features/dashboard/MyTickets'
import { User as UserIcon } from 'lucide-react'

export function Component() {
  usePageTitle('Dashboard')
  const { user } = useAuth()
  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full border-2 border-border bg-muted">
          {user?.avatar_url ? (
            <img src={user.avatar_url} alt="" className="h-full w-full object-cover" />
          ) : (
            <UserIcon className="h-5 w-5 text-muted-foreground" />
          )}
        </div>
        <div>
          <h1 className="text-2xl font-bold">Dashboard</h1>
          <p className="text-sm text-muted-foreground">Welcome back, {user?.full_name || user?.email}</p>
        </div>
      </div>

      {/* stat row */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <WalletCard />
        <DrawCountdown />
        <PreviousWinnings />
      </div>

      {/* main content */}
      <div className="grid gap-4 lg:grid-cols-4">
        <div className="lg:col-span-3"><ActiveRaffles /></div>
        <div className="space-y-4 lg:col-span-2">
          <RecentTransactions />
        </div>
        <div className="lg:col-span-2">
          <MyTickets />
        </div>
      </div>
    </div>
  )
}

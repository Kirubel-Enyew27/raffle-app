import { useAuth } from '@/contexts/AuthContext'
import { WalletCard } from '@/features/dashboard/WalletCard'
import { DrawCountdown } from '@/features/dashboard/DrawCountdown'
import { PreviousWinnings } from '@/features/dashboard/PreviousWinnings'
import { ActiveRaffles } from '@/features/dashboard/ActiveRaffles'
import { RecentTransactions } from '@/features/dashboard/RecentTransactions'
import { MyTickets } from '@/features/dashboard/MyTickets'

export function Component() {
  const { user } = useAuth()
  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <div>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-sm text-muted-foreground">Welcome back, {user?.email}</p>
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

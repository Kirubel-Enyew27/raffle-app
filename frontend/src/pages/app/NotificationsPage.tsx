import { usePageTitle } from '@/hooks/usePageTitle'
import { Bell } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'

export function Component() {
  usePageTitle('Notifications')
  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <h1 className="text-2xl font-bold">Notifications</h1>
      <Card>
        <CardContent className="flex flex-col items-center py-16">
          <Bell className="mb-3 h-12 w-12 text-muted-foreground/30" />
          <p className="text-lg font-medium text-muted-foreground">Coming soon</p>
          <p className="mt-1 text-sm text-muted-foreground/60">
            Real-time notifications for draw results, prize payouts, and more will appear here.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

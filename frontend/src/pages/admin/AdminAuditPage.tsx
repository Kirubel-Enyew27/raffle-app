import { usePageTitle } from '@/hooks/usePageTitle'
import { ScrollText } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'

export function Component() {
  usePageTitle('Admin Audit Log')
  return (
    <div className="p-6 space-y-4">
      <div>
        <h1 className="text-2xl font-bold">Audit Log</h1>
        <p className="text-sm text-muted-foreground">System activity log</p>
      </div>
      <Card>
        <CardContent className="flex flex-col items-center py-16">
          <ScrollText className="mb-3 h-12 w-12 text-muted-foreground/30" />
          <p className="text-lg font-medium text-muted-foreground">Coming soon</p>
          <p className="mt-1 text-sm text-muted-foreground/60">
            A filterable audit trail of all system events will appear here.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

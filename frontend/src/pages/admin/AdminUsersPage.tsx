import { usePageTitle } from '@/hooks/usePageTitle'
import { Users } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'

export function Component() {
  usePageTitle('Admin Users')
  return (
    <div className="p-6 space-y-4">
      <div>
        <h1 className="text-2xl font-bold">Users</h1>
        <p className="text-sm text-muted-foreground">User management</p>
      </div>
      <Card>
        <CardContent className="flex flex-col items-center py-16">
          <Users className="mb-3 h-12 w-12 text-muted-foreground/30" />
          <p className="text-lg font-medium text-muted-foreground">Coming soon</p>
          <p className="mt-1 text-sm text-muted-foreground/60">
            User management with search, role assignment, and status controls will appear here.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

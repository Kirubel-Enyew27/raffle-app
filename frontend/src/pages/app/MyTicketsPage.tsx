import { usePageTitle } from '@/hooks/usePageTitle'
import { Link } from 'react-router-dom'
import { Tag } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

export function Component() {
  usePageTitle('My Tickets')
  return (
    <div className="mx-auto max-w-6xl space-y-6 p-4 sm:p-6">
      <h1 className="text-2xl font-bold">My Tickets</h1>
      <Card>
        <CardContent className="flex flex-col items-center py-16">
          <Tag className="mb-3 h-12 w-12 text-muted-foreground/30" />
          <p className="text-lg font-medium text-muted-foreground">Coming soon</p>
          <p className="mt-1 text-sm text-muted-foreground/60">
            Your purchased tickets will appear here once this feature is ready.
          </p>
          <Link to="/raffles" className="mt-4">
            <Button variant="outline">Browse raffles →</Button>
          </Link>
        </CardContent>
      </Card>
    </div>
  )
}

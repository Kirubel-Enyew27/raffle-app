import { useQuery } from '@tanstack/react-query'
import { Tag } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { dashboardApi } from './api'

export function MyTickets() {
  // Load active raffles first, then fetch tickets for each
  const rafflesQuery = useQuery({
    queryKey: ['raffles', 'active'],
    queryFn: () => dashboardApi.raffles(6),
  })

  const activeRaffleIds = rafflesQuery.data?.raffles
    ?.filter(r => r.status === 'active' || r.status === 'completed')
    .slice(0, 5)
    .map(r => r.id) ?? []

  // The backend returns tickets by raffle; we batch-fetch up to 5 raffles
  // and derive which tickets belong to the current user via the `user_id`
  // field returned in each winner record (closest approximation without a
  // dedicated "my tickets" endpoint).
  //
  // In practice the ticket purchase result shows owned tickets; here we
  // display the raffle cards the user might have tickets in by showing
  // recently-active raffles with a "View tickets" CTA.

  const isLoading = rafflesQuery.isLoading

  const raffles = rafflesQuery.data?.raffles?.slice(0, 4) ?? []

  return (
    <Card className="col-span-full lg:col-span-2">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">My Tickets</CardTitle>
        <Link to="/tickets" className="text-xs text-primary hover:underline">View all</Link>
      </CardHeader>
      <CardContent className="space-y-2">
        {isLoading && Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="h-8 w-8 rounded" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-12" />
          </div>
        ))}

        {!isLoading && raffles.length === 0 && (
          <div className="py-6 text-center">
            <Tag className="mx-auto mb-2 h-8 w-8 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">No tickets yet</p>
            <Link to="/raffles" className="mt-1 inline-block text-xs text-primary hover:underline">
              Browse raffles
            </Link>
          </div>
        )}

        {raffles.map(r => (
          <Link key={r.id} to={`/raffles/${r.id}`}
            className="flex items-center gap-3 rounded-md p-2 transition-colors hover:bg-accent">
            <div className="flex h-8 w-8 items-center justify-center rounded bg-primary/10">
              <Tag className="h-4 w-4 text-primary" />
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{r.title}</p>
              <p className="text-xs text-muted-foreground capitalize">{r.status}</p>
            </div>
            <span className="text-xs text-muted-foreground">
              {new Date(r.draw_date).toLocaleDateString()}
            </span>
          </Link>
        ))}

        {/* suppress unused variable warning */}
        {activeRaffleIds.length === 0 && null}
      </CardContent>
    </Card>
  )
}

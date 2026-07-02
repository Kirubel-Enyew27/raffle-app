import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { notificationsApi } from '@/features/notifications/api'
import { usePageTitle } from '@/hooks/usePageTitle'
import { Bell, Check, Inbox } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

export function Component() {
  usePageTitle('Notifications')
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => notificationsApi.list({ limit: 50 }),
  })

  const markReadMutation = useMutation({
    mutationFn: (id: string) => notificationsApi.markRead(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['notifications-unread'] })
    },
  })

  const items = data?.items || []

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr)
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-4 sm:p-6 animate-fade-in">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Notifications</h1>
        <p className="text-sm text-muted-foreground">Stay updated on your draws, wins, and wallet activities.</p>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((n) => (
            <div key={n} className="h-20 w-full animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      ) : items.length === 0 ? (
        <Card className="border-dashed">
          <CardContent className="flex flex-col items-center py-16 text-center">
            <div className="rounded-full bg-primary/10 p-3 text-primary">
              <Inbox className="h-8 w-8 animate-bounce" />
            </div>
            <h3 className="mt-4 text-lg font-semibold">No notifications</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              You are all caught up! Real-time notifications will show up here.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {items.map((item) => {
            const isRead = !!item.ReadAt
            return (
              <div
                key={item.ID}
                className={`flex items-start gap-4 rounded-xl border p-4 transition-all duration-200 hover:bg-accent/15 ${
                  isRead ? 'bg-card/40 opacity-75 border-border' : 'bg-card border-primary/25 shadow-sm'
                }`}
              >
                <div
                  className={`mt-0.5 rounded-full p-2 ${
                    isRead ? 'bg-muted text-muted-foreground' : 'bg-primary/10 text-primary'
                  }`}
                >
                  {item.Event === 'winner_announcement' ? '🎉' : <Bell className="h-4 w-4" />}
                </div>
                <div className="flex-1 space-y-1">
                  <div className="flex items-center gap-2">
                    <h4 className="font-semibold text-sm leading-none text-foreground">{item.Subject}</h4>
                    {!isRead && (
                      <Badge variant="secondary" className="px-1.5 py-0.2 text-[10px] bg-primary text-primary-foreground">
                        New
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground leading-relaxed">{item.Body}</p>
                  <span className="block text-[10px] text-muted-foreground/60">{formatDate(item.CreatedAt)}</span>
                </div>
                {!isRead && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-8 shrink-0 px-2.5 text-xs font-medium hover:bg-accent/40"
                    onClick={() => markReadMutation.mutate(item.ID)}
                    title="Mark as read"
                  >
                    <Check className="mr-1 h-3.5 w-3.5 text-emerald-500" />
                    Read
                  </Button>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, ShieldCheck, X } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePageTitle } from '@/hooks/usePageTitle'
import { adminApi, type AuditLog } from '@/features/admin/api'
import { DateRangeBar, EmptyState, ErrorState, PageHeader, Pagination } from '@/features/admin/AdminUi'
import { formatDate, formatNumber, shortId } from '@/features/admin/AdminUtils'

const PAGE_SIZE = 25

function pretty(value?: string | null) {
  if (!value) return '—'
  try { return JSON.stringify(JSON.parse(value), null, 2) } catch { return value }
}

function AuditDrawer({ log, onClose }: { log: AuditLog; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <button className="absolute inset-0 bg-black/55 backdrop-blur-[2px]" aria-label="Close audit entry" onClick={onClose} />
      <aside className="relative h-full w-full max-w-lg overflow-y-auto border-l border-border bg-card shadow-2xl">
        <div className="sticky top-0 flex items-center justify-between border-b border-border/70 bg-card/95 px-4 py-4 backdrop-blur">
          <div><p className="text-[10px] font-bold uppercase tracking-[0.2em] text-primary">Audit entry</p><h2 className="text-lg font-black">{log.action}</h2></div>
          <button onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-lg border border-border hover:bg-accent" aria-label="Close"><X className="h-4 w-4" /></button>
        </div>
        <div className="space-y-4 p-4 text-xs">
          <div className="grid grid-cols-2 gap-2">
            {[['Actor type', log.actor_type], ['Resource', log.resource_type], ['IP address', log.ip_address || '—'], ['Created', formatDate(log.created_at, true)]].map(([label, value]) => (
              <div key={label} className="rounded-xl border border-border/60 bg-muted/20 p-3"><p className="text-[9px] uppercase text-muted-foreground">{label}</p><p className="mt-1 break-words font-medium">{value}</p></div>
            ))}
          </div>
          <div className="space-y-2 rounded-xl border border-border/60 p-3">
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Log ID</span><span className="break-all font-mono text-[10px]">{log.id}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Actor ID</span><span className="break-all font-mono text-[10px]">{log.actor_id || '—'}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">Resource ID</span><span className="break-all font-mono text-[10px]">{log.resource_id || '—'}</span></div>
            <div className="flex justify-between gap-3"><span className="text-muted-foreground">User agent</span><span className="max-w-[240px] break-words text-right text-[10px]">{log.user_agent || '—'}</span></div>
          </div>
          <div><p className="mb-1.5 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">Previous value</p><pre className="max-h-56 overflow-auto rounded-xl bg-muted/40 p-3 text-[10px] leading-relaxed">{pretty(log.old_value)}</pre></div>
          <div><p className="mb-1.5 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">New value</p><pre className="max-h-56 overflow-auto rounded-xl bg-muted/40 p-3 text-[10px] leading-relaxed">{pretty(log.new_value)}</pre></div>
        </div>
      </aside>
    </div>
  )
}

export function Component() {
  usePageTitle('Audit Log · KWIK Admin')
  const [action, setAction] = useState('')
  const [resource, setResource] = useState('')
  const [actor, setActor] = useState('')
  const [query, setQuery] = useState({ action: '', resource: '', actor: '' })
  const [range, setRange] = useState({ from: '', to: '' })
  const [offset, setOffset] = useState(0)
  const [selected, setSelected] = useState<AuditLog | null>(null)
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'audit', query, range, offset],
    queryFn: () => adminApi.audit({ action: query.action || undefined, resource_type: query.resource || undefined, user_id: query.actor || undefined, from: range.from || undefined, to: range.to || undefined, limit: PAGE_SIZE, offset }),
  })

  return (
    <div className="mx-auto w-full max-w-[1450px] space-y-5 p-3 sm:p-5 lg:p-7">
      <PageHeader
        eyebrow="Trust and traceability"
        title="Audit log"
        description="Administrator-only record of sensitive platform actions, with actor, resource, and before/after values."
        actions={<span className="rounded-full border border-border bg-card px-3 py-1.5 text-xs font-semibold">{formatNumber(data?.total ?? 0)} entries</span>}
      />
      <Card className="border-border/70 p-3 sm:p-4">
        <form onSubmit={event => { event.preventDefault(); setOffset(0); setQuery({ action: action.trim(), resource: resource.trim(), actor: actor.trim() }) }} className="grid gap-2 sm:grid-cols-[1fr_1fr_1fr_auto]">
          <div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input value={action} onChange={event => setAction(event.target.value)} placeholder="Action, e.g. withdrawal.approve" className="h-10 pl-9" /></div>
          <Input value={resource} onChange={event => setResource(event.target.value)} placeholder="Resource type" className="h-10" />
          <Input value={actor} onChange={event => setActor(event.target.value)} placeholder="Actor user ID" className="h-10" />
          <Button type="submit" variant="secondary" className="h-10">Apply</Button>
        </form>
        <div className="mt-2 flex flex-wrap gap-2">
          <DateRangeBar from={range.from} to={range.to} onChange={(from, to) => { setRange({ from, to }); setOffset(0) }} />
          {(query.action || query.resource || query.actor || range.from || range.to) && <Button variant="ghost" size="sm" onClick={() => { setAction(''); setResource(''); setActor(''); setQuery({ action: '', resource: '', actor: '' }); setRange({ from: '', to: '' }); setOffset(0) }}>Clear</Button>}
        </div>
      </Card>

      <Card className="overflow-hidden border-border/70">
        {isError ? <ErrorState message={(error as Error)?.message} onRetry={() => refetch()} /> : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full min-w-[900px] text-left text-sm">
                <thead className="border-b border-border/60 bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <tr><th className="px-4 py-3 font-semibold">Action</th><th className="px-4 py-3 font-semibold">Actor</th><th className="px-4 py-3 font-semibold">Resource</th><th className="px-4 py-3 font-semibold">IP</th><th className="px-4 py-3 font-semibold">Time</th></tr>
                </thead>
                <tbody className="divide-y divide-border/50">
                  {isLoading && Array.from({ length: 8 }).map((_, index) => <tr key={index}>{Array.from({ length: 5 }).map((__, cell) => <td key={cell} className="px-4 py-3"><Skeleton className="h-4 w-full" /></td>)}</tr>)}
                  {!isLoading && data?.logs.length === 0 && <tr><td colSpan={5}><EmptyState title="No audit entries match this filter" description="Try a broader action, actor, or date range." /></td></tr>}
                  {data?.logs.map(log => (
                    <tr key={log.id} className="cursor-pointer transition-colors hover:bg-muted/25" onClick={() => setSelected(log)}>
                      <td className="px-4 py-3"><p className="font-semibold">{log.action}</p><p className="font-mono text-[10px] text-muted-foreground">{shortId(log.id)}</p></td>
                      <td className="px-4 py-3"><p className="text-xs capitalize">{log.actor_type}</p><p className="font-mono text-[10px] text-muted-foreground">{log.actor_id ? shortId(log.actor_id) : '—'}</p></td>
                      <td className="px-4 py-3"><p className="text-xs font-medium">{log.resource_type}</p><p className="font-mono text-[10px] text-muted-foreground">{log.resource_id ? shortId(log.resource_id) : '—'}</p></td>
                      <td className="px-4 py-3 font-mono text-[10px] text-muted-foreground">{log.ip_address || '—'}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(log.created_at, true)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="divide-y divide-border/60 md:hidden">
              {isLoading && Array.from({ length: 4 }).map((_, i) => <div key={i} className="p-4"><Skeleton className="h-16 w-full" /></div>)}
              {!isLoading && data?.logs.length === 0 && <EmptyState title="No audit entries match this filter" />}
              {data?.logs.map(log => (
                <button key={log.id} onClick={() => setSelected(log)} className="block w-full p-4 text-left hover:bg-muted/30">
                  <p className="text-sm font-semibold">{log.action}</p>
                  <p className="mt-1 text-[10px] capitalize text-muted-foreground">{log.actor_type} · {log.resource_type} · {formatDate(log.created_at, true)}</p>
                </button>
              ))}
            </div>
            <Pagination total={data?.total ?? 0} limit={PAGE_SIZE} offset={offset} onChange={setOffset} loading={isLoading} />
          </>
        )}
      </Card>
      <p className="flex items-center gap-1.5 text-[10px] text-muted-foreground"><ShieldCheck className="h-3 w-3" /> Audit access now requires both authentication and the admin role.</p>
      {selected && <AuditDrawer log={selected} onClose={() => setSelected(null)} />}
    </div>
  )
}
